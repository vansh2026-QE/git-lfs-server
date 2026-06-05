package qe

import "testing"

const validPointer = "version https://git-lfs.github.com/spec/v1\n" +
	"oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\n" +
	"size 12\n"

func TestParsePointerValid(t *testing.T) {
	p, err := ParsePointer([]byte(validPointer))
	if err != nil {
		t.Fatalf("ParsePointer: %v", err)
	}
	if p.OID != "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393" {
		t.Errorf("OID = %q", p.OID)
	}
	if p.Size != 12 {
		t.Errorf("Size = %d, want 12", p.Size)
	}
}

func TestParsePointerErrors(t *testing.T) {
	cases := map[string]string{
		"missing version": "oid sha256:abc\nsize 1\n",
		"missing oid":     "version https://git-lfs.github.com/spec/v1\nsize 1\n",
		"missing size":    "version https://git-lfs.github.com/spec/v1\noid sha256:abc\n",
		"non-sha256 oid":  "version https://git-lfs.github.com/spec/v1\noid md5:abc\nsize 1\n",
		"bad size":        "version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize huge\n",
		"not a pointer":   "just some bytes\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePointer([]byte(in)); err == nil {
				t.Errorf("expected error for %q", name)
			}
		})
	}
}
