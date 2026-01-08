package fileutil

import (
	"os"
	"path/filepath"
	"strings"
)

// CollectFilesRecursively collects all files in a directory, skipping .git and .jj
func CollectFilesRecursively(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git and .jj directories
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".jj" {
				return filepath.SkipDir
			}
		}

		// Add files (not directories)
		if !d.IsDir() {
			// Also skip if path contains /.git/ or /.jj/
			if strings.Contains(path, "/.git/") || strings.Contains(path, "/.jj/") {
				return nil
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}

		return nil
	})

	return files, err
}
