package token_test

import (
	"strings"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/token"
)

const (
	shPrivate = "715dc8493c36579a5b116995100f635e3572fdf8703e708ef1a08d943b36774e"
	shPrivBob = "ae6b23b819e47d3a6994b5bc78ff1773fc7b9352fd64d46f1fa87663a7879da4"
	shSecret  = "a5c785ddabe32089aa3df20f6dba4079faffd634009a9f9954ba735dfa96e604"
)

func TestPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"design example", "private/privBob/bobsecret.txt", shPrivate + "/" + shPrivBob + "/" + shSecret},
		{"single component", "private", shPrivate},
		{"leading slash", "/private/privBob/bobsecret.txt", shPrivate + "/" + shPrivBob + "/" + shSecret},
		{"trailing slash", "private/privBob/bobsecret.txt/", shPrivate + "/" + shPrivBob + "/" + shSecret},
		{"duplicate slashes", "private//privBob///bobsecret.txt", shPrivate + "/" + shPrivBob + "/" + shSecret},
		{"empty path", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := token.Path(tc.in)
			if err != nil {
				t.Fatalf("Path(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Path(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPathRejectsDotDot(t *testing.T) {
	for _, in := range []string{"..", "private/../etc", "a/b/.."} {
		if _, err := token.Path(in); err == nil {
			t.Errorf("Path(%q) = nil error, want rejection", in)
		}
	}
}

func TestPathCaseSensitive(t *testing.T) {
	lower, _ := token.Path("private")
	upper, _ := token.Path("Private")
	if lower == upper {
		t.Errorf("tokenization must be case-sensitive: %q == %q", lower, upper)
	}
}

func TestReverseMapRoundTrip(t *testing.T) {
	m := token.ReverseMap([]string{"private/privBob/bobsecret.txt"})
	want := map[string]string{
		shPrivate: "private",
		shPrivBob: "private/privBob",
		shSecret:  "private/privBob/bobsecret.txt",
	}
	for tok, real := range want {
		if got := m[tok]; got != real {
			t.Errorf("ReverseMap[%s] = %q, want %q", tok, got, real)
		}
	}
	if len(m) != len(want) {
		t.Errorf("ReverseMap size = %d, want %d: %v", len(m), len(want), m)
	}
}

// Bob, authorized only under private/privBob, can reveal the shared ancestor
// "private" but not Alice's sibling, so her token path renders private/<hash>/<hash> (§3).
func TestReverseMapPartialReveal(t *testing.T) {
	m := token.ReverseMap([]string{"private/privBob/bobsecret.txt"})

	aliceToken, _ := token.Path("private/privAlice/secret.txt")
	comps := strings.Split(aliceToken, "/")

	revealed := make([]string, len(comps))
	for i, c := range comps {
		if real, ok := m[c]; ok {
			revealed[i] = real[strings.LastIndex(real, "/")+1:]
		} else {
			revealed[i] = "<hash>"
		}
	}
	got := strings.Join(revealed, "/")
	if want := "private/<hash>/<hash>"; got != want {
		t.Errorf("partial reveal = %q, want %q", got, want)
	}
}

func TestReverseMapSkipsInvalid(t *testing.T) {
	m := token.ReverseMap([]string{"private/../etc", "private/privBob/bobsecret.txt"})
	if _, ok := m[shPrivBob]; !ok {
		t.Errorf("valid path dropped alongside invalid one: %v", m)
	}
}
