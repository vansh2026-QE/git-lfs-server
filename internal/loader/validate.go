package loader

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/policy"
)

const supportedVersion = 1

var principalIDRE = regexp.MustCompile(`^(user|group|service):[a-zA-Z0-9_.-]+$`)

func validateVersion(v int) error {
	if v != supportedVersion {
		return fmt.Errorf("loader: unsupported policy version %d (want %d)", v, supportedVersion)
	}
	return nil
}

func validatePrincipalID(id string) error {
	if id == policy.PrincipalAnonymous {
		return nil
	}
	if principalIDRE.MatchString(id) {
		return nil
	}
	return fmt.Errorf("loader: invalid principal id %q", id)
}

// parseAccess turns an SVN-style access code into the actions it grants:
// 'r' -> download, 'w' -> upload. "rw"/"wr" grant both. The code must be
// non-empty and contain each of 'r'/'w' at most once; anything else is an
// error. See docs/auth-design.md §10.2.
func parseAccess(code string) ([]policy.Action, error) {
	if code == "" {
		return nil, fmt.Errorf("loader: empty access code")
	}
	var read, write bool
	for _, c := range code {
		switch c {
		case 'r':
			if read {
				return nil, fmt.Errorf("loader: repeated %q in access code %q", string(c), code)
			}
			read = true
		case 'w':
			if write {
				return nil, fmt.Errorf("loader: repeated %q in access code %q", string(c), code)
			}
			write = true
		default:
			return nil, fmt.Errorf("loader: invalid access code %q (want combinations of 'r' and 'w')", code)
		}
	}
	// Order actions write-then-read for stable, deterministic trie population.
	var actions []policy.Action
	if write {
		actions = append(actions, policy.ActionUpload)
	}
	if read {
		actions = append(actions, policy.ActionDownload)
	}
	return actions, nil
}

// validateGrantPath rejects grant patterns the trie cannot represent: globs
// inside segments (*.bin, a/*/b, **/*.go). The only supported wildcard forms
// are "**" (root) and a trailing "/**" (subtree). See docs/auth-design.md §10.3.
func validateGrantPath(path string) error {
	if path == "**" {
		return nil
	}
	stripped := strings.TrimSuffix(path, "/**")
	if strings.Contains(stripped, "*") {
		return fmt.Errorf("loader: glob inside segment not supported in path %q", path)
	}
	return nil
}
