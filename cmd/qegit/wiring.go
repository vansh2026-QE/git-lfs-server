package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// forkPaths holds absolute paths to the bundled fork git-lfs and its contrib
// helper scripts. wire() reproduces, in Go, the repo setup scripts/lfs-setup.sh
// performed, so qegit owns wiring end to end (design §8). Contrib paths may be
// empty when a helper is absent; wiring then skips it, mirroring the script's
// guarded behavior.
type forkPaths struct {
	lfsBin  string // fork git-lfs (absolute) or "git-lfs" if not bundled
	merge   string // contrib/lfs-text-merge.sh
	diff    string // contrib/lfs-text-diff.sh
	redact  string // contrib/redact-commit-msg.sh
	msgPush string // contrib/lfs-msg-push.sh
}

// forkRoot returns the client-implementation/git-lfs directory. It honors
// QEGIT_FORK_DIR, else walks up from the working directory. A stable install
// location is design §8's job (out of scope here).
func forkRoot() (string, error) {
	if p := os.Getenv("QEGIT_FORK_DIR"); p != "" {
		return p, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "client-implementation", "git-lfs")
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate client-implementation/git-lfs (set QEGIT_FORK_DIR)")
		}
		dir = parent
	}
}

// resolveFork locates the fork git-lfs binary and contrib helpers. A bundled,
// executable bin/git-lfs is preferred (so transfers carry the real obj.Name the
// policy server needs); otherwise it falls back to git-lfs on PATH. Missing
// contrib scripts are left empty and skipped during wiring.
func resolveFork() (forkPaths, error) {
	root, err := forkRoot()
	if err != nil {
		return forkPaths{}, err
	}
	f := forkPaths{lfsBin: "git-lfs"}
	bin := filepath.Join(root, "bin", "git-lfs")
	if fi, err := os.Stat(bin); err == nil && fi.Mode()&0o111 != 0 {
		f.lfsBin = bin
	}
	contrib := filepath.Join(root, "contrib")
	f.merge = existingFile(filepath.Join(contrib, "lfs-text-merge.sh"))
	f.diff = existingFile(filepath.Join(contrib, "lfs-text-diff.sh"))
	f.redact = existingFile(filepath.Join(contrib, "redact-commit-msg.sh"))
	f.msgPush = existingFile(filepath.Join(contrib, "lfs-msg-push.sh"))
	return f, nil
}

// existingFile returns path if it exists, else "".
func existingFile(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// buildLFSURL renders the authenticated lfs.url (scheme://user:pass@host/repo)
// from the connection flags, the Go port of lfs-setup.sh's lfs_url_auth. The
// password defaults to "<user>pw" to match the demo credentials. Userinfo is
// percent-encoded by url; qe.NewClient parses it back symmetrically.
func buildLFSURL(server, repo, user, password string) (string, error) {
	u, err := url.Parse(strings.TrimSuffix(server, "/"))
	if err != nil {
		return "", fmt.Errorf("parse server %q: %w", server, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("server %q must be scheme://host", server)
	}
	if password == "" {
		password = user + "pw"
	}
	u.User = url.UserPassword(user, password)
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + repo
	return u.String(), nil
}

// wire configures a freshly created or cloned hashed repo to use the fork:
// lfs.url, 403-as-declined, the fork's LFS filters and hooks, the content-aware
// merge and diff drivers, and (when provided) the local git identity. It is the
// Go port of lfs-setup.sh's wire_fork_paths + wire_merge_driver +
// wire_diff_driver + set_identity. Filters point at the fork by absolute path,
// so the PATH export the script needed is unnecessary here.
func (f forkPaths) wire(dir, lfsURL, name, email string) error {
	set := func(key, val string) error { return gitConfigSet(dir, key, val) }

	if err := set("lfs.url", lfsURL); err != nil {
		return err
	}
	if err := set("lfs.skipdownloaderrorcodes", "403"); err != nil {
		return err
	}
	if err := runCommand(dir, nil, f.lfsBin, "install", "--local"); err != nil {
		return err
	}

	filters := [][2]string{
		{"filter.lfs.process", f.lfsBin + " filter-process"},
		{"filter.lfs.clean", f.lfsBin + " clean -- %f"},
		{"filter.lfs.smudge", f.lfsBin + " smudge -- %f"},
		{"filter.lfs.required", "true"},
	}
	for _, kv := range filters {
		if err := set(kv[0], kv[1]); err != nil {
			return err
		}
	}
	if err := f.writeHooks(dir); err != nil {
		return err
	}
	if err := f.wireMergeDriver(dir); err != nil {
		return err
	}
	if err := f.wireDiffDriver(dir); err != nil {
		return err
	}
	if name != "" {
		if err := set("user.name", name); err != nil {
			return err
		}
	}
	if email != "" {
		if err := set("user.email", email); err != nil {
			return err
		}
	}
	return nil
}

// writeHooks installs the fork's LFS hooks plus the message-redaction chain,
// matching the hook scripts lfs-setup.sh wrote. The post-* hooks just invoke the
// fork; pre-push uploads cached commit messages (fail-closed) before the LFS
// pre-push; commit-msg redacts the real message to a "msg:<oid>" placeholder.
func (f forkPaths) writeHooks(dir string) error {
	for _, h := range []string{"post-checkout", "post-commit", "post-merge"} {
		content := fmt.Sprintf("#!/bin/sh\n\"%s\" %s \"$@\"\n", f.lfsBin, h)
		if err := writeHook(dir, h, content); err != nil {
			return err
		}
	}
	var prePush string
	if f.msgPush != "" {
		prePush = fmt.Sprintf("#!/bin/sh\n\"%s\" \"$@\" || exit $?\nexec \"%s\" pre-push \"$@\"\n", f.msgPush, f.lfsBin)
	} else {
		prePush = fmt.Sprintf("#!/bin/sh\n\"%s\" pre-push \"$@\"\n", f.lfsBin)
	}
	if err := writeHook(dir, "pre-push", prePush); err != nil {
		return err
	}
	if f.redact != "" {
		content := fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"$@\"\n", f.redact)
		if err := writeHook(dir, "commit-msg", content); err != nil {
			return err
		}
	}
	return nil
}

// wireMergeDriver registers the content-aware LFS 3-way merge driver locally.
// The committed .gitattributes opts paths in via merge=lfs-text; the driver
// definition must stay local because git refuses driver commands from committed
// attributes. Skipped with a warning when the script is absent.
func (f forkPaths) wireMergeDriver(dir string) error {
	if f.merge == "" {
		fmt.Fprintln(os.Stderr, "qegit: LFS merge driver not found; skipping merge-driver setup")
		return nil
	}
	if err := gitConfigSet(dir, "merge.lfs-text.name", "LFS content-aware 3-way merge"); err != nil {
		return err
	}
	driver := fmt.Sprintf("GIT_LFS='%s' '%s' %%O %%A %%B %%L %%P", f.lfsBin, f.merge)
	return gitConfigSet(dir, "merge.lfs-text.driver", driver)
}

// wireDiffDriver registers the LFS textconv diff driver locally, so git
// diff/log -p/show compare smudged content instead of pointer stubs. Skipped
// with a warning when the script is absent.
func (f forkPaths) wireDiffDriver(dir string) error {
	if f.diff == "" {
		fmt.Fprintln(os.Stderr, "qegit: LFS diff driver not found; skipping diff-driver setup")
		return nil
	}
	textconv := fmt.Sprintf("GIT_LFS='%s' '%s'", f.lfsBin, f.diff)
	if err := gitConfigSet(dir, "diff.lfs.textconv", textconv); err != nil {
		return err
	}
	return gitConfigSet(dir, "diff.lfs.cachetextconv", "true")
}

// gitattributesContent tracks everything via LFS except the two files git must
// read by name. Matches lfs-setup.sh's write_gitattributes; these names stay
// plaintext as a deliberate cleartext exception (design §5).
const gitattributesContent = `* filter=lfs diff=lfs merge=lfs-text -text
.gitattributes !filter !diff !merge text
.gitignore !filter !diff !merge text
`

// writeGitattributes drops the committed .gitattributes into the hashed repo.
func writeGitattributes(dir string) error {
	path := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(path, []byte(gitattributesContent), 0o644); err != nil {
		return fmt.Errorf("write .gitattributes: %w", err)
	}
	return nil
}

// gitConfigSet writes one repo-local git config value.
func gitConfigSet(dir, key, value string) error {
	return runCommand(dir, nil, "git", "config", "--local", key, value)
}

// writeHook writes an executable git hook into the repo's hooks directory.
func writeHook(dir, name, content string) error {
	rel, err := gitOutput(dir, "rev-parse", "--git-path", "hooks")
	if err != nil {
		return err
	}
	hooks := rel
	if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(dir, rel)
	}
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(hooks, name), []byte(content), 0o755); err != nil {
		return fmt.Errorf("write hook %s: %w", name, err)
	}
	return nil
}

// gitOutput runs git in dir and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
