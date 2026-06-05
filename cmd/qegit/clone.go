package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/qe"
)

// runClone clones an existing hashed repo and projects the caller's authorized
// subtree into a legible workspace. It git-clones with GIT_LFS_SKIP_SMUDGE=1 so
// the hashed working tree stays uniformly token-named pointer stubs (design
// §7.1), wires the repo to the fork, records .qe/config, then runs the reveal:
// /names -> token path -> stub oid -> /content bytes written at the real name in
// legible. Unauthorized files never appear in legible (§9).
func runClone(argv []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	common := bindCommon(fs)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if common.user == "" {
		return fmt.Errorf("clone: -u USER required")
	}
	remote := fs.Arg(0)
	if remote == "" {
		return fmt.Errorf("clone: REMOTE required")
	}
	dir := fs.Arg(1)
	if dir == "" {
		dir = defaultCloneDir(remote)
	}

	layout, err := qe.NewLayout(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(layout.QeDir(), 0o755); err != nil {
		return fmt.Errorf("clone: create %s: %w", layout.QeDir(), err)
	}

	fork, err := resolveFork()
	if err != nil {
		return err
	}
	lfsURL, err := buildLFSURL(common.server, common.repo, common.user, common.password)
	if err != nil {
		return err
	}

	hashed := layout.Hashed()
	if err := runCommand("", []string{"GIT_LFS_SKIP_SMUDGE=1"},
		"git", "clone",
		"-c", "lfs.url="+lfsURL,
		"-c", "lfs.skipdownloaderrorcodes=403",
		remote, hashed); err != nil {
		return err
	}
	// Pass the raw identity flags: clone records an identity only if the user
	// supplied -n/-e, leaving the cloned repo's inherited config otherwise.
	if err := fork.wire(hashed, lfsURL, common.name, common.email); err != nil {
		return err
	}

	if err := layout.WriteConfig(qe.Config{
		Server: common.server,
		Repo:   common.repo,
		User:   common.user,
		LFSURL: lfsURL,
	}); err != nil {
		return err
	}

	client, err := qe.NewClient(lfsURL)
	if err != nil {
		return err
	}
	if err := qe.Reveal(context.Background(), client, layout); err != nil {
		return err
	}

	fmt.Printf("qegit: cloned into %s; revealed authorized files into legible.\n",
		layout.Legible())
	return nil
}

// defaultCloneDir derives the workspace directory name from the remote, matching
// git's default: the last path segment with any trailing ".git" stripped.
func defaultCloneDir(remote string) string {
	base := filepath.Base(strings.TrimSuffix(remote, "/"))
	return strings.TrimSuffix(base, ".git")
}
