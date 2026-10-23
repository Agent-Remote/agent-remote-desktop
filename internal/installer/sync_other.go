//go:build !darwin && !linux

package installer

// Windows uses NTFS write ordering for the rename operations here and does
// not expose a portable directory fsync through the Go standard library.
// File contents are still flushed by syncTree on Unix; this is a no-op on
// unsupported build hosts so the release helper remains cross-compilable.
func syncDirectory(string) error { return nil }
