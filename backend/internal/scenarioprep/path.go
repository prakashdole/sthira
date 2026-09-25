package scenarioprep

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// sha256Hex returns the lowercase hex SHA256 of body.
func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// readBoundedFile reads at most maxBytes from path.
// It stats the file first using os.Stat (and os.Lstat) to verify it is a
// regular file and that its size does not exceed maxBytes, preventing
// unbounded memory allocations before decoding.
func readBoundedFile(path string, maxBytes int64) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIO, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrIO, path)
	}
	if fi.Size() > maxBytes {
		return nil, fmt.Errorf("%w: file size %d bytes exceeds limit %d bytes", ErrBoundedRead, fi.Size(), maxBytes)
	}
	f, err := os.Open(filepath.Clean(path)) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()

	// Read with LimitReader capped at maxBytes + 1 to detect dynamic growth or pseudo-files.
	lr := io.LimitReader(f, maxBytes+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIO, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: file size exceeds limit %d bytes", ErrBoundedRead, maxBytes)
	}
	return body, nil
}

// safeResolve returns the canonical absolute path of rel inside root after
// rejecting any path that escapes the workspace or contains symlinks pointing
// outside. It inspects every path component (parent directories and leaf)
// to prevent symlink traversal escapes.
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
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".." {
			return "", fmt.Errorf("reference path %q contains parent-relative segment", rel)
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("workspace absolute path: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("workspace root: %w", err)
	}

	joined := filepath.Join(realRoot, clean)
	if !pathInside(joined, realRoot) {
		return "", fmt.Errorf("reference path %q escapes workspace", rel)
	}

	// Step through each component of clean from realRoot, checking Lstat.
	// If any component is a symlink, evaluate it and verify that its target
	// remains strictly inside realRoot.
	segs := strings.Split(filepath.ToSlash(clean), "/")
	curr := realRoot
	for _, seg := range segs {
		if seg == "" || seg == "." {
			continue
		}
		curr = filepath.Join(curr, seg)
		fi, err := os.Lstat(curr)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("path component %q does not exist: %w", seg, os.ErrNotExist)
			}
			return "", fmt.Errorf("stat path component %q: %w", seg, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(curr)
			if err != nil {
				return "", fmt.Errorf("%w: evaluating symlink %q: %v", ErrUnsafeSymlink, seg, err)
			}
			realResolved, err := filepath.Abs(resolved)
			if err != nil || !pathInside(realResolved, realRoot) {
				return "", fmt.Errorf("%w: symlink %q escapes workspace to %q", ErrUnsafeSymlink, seg, resolved)
			}
			curr = realResolved
		}
	}

	// Verify the final canonical path
	finalReal, err := filepath.EvalSymlinks(curr)
	if err != nil {
		return "", fmt.Errorf("resolving path %q: %w", clean, err)
	}
	realTarget, err := filepath.Abs(finalReal)
	if err != nil || !pathInside(realTarget, realRoot) {
		return "", fmt.Errorf("%w: path %q resolves outside workspace to %q", ErrUnsafeSymlink, rel, finalReal)
	}

	// Ensure the leaf exists and is a regular file
	info, err := os.Stat(finalReal)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("reference path %q is not a regular file", rel)
	}

	return finalReal, nil
}

// pathInside reports whether target resolves inside root (lexical, after
// cleaning). Both arguments should be absolute and evaluated.
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
	if realIn, err := filepath.EvalSymlinks(in); err == nil {
		in = realIn
	}
	if realOut, err := filepath.EvalSymlinks(out); err == nil {
		out = realOut
	} else if realParent, err := filepath.EvalSymlinks(filepath.Dir(out)); err == nil {
		out = filepath.Join(realParent, filepath.Base(out))
	}
	return pathInside(out, in)
}

// destinationExists reports whether path exists on disk (as a file,
// directory, symlink, or other special file).
func destinationExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
