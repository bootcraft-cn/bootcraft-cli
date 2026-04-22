package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

var alwaysExcludeDirs = map[string]bool{
	".git":         true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"node_modules": true,
	"dist":         true,
	".next":        true,
	".nuxt":        true,
	"target":       true,
	"build":        true,
	"out":          true,
	".gradle":      true,
	"vendor":       true,
}

var alwaysExcludeExts = map[string]bool{
	".pyc":   true,
	".pyo":   true,
	".class": true,
	".jar":   true,
	".war":   true,
	".ear":   true,
	".o":     true,
	".a":     true,
	".so":    true,
	".exe":   true,
	".log":   true,
}

var alwaysExcludeFiles = map[string]bool{
	".DS_Store": true,
	"Thumbs.db": true,
}

var alwaysExcludeSuffixes = []string{
	".egg-info",
}

func Pack(dir string) (*bytes.Buffer, int, int64, error) {
	gitIgnore := loadIgnoreFile(filepath.Join(dir, ".gitignore"))
	bcIgnore := loadIgnoreFile(filepath.Join(dir, ".bootcraftignore"))

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	var fileCount int
	var totalSize int64

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		// Normalize to forward slashes for matching
		relSlash := filepath.ToSlash(rel)
		baseName := d.Name()

		if d.IsDir() {
			if alwaysExcludeDirs[baseName] {
				return filepath.SkipDir
			}
			if gitIgnore != nil && gitIgnore.MatchesPath(relSlash+"/") {
				return filepath.SkipDir
			}
			if bcIgnore != nil && bcIgnore.MatchesPath(relSlash+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		// bootcraft.yml is always included
		if baseName != "bootcraft.yml" {
			if shouldExcludeFile(baseName) {
				return nil
			}
			if gitIgnore != nil && gitIgnore.MatchesPath(relSlash) {
				return nil
			}
			if bcIgnore != nil && bcIgnore.MatchesPath(relSlash) {
				return nil
			}
		}

		// Security: reject path traversal
		if strings.Contains(relSlash, "../") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relSlash

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		n, err := copyToTar(tw, f)
		if err != nil {
			return err
		}

		fileCount++
		totalSize += n
		return nil
	})

	if err != nil {
		return nil, 0, 0, err
	}

	if err := tw.Close(); err != nil {
		return nil, 0, 0, err
	}
	if err := gw.Close(); err != nil {
		return nil, 0, 0, err
	}

	return buf, fileCount, totalSize, nil
}

func copyToTar(tw *tar.Writer, f *os.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := tw.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return total, err
		}
	}
	return total, nil
}

func shouldExcludeFile(name string) bool {
	if alwaysExcludeFiles[name] {
		return true
	}
	ext := filepath.Ext(name)
	if alwaysExcludeExts[ext] {
		return true
	}
	for _, suffix := range alwaysExcludeSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func loadIgnoreFile(path string) *ignore.GitIgnore {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	ig, err := ignore.CompileIgnoreFile(path)
	if err != nil {
		return nil
	}
	return ig
}
