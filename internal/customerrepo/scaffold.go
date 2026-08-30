package customerrepo

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:scaffold
var scaffoldFS embed.FS

// InitProject copies the embedded one/v1 scaffold into dir.
// Fails if dir exists and is non-empty (unless force).
func InitProject(dir string, customerID string, force bool) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	ents, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	if len(ents) > 0 && !force {
		return fmt.Errorf("directory not empty: %s (pass force to overwrite scaffold files)", abs)
	}
	err = fs.WalkDir(scaffoldFS, "scaffold", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "scaffold")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		dest := filepath.Join(abs, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := scaffoldFS.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == "one.yaml" && customerID != "" && customerID != "REPLACE_CUSTOMER_ID" {
			s := string(data)
			s = strings.Replace(s, "REPLACE_CUSTOMER_ID", customerID, 1)
			data = []byte(s)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
	return err
}
