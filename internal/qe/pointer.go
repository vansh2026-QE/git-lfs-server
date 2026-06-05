package qe

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Pointer is the parsed content of a Git LFS pointer stub: the object id and
// declared size that let qegit dereference real bytes via lfsd. See the LFS
// pointer spec (the 3-line text format) and docs/name-hiding-design.md §7.1,
// where the hashed working tree holds exactly these stubs.
type Pointer struct {
	OID  string // hex sha256 digest (the "sha256:" prefix stripped)
	Size int64  // declared byte size
}

// pointerVersionPrefix is the marker every LFS pointer's first key carries; we
// require it so an arbitrary text file is not mistaken for a pointer.
const pointerVersionPrefix = "version https://git-lfs.github.com/spec/"

// ParsePointer reads a minimal LFS pointer stub and extracts oid and size. The
// format is line-oriented "key value" pairs, sorted after the leading version
// line. We accept only sha256 oids (the digest this server records) and reject
// input missing the version marker, oid, or size.
func ParsePointer(data []byte) (Pointer, error) {
	var (
		p          Pointer
		sawVersion bool
		sawOID     bool
		sawSize    bool
	)
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, " ")
		if !ok {
			return Pointer{}, fmt.Errorf("qe: malformed pointer line %q", line)
		}
		switch key {
		case "version":
			if !strings.HasPrefix(line, pointerVersionPrefix) {
				return Pointer{}, fmt.Errorf("qe: unrecognized pointer version %q", val)
			}
			sawVersion = true
		case "oid":
			digest, ok := strings.CutPrefix(val, "sha256:")
			if !ok {
				return Pointer{}, fmt.Errorf("qe: unsupported pointer oid %q (want sha256:)", val)
			}
			p.OID = digest
			sawOID = true
		case "size":
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return Pointer{}, fmt.Errorf("qe: invalid pointer size %q: %w", val, err)
			}
			p.Size = n
			sawSize = true
		}
	}
	if err := sc.Err(); err != nil {
		return Pointer{}, fmt.Errorf("qe: read pointer: %w", err)
	}
	switch {
	case !sawVersion:
		return Pointer{}, fmt.Errorf("qe: not an LFS pointer (missing version line)")
	case !sawOID:
		return Pointer{}, fmt.Errorf("qe: pointer missing oid")
	case !sawSize:
		return Pointer{}, fmt.Errorf("qe: pointer missing size")
	}
	return p, nil
}
