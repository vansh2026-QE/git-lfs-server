package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/qe"
)

// changedPaths splits reals/tokens into the subset whose legible bytes differ
// from the pointer staged in the hashed repo's index. A path absent from the
// index (new file) or one whose legible sha256/size no longer matches the
// staged pointer is "changed"; unchanged paths are dropped so `qegit add` only
// rewrites and re-stages what actually changed (the working tree is not used as
// the signal because our own SyncIn leaves real bytes there). reals and tokens
// are parallel slices. Returns the changed reals and their tokens.
func changedPaths(layout qe.Layout, reals, tokens []string) ([]string, []string, error) {
	staged, err := stagedPointers(layout.Hashed(), tokens)
	if err != nil {
		return nil, nil, err
	}
	var cr, ct []string
	for i, real := range reals {
		if ptr, ok := staged[tokens[i]]; ok {
			oid, size, err := fileOID(filepath.Join(layout.Legible(), filepath.FromSlash(real)))
			if err != nil {
				return nil, nil, err
			}
			if ptr.OID == oid && ptr.Size == size {
				continue
			}
		}
		cr = append(cr, real)
		ct = append(ct, tokens[i])
	}
	return cr, ct, nil
}

// stagedPointers reads each token path's stage-0 (index) blob via a single
// `git cat-file --batch` and returns the parsed LFS pointer for the paths that
// are present and parse as pointers. Missing or non-pointer entries are omitted.
func stagedPointers(hashed string, tokens []string) (map[string]qe.Pointer, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	var in bytes.Buffer
	for _, t := range tokens {
		fmt.Fprintf(&in, ":%s\n", t)
	}
	cmd := exec.Command("git", "-C", hashed, "cat-file", "--batch")
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file: %w", err)
	}
	return parseBatch(tokens, out)
}

// parseBatch consumes `git cat-file --batch` output in input order. For each
// token it reads either a "<sha> blob <size>" header followed by <size> bytes
// and a newline, or a "<input> missing" line (omitted from the result).
func parseBatch(tokens []string, out []byte) (map[string]qe.Pointer, error) {
	m := make(map[string]qe.Pointer)
	r := bufio.NewReader(bytes.NewReader(out))
	for _, tok := range tokens {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("cat-file: short output for %s: %w", tok, err)
		}
		fields := strings.Fields(strings.TrimRight(header, "\n"))
		if len(fields) >= 2 && fields[len(fields)-1] == "missing" {
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("cat-file: unexpected header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("cat-file: bad size in %q", header)
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(r, content); err != nil {
			return nil, fmt.Errorf("cat-file: read content for %s: %w", tok, err)
		}
		if _, err := r.Discard(1); err != nil { // trailing newline
			return nil, fmt.Errorf("cat-file: missing newline after %s: %w", tok, err)
		}
		if ptr, err := qe.ParsePointer(content); err == nil {
			m[tok] = ptr
		}
	}
	return m, nil
}

// fileOID returns the lowercase hex sha256 and byte size of the file at path,
// matching the oid/size an LFS pointer records for its real bytes.
func fileOID(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
