// Package attachment contains the local, fail-closed checks for exact remote
// clipboard and file-transfer payloads.  The CLI remains the authority for
// the identity-bound upload, but the desktop must apply the limits it
// observed before offering a payload to the terminal.
package attachment

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/transfer"
)

var (
	ErrInvalidLimits = errors.New("invalid attachment limits")
	ErrTooMany       = errors.New("attachment entry limit exceeded")
	ErrTooLarge      = errors.New("attachment byte limit exceeded")
	ErrInvalidFile   = errors.New("attachment file is unavailable or not regular")
	ErrInvalidData   = errors.New("attachment payload is invalid")
)

// Limits is the v1 exact-transfer declaration copied from the CLI
// capabilities response.  Zero values are intentionally invalid: callers
// must never treat a missing declaration as unlimited.
type Limits struct {
	MaxBytes   int64
	MaxEntries int
}

// ContainsPayload reports whether data carries a file, image, or unsupported
// custom attachment. Plain text and rich-text alternatives remain ordinary
// terminal paste data.
func ContainsPayload(data transfer.Data) bool {
	formats := data.Formats()
	if slices.Contains(formats, transfer.FileList) {
		return true
	}
	for _, format := range formats {
		if isImageFormat(format) {
			return true
		}
		if isUnsupportedFormat(format) {
			return true
		}
	}
	return slices.Contains(formats, transfer.URIList) &&
		!slices.Contains(formats, transfer.Text) &&
		!slices.Contains(formats, transfer.HTML) &&
		!slices.Contains(formats, transfer.Markdown)
}

func (l Limits) Validate() error {
	if l.MaxBytes <= 0 || l.MaxBytes > 256*1024*1024 || l.MaxEntries <= 0 || l.MaxEntries > 32 {
		return ErrInvalidLimits
	}
	return nil
}

// ValidatePaths checks files before constructing a native transfer offer.
// Lstat deliberately rejects symlinks so a later CLI read cannot be redirected
// after the user reviewed a different path.  The exact CLI repeats this check
// at upload time to cover the TOCTOU window between these two operations.
func (l Limits) ValidatePaths(paths []string) error {
	if err := l.Validate(); err != nil {
		return err
	}
	if len(paths) == 0 {
		return ErrInvalidData
	}
	if len(paths) > l.MaxEntries {
		return ErrTooMany
	}
	var total int64
	for _, path := range paths {
		if path == "" || !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			return ErrInvalidFile
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 {
			return ErrInvalidFile
		}
		if info.Size() > l.MaxBytes || total > l.MaxBytes-info.Size() {
			return ErrTooLarge
		}
		total += info.Size()
	}
	return nil
}

// ValidateData validates file and image representations in a native clipboard
// payload. Text-only data is accepted without requiring exact-transfer
// limits; callers can therefore keep ordinary terminal copy/paste behavior.
// A payload containing a file or image is either fully within the declaration
// or rejected. It is never reduced to text, since that could silently turn an
// attachment into an unintended shell command. Unknown custom representations
// are rejected because the exact bridge does not declare them.
func (l Limits) ValidateData(data transfer.Data) error {
	if err := l.Validate(); err != nil {
		return err
	}
	formats := data.Formats()
	hasText := slices.Contains(formats, transfer.Text) || slices.Contains(formats, transfer.HTML) || slices.Contains(formats, transfer.Markdown)
	hasFiles := slices.Contains(formats, transfer.FileList)
	hasURIList := slices.Contains(formats, transfer.URIList)
	hasUnsupported := false
	hasImage := false
	for _, format := range formats {
		if isImageFormat(format) {
			hasImage = true
			continue
		}
		if isUnsupportedFormat(format) {
			hasUnsupported = true
		}
	}
	if hasUnsupported {
		return ErrInvalidData
	}
	if !hasFiles && !hasImage && !(hasURIList && !hasText) {
		return nil
	}

	entries := 0
	var total int64
	if hasFiles || (hasURIList && !hasText) {
		files, err := data.Files()
		if err != nil || len(files) == 0 {
			return fmt.Errorf("%w: file list", ErrInvalidData)
		}
		if err := l.ValidatePaths(files); err != nil {
			return err
		}
		entries += len(files)
		for _, path := range files {
			info, err := os.Lstat(path)
			if err != nil {
				return ErrInvalidFile
			}
			if total > math.MaxInt64-info.Size() {
				return ErrTooLarge
			}
			total += info.Size()
		}
	}
	if hasImage {
		for _, item := range data.Items() {
			var largest int64
			found := false
			for _, format := range item.Formats() {
				if !isImageFormat(format) {
					continue
				}
				found = true
				bytes, err := item.Read(format)
				if err != nil {
					return fmt.Errorf("%w: image: %v", ErrInvalidData, err)
				}
				if len(bytes) == 0 {
					return fmt.Errorf("%w: empty image", ErrInvalidData)
				}
				if int64(len(bytes)) > largest {
					largest = int64(len(bytes))
				}
			}
			if !found {
				continue
			}
			if largest > l.MaxBytes || total > l.MaxBytes-largest {
				return ErrTooLarge
			}
			total += largest
			entries++
		}
	}
	if entries == 0 {
		return ErrInvalidData
	}
	if entries > l.MaxEntries {
		return ErrTooMany
	}
	if total > l.MaxBytes {
		return ErrTooLarge
	}
	return nil
}

func isImageFormat(format transfer.Format) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(format))), "image/")
}

func isUnsupportedFormat(format transfer.Format) bool {
	return format != transfer.Text && format != transfer.HTML && format != transfer.Markdown && format != transfer.URIList && format != transfer.FileList && !isImageFormat(format)
}
