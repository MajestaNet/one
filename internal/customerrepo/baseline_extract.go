package customerrepo

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ExtractBaselineFromZip writes only .one/baseline/** from an export zip into destRoot.
// Customer metadata/src/tests are left untouched.
func ExtractBaselineFromZip(r io.ReaderAt, size int64, destRoot string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	abs, err := filepath.Abs(destRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(abs, ".one", "baseline"), 0o755); err != nil {
		return err
	}
	// Clear previous baseline objects/fields so removals from managed seed apply.
	_ = os.RemoveAll(filepath.Join(abs, ".one", "baseline"))
	if err := os.MkdirAll(filepath.Join(abs, ".one", "baseline"), 0o755); err != nil {
		return err
	}
	wrote := 0
	for _, f := range zr.File {
		name := path.Clean("/" + strings.ReplaceAll(f.Name, "\\", "/"))
		name = strings.TrimPrefix(name, "/")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		if name != ".one/baseline" && !strings.HasPrefix(name, ".one/baseline/") {
			continue
		}
		dest := filepath.Join(abs, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, 8<<20))
		_ = rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		wrote++
	}
	if wrote == 0 {
		return fmt.Errorf("export zip contained no .one/baseline entries")
	}
	return nil
}
