package main

import (
	"os"

	"github.com/vansh2026/git-lfs-server/internal/qe"
)

// runCommit commits staged token paths in the hashed repo while binding the
// commit message to real paths. It derives the token->real sidecar from the
// legible tree, writes it to a temp file, and runs `git commit <args...>` in the
// hashed repo with QEGIT_SIDECAR pointing at that file so the commit-msg
// redaction hook can translate staged token paths back to real ones (design
// §8.4). The sidecar is removed afterward; it is never persisted (§7.3). All
// argv is passed through to git commit (e.g. -m, -a, --amend).
func runCommit(argv []string) error {
	layout, err := layoutFromCwd()
	if err != nil {
		return err
	}

	m, err := qe.BuildSidecar(layout)
	if err != nil {
		return err
	}
	sidecar, err := qe.WriteSidecar(m)
	if err != nil {
		return err
	}
	defer os.Remove(sidecar)

	env := []string{"QEGIT_SIDECAR=" + sidecar}
	return runCommand(layout.Hashed(), env, "git", append([]string{"commit"}, argv...)...)
}
