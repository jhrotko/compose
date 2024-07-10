package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// ConvertSymbolicLink converts the section of an absolute path if it is a
// symbolic link
//
// Parameters:
//   - path: an absolute path
//
// Returns:
//   - converted path if it has a symbolic link or the same path if there is
//     no symbolic link
func ConvertSymbolicLink(path string) (string, error) {
	sym, part, err := getSymbolinkLink(path)
	if err != nil {
		return "", err
	}
	if sym == "" && part == "" {
		// no symbolic link detected
		return path, nil
	}
	return strings.Replace(path, part, sym, 1), nil

}

// getSymbolinkLink parses all parts of the path and returns the
//
// Parameters:
//   - path: an absolute path
//
// Returns:
//   - string section of the path that is a symbolic link
//   - string correspondent path section of the symbolic link
//   - An error
func getSymbolinkLink(path string) (string, string, error) {
	parts := strings.Split(path, string(os.PathSeparator))

	// Reconstruct the path step by step, checking each component
	var currentPath string
	if filepath.IsAbs(path) {
		currentPath = string(os.PathSeparator)
	}

	for _, part := range parts {
		if part == "" {
			continue
		}
		currentPath = filepath.Join(currentPath, part)

		if isSymLink := isSymbolicLink(currentPath); isSymLink {
			// return symbolic link, and correspondent part
			target, err := filepath.EvalSymlinks(currentPath)
			if err != nil {
				return "", "", err
			}
			return target, currentPath, nil
		}
	}
	return "", "", nil // no symbolic link
}

// isSymbolicLink validates if the path is a symbolic link
func isSymbolicLink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}

	// Check if the file mode indicates a symbolic link
	return info.Mode()&os.ModeSymlink != 0
}
