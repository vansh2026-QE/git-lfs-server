package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vansh2026/git-lfs-server/internal/qe"
)

// commonFlags are the connection/identity flags shared by init and clone.
type commonFlags struct {
	user, server, repo, password, name, email string
}

func bindCommon(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	fs.StringVar(&c.user, "u", "", "LFS user (required)")
	fs.StringVar(&c.server, "s", "http://localhost:8080", "lfsd origin")
	fs.StringVar(&c.repo, "r", "demo", "policy repo name")
	fs.StringVar(&c.password, "p", "", "user password (default <user>pw)")
	fs.StringVar(&c.name, "n", "", "git user.name (default user)")
	fs.StringVar(&c.email, "e", "", "git user.email (default user@example.com)")
	return c
}

// identity resolves the git identity, defaulting name to the user and email to
// user@example.com. init always sets an identity; clone passes the raw flags so
// an identity is recorded only when the user supplied one (matching the script).
func (c *commonFlags) identity() (name, email string) {
	name, email = c.name, c.email
	if name == "" {
		name = c.user
	}
	if email == "" {
		email = c.user + "@example.com"
	}
	return name, email
}

// runInit scaffolds a fresh workspace: it creates the .qe layout, git-inits the
// hashed repo, wires it to the fork (filters/hooks/drivers + lfs.url +
// skipdownloaderrorcodes via wire), commits the .gitattributes, and records
// .qe/config. The legible workspace starts empty; the user populates it and runs
// add/commit in a later slice. There is nothing to reveal on init.
func runInit(argv []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	common := bindCommon(fs)
	origin := fs.String("o", "", "git remote to add as origin")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if common.user == "" {
		return fmt.Errorf("init: -u USER required")
	}
	dir := fs.Arg(0)
	if dir == "" {
		return fmt.Errorf("init: DIR required")
	}

	layout, err := qe.NewLayout(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(layout.QeDir(), 0o755); err != nil {
		return fmt.Errorf("init: create %s: %w", layout.QeDir(), err)
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
	if err := runCommand("", nil, "git", "init", "-b", "main", hashed); err != nil {
		return err
	}
	if *origin != "" {
		if err := runCommand(hashed, nil, "git", "remote", "add", "origin", *origin); err != nil {
			return err
		}
	}
	name, email := common.identity()
	if err := fork.wire(hashed, lfsURL, name, email); err != nil {
		return err
	}
	if err := writeGitattributes(hashed); err != nil {
		return err
	}
	if err := runCommand(hashed, nil, "git", "add", ".gitattributes"); err != nil {
		return err
	}
	if err := runCommand(hashed, nil, "git", "commit", "-m",
		"Track all files with Git LFS (except git metadata)"); err != nil {
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

	fmt.Printf("qegit: initialized workspace at %s (hashed repo: %s)\n",
		layout.Legible(), hashed)
	return nil
}
