package main

import "testing"

func TestBuildLFSURL(t *testing.T) {
	cases := []struct {
		name                         string
		server, repo, user, password string
		want                         string
	}{
		{"default password", "http://localhost:8080", "demo", "bob", "", "http://bob:bobpw@localhost:8080/demo"},
		{"explicit password", "http://localhost:8080", "demo", "alice", "s3cret", "http://alice:s3cret@localhost:8080/demo"},
		{"trailing slash", "http://localhost:8080/", "demo", "bob", "", "http://bob:bobpw@localhost:8080/demo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildLFSURL(tc.server, tc.repo, tc.user, tc.password)
			if err != nil {
				t.Fatalf("buildLFSURL: %v", err)
			}
			if got != tc.want {
				t.Errorf("buildLFSURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildLFSURLRejectsBadServer(t *testing.T) {
	if _, err := buildLFSURL("localhost:8080", "demo", "bob", ""); err == nil {
		t.Fatal("expected error for server missing scheme")
	}
}
