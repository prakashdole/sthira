package scenarioprep

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sha256Hex returns the lowercase hex SHA256 of body.
func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// safeResolve returns the absolute path of rel inside root after rejecting
// any path that escapes the workspace or contains symlinks pointing
// outside. The returned path is realpath-resolved where possible.
func safeResolve(root, rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty reference path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("reference path %q is absolute; must be workspace-relative", rel)
	}
	if strings.HasPrefix(rel, "~") {
		return "", fmt.Errorf("reference path %q expands a user home", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reference path %q escapes workspace", rel)
	}
	joined := filepath.Join(root, clean)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %v", rel, err)
	}
	relToRoot, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("relativize %q: %v", rel, err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reference path %q escapes workspace", rel)
	}
	// Refuse any literal ".." segments that survive cleaning.
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("reference path %q contains parent-relative segment", rel)
		}
	}
	return abs, nil
}

// pathInside reports whether target resolves inside root (lexical, after
// cleaning). Both arguments should be absolute.
func pathInside(target, root string) bool {
	clean := filepath.Clean(target)
	rootClean := filepath.Clean(root)
	if clean == rootClean {
		return true
	}
	prefix := rootClean + string(filepath.Separator)
	return strings.HasPrefix(clean, prefix)
}

// outputInsideInput reports whether an output directory sits inside the
// input workspace, which would cause recursive copies. Refused by bundle.
func outputInsideInput(output, input string) bool {
	out, err := filepath.Abs(output)
	if err != nil {
		return false
	}
	in, err := filepath.Abs(input)
	if err != nil {
		return false
	}
	return pathInside(out, in)
}

// fileExists reports whether path exists and is a regular file (no dir,
// no device, no symlink traversal).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}