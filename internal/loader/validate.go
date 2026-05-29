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

func parseAction(name string) (policy.Action, error) {
	if name == "*" {
		return "", fmt.Errorf("loader: wildcard action %q is not allowed", name)
	}
	switch policy.Action(name) {
	case policy.ActionUpload, policy.ActionDownload:
		return policy.Action(name), nil
	default:
		return "", fmt.Errorf("loader: unknown action %q", name)
	}
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
