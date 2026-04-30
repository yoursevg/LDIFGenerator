package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var schemaFileExtensions = map[string]bool{
	".conf":   true,
	".ldif":   true,
	".schema": true,
}

func ResolveInputPaths(inputs []string) ([]string, error) {
	var paths []string
	for _, input := range inputs {
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		info, err := os.Stat(input)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			paths = append(paths, input)
			continue
		}
		dirPaths, err := schemaFilesInDir(input)
		if err != nil {
			return nil, err
		}
		paths = append(paths, dirPaths...)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no schema files found")
	}
	return paths, nil
}

func schemaFilesInDir(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if schemaFileExtensions[strings.ToLower(filepath.Ext(path))] {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}
