package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vansh2026/git-lfs-server/internal/qe"
)

// layoutFromCwd locates the enclosing .qe workspace by walking up from the
// current directory until a directory containing a .qe child is found, and
// returns its Layout. Unlike init/clone (which take a DIR arg), add/commit run
// from inside an existing legible workspace, so the root is discovered rather
// than supplied. Fails if no .qe is found up to the filesystem root.
func layoutFromCwd() (qe.Layout, error) {
	dir, err := os.Getwd()
	if err != nil {
		return qe.Layout{}, err
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".qe")); err == nil && fi.IsDir() {
			return qe.NewLayout(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return qe.Layout{}, fmt.Errorf("not inside a qegit workspace (no .qe found)")
		}
		dir = parent
	}
}
