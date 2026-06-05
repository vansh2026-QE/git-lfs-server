package token

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Path returns the per-component tokenization of realPath. Each path component
// is replaced by the token of the cumulative normalized prefix up to and
// including that component, joined by "/":
//
//	Path("private/privBob/bobsecret.txt") =
//	    SHA("private")/SHA("private/privBob")/SHA("private/privBob/bobsecret.txt")
//
// The token is unkeyed and deterministic, so the same real path yields the same
// token path everywhere (clones, branches). Normalization matches the policy
// server's splitPath (docs/auth-design.md §6.5) so both sides agree.
// See docs/name-hiding-design.md §5.
func Path(realPath string) (string, error) {
	segs, err := segments(realPath)
	if err != nil {
		return "", err
	}
	tokens := make([]string, len(segs))
	for i := range segs {
		tokens[i] = component(strings.Join(segs[:i+1], "/"))
	}
	return strings.Join(tokens, "/"), nil
}

// ReverseMap builds the token->real lookup a viewer uses to de-tokenize the
// parts of a tree it is authorized to see. For every cumulative prefix of every
// authorized real path it maps that prefix's token component -> the real
// prefix. A consumer de-tokenizes a token path component by component: a
// component present in the map reveals its real prefix (whose last segment is
// the real name); an absent component stays hashed. Ancestors of any authorized
// node are therefore revealed while unrelated siblings remain tokens (§3, §7.3).
func ReverseMap(realPaths []string) map[string]string {
	m := make(map[string]string)
	for _, rp := range realPaths {
		segs, err := segments(rp)
		if err != nil {
			continue
		}
		for i := range segs {
			prefix := strings.Join(segs[:i+1], "/")
			m[component(prefix)] = prefix
		}
	}
	return m
}

// component is the token for one cumulative normalized prefix: the lowercase
// hex SHA-256 of the prefix bytes.
func component(prefix string) string {
	sum := sha256.Sum256([]byte(prefix))
	return hex.EncodeToString(sum[:])
}

// segments splits on '/', drops empty segments, and rejects '..'. It mirrors
// policy.splitPath so the client tokenizes the real paths the server returns
// under identical rules (docs/auth-design.md §6.5).
func segments(path string) ([]string, error) {
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		switch p {
		case "":
			continue
		case "..":
			return nil, fmt.Errorf("token: '..' not allowed in path %q", path)
		}
		out = append(out, p)
	}
	return out, nil
}
