// Command qegit is the name-hiding git wrapper: it drives the two-directory
// (.qe) model from docs/name-hiding-design.md §7. The user lives in a legible
// workspace of real names and real bytes; underneath, a hashed git repo holds
// token-named LFS pointer stubs (what the remote stores). This slice ships the
// scaffolding plus the first two commands, init and clone; commit/push and the
// rest land in later slices (§10).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "init":
		err = runInit(args)
	case "clone":
		err = runClone(args)
	case "add":
		err = runAdd(args)
	case "commit":
		err = runCommit(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "qegit: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "qegit: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `qegit — name-hiding git wrapper

Usage:
  qegit init   -u USER [-s SERVER] [-r REPO] [-p PASSWORD] [-o ORIGIN] [-n NAME] [-e EMAIL] DIR
  qegit clone  -u USER [-s SERVER] [-r REPO] [-p PASSWORD] [-n NAME] [-e EMAIL] REMOTE [DIR]
  qegit add    PATH...
  qegit commit [git commit args...]

  init    scaffold a new .qe workspace and wire its hashed repo
  clone   clone an existing hashed repo and reveal authorized files into legible
  add     stage real files into the hashed repo as token-named LFS pointers
  commit  commit staged changes, binding the redacted message to real paths

Flags:
  -u USER       LFS user (e.g. alice, bob). Required.
  -s SERVER     lfsd origin (default: http://localhost:8080)
  -r REPO       policy repo name in the LFS URL path (default: demo)
  -p PASSWORD   user password (default: <USER>pw)
  -o ORIGIN     (init only) git remote to add as origin
  -n NAME       local git user.name  (default: USER)
  -e EMAIL      local git user.email (default: USER@example.com)
`)
}

// runCommand runs name with args, inheriting stdio so git/lfs output reaches the
// user. extraEnv entries (KEY=VALUE) are appended to the current environment;
// pass nil for none.
func runCommand(workdir string, extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = workdir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
