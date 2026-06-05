package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBatchPointerAndMissing(t *testing.T) {
	ptr := "version https://git-lfs.github.com/spec/v1\n" +
		"oid sha256:1111111111111111111111111111111111111111111111111111111111111111\n" +
		"size 7\n"
	// "found/tok" returns a blob; "missing/tok" is absent from the index.
	out := fmt.Sprintf("deadbeef blob %d\n%s\n:missing/tok missing\n", len(ptr), ptr)

	m, err := parseBatch([]string{"found/tok", "missing/tok"}, []byte(out))
	if err != nil {
		t.Fatalf("parseBatch: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("map size = %d, want 1: %v", len(m), m)
	}
	got, ok := m["found/tok"]
	if !ok {
		t.Fatal("found/tok absent from map")
	}
	if got.OID != "1111111111111111111111111111111111111111111111111111111111111111" || got.Size != 7 {
		t.Errorf("pointer = %+v, want oid=1...1 size=7", got)
	}
	if _, ok := m["missing/tok"]; ok {
		t.Error("missing/tok should not be in map")
	}
}

func TestParseBatchSkipsNonPointerBlob(t *testing.T) {
	blob := "just some text\n"
	out := fmt.Sprintf("cafef00d blob %d\n%s\n", len(blob), blob)
	m, err := parseBatch([]string{"plain/tok"}, []byte(out))
	if err != nil {
		t.Fatalf("parseBatch: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("map = %v, want empty (non-pointer blob skipped)", m)
	}
}

func TestFileOIDMatchesSha256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	data := []byte("the real bytes\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	oid, size, err := fileOID(path)
	if err != nil {
		t.Fatalf("fileOID: %v", err)
	}
	sum := sha256.Sum256(data)
	if oid != hex.EncodeToString(sum[:]) {
		t.Errorf("oid = %s, want %s", oid, hex.EncodeToString(sum[:]))
	}
	if size != int64(len(data)) {
		t.Errorf("size = %d, want %d", size, len(data))
	}
}
