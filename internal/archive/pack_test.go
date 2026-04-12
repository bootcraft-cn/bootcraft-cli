package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func extractFileNames(t *testing.T, buf *io.Reader) []string {
	t.Helper()
	gr, err := gzip.NewReader(*buf)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	sort.Strings(names)
	return names
}

func TestPack_BasicDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootcraft.yml"), []byte("course: test\nlanguage: go\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "pkg", "util.go"), []byte("package pkg\n"), 0644)

	buf, count, size, err := Pack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 files, got %d", count)
	}
	if size == 0 {
		t.Error("expected non-zero total size")
	}

	r := io.Reader(buf)
	names := extractFileNames(t, &r)
	expected := []string{"bootcraft.yml", "main.go", "pkg/util.go"}
	if len(names) != len(expected) {
		t.Errorf("expected %v, got %v", expected, names)
	}
	for i, n := range expected {
		if i < len(names) && names[i] != n {
			t.Errorf("expected %q at pos %d, got %q", n, i, names[i])
		}
	}
}

func TestPack_ExcludesGitDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootcraft.yml"), []byte("course: test\nlanguage: go\n"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644)

	buf, count, _, err := Pack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 file, got %d", count)
	}

	r := io.Reader(buf)
	names := extractFileNames(t, &r)
	for _, n := range names {
		if n == ".git/HEAD" || n == ".git/objects" {
			t.Errorf(".git should be excluded, found %q", n)
		}
	}
}

func TestPack_GitignoreExclusion(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootcraft.yml"), []byte("course: test\nlanguage: go\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("secret.txt\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret\n"), 0644)

	buf, count, _, err := Pack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 { // bootcraft.yml, .gitignore, main.go
		t.Errorf("expected 3 files, got %d", count)
	}

	r := io.Reader(buf)
	names := extractFileNames(t, &r)
	for _, n := range names {
		if n == "secret.txt" {
			t.Error("secret.txt should be excluded by .gitignore")
		}
	}
}

func TestPack_BootcraftignoreExclusion(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootcraft.yml"), []byte("course: test\nlanguage: go\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".bootcraftignore"), []byte("data/\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "data"), 0755)
	os.WriteFile(filepath.Join(dir, "data", "large.bin"), []byte("data\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	buf, _, _, err := Pack(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := io.Reader(buf)
	names := extractFileNames(t, &r)
	for _, n := range names {
		if n == "data/large.bin" {
			t.Error("data/large.bin should be excluded by .bootcraftignore")
		}
	}
}

func TestPack_BootcraftYmlAlwaysIncluded(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootcraft.yml"), []byte("course: test\nlanguage: go\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("bootcraft.yml\n"), 0644)

	buf, count, _, err := Pack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Error("expected at least 1 file")
	}

	r := io.Reader(buf)
	names := extractFileNames(t, &r)
	found := false
	for _, n := range names {
		if n == "bootcraft.yml" {
			found = true
		}
	}
	if !found {
		t.Error("bootcraft.yml should always be included even if .gitignore excludes it")
	}
}

func TestPack_HardcodedExclusions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootcraft.yml"), []byte("course: test\nlanguage: go\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, "__pycache__"), 0755)
	os.WriteFile(filepath.Join(dir, "__pycache__", "mod.pyc"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "test.pyc"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte(""), 0644)

	buf, count, _, err := Pack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 { // bootcraft.yml, main.go
		t.Errorf("expected 2 files, got %d", count)
	}

	r := io.Reader(buf)
	names := extractFileNames(t, &r)
	excluded := []string{"node_modules/pkg/index.js", "__pycache__/mod.pyc", "test.pyc", ".DS_Store"}
	for _, e := range excluded {
		for _, n := range names {
			if n == e {
				t.Errorf("%q should be excluded", e)
			}
		}
	}
}
