package files

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Storage struct {
	Dir string
}

func (s *Storage) Save(r io.Reader, ext string) (string, error) {
	ext = normalizeExt(ext)
	name := uuid.NewString() + ext
	full := filepath.Join(s.Dir, name)
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		os.Remove(full)
		return "", err
	}
	return name, nil
}

func (s *Storage) Path(name string) string {
	return filepath.Join(s.Dir, name)
}

func (s *Storage) Open(name string) (*os.File, error) {
	return os.Open(s.Path(name))
}

func (s *Storage) Delete(name string) error {
	err := os.Remove(s.Path(name))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func Ext(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

func StripExt(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}
