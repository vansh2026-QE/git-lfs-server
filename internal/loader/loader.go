package loader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// Loader reads policy bytes from a PolicyStore and produces a validated
// in-memory Policy plus optional memberships. See docs/auth-design.md §10.
type Loader struct {
	store ports.PolicyStore
}

// New returns a Loader backed by store.
func New(store ports.PolicyStore) *Loader {
	return &Loader{store: store}
}

// Load fetches bytes, decodes JSON, validates, and builds tries. Fail-loud on
// malformed input; subsumed grants and undefined group refs become Warnings.
func (l *Loader) Load(ctx context.Context) (*LoadResult, error) {
	raw, err := l.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("loader: read policy: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Fail loud on typo'd or stray keys (e.g. "principal" for "principals")
	// rather than silently dropping the grants they were meant to carry.
	dec.DisallowUnknownFields()
	var doc policyDocument
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("loader: decode JSON: %w", err)
	}
	if err := validateVersion(doc.Version); err != nil {
		return nil, err
	}
	return buildPolicy(&doc)
}

func buildPolicy(doc *policyDocument) (*LoadResult, error) {
	var warnings []Warning
	declaredGroups := collectDeclaredGroups(doc)

	p := &policy.Policy{Repos: make(map[string]*policy.RepoPolicy, len(doc.Repos))}
	for repoName, repoDoc := range doc.Repos {
		rp := &policy.RepoPolicy{Principals: make(map[string]*policy.PrincipalGrants)}
		for principal, grants := range repoDoc.Principals {
			if err := validatePrincipalID(principal); err != nil {
				return nil, err
			}
			pg := &policy.PrincipalGrants{Tries: make(map[policy.Action]*policy.Trie)}
			for actionName, paths := range grants {
				action, err := parseAction(actionName)
				if err != nil {
					return nil, err
				}
				tr := policy.NewTrie()
				for _, path := range paths {
					if err := validateGrantPath(path); err != nil {
						return nil, err
					}
					id := deriveGrantID(repoName, principal, action, path)
					_, warn, err := tr.Insert(path, id)
					if err != nil {
						return nil, fmt.Errorf("loader: repo %q principal %q action %s path %q: %w",
							repoName, principal, action, path, err)
					}
					if warn != "" {
						warnings = append(warnings, Warning{Message: warn})
					}
				}
				pg.Tries[action] = tr
			}
			rp.Principals[principal] = pg
		}
		p.Repos[repoName] = rp
	}

	for user, groups := range doc.Memberships {
		for _, g := range groups {
			if _, ok := declaredGroups[g]; !ok {
				warnings = append(warnings, Warning{
					Message: fmt.Sprintf("membership: user %q references undefined group %q", user, g),
				})
			}
		}
	}

	return &LoadResult{
		Policy:      p,
		Memberships: doc.Memberships,
		Warnings:    warnings,
	}, nil
}

func deriveGrantID(repo, principal string, action policy.Action, path string) policy.GrantID {
	return policy.GrantID(repo + "|" + principal + "|" + string(action) + "|" + path)
}

func collectDeclaredGroups(doc *policyDocument) map[string]struct{} {
	out := make(map[string]struct{})
	for _, repo := range doc.Repos {
		for principal := range repo.Principals {
			if strings.HasPrefix(principal, policy.PrincipalPrefixGroup) {
				out[principal] = struct{}{} // insert the key in the map, the value itself is useless.
				// struct {}{} takes 0 bytes.
			}
		}
	}
	return out
}
