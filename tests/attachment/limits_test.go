package attachment_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/attachment"
	"github.com/egoist/mygo/transfer"
)

func TestValidatePathsEnforcesEntryAndByteLimits(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	if err := os.WriteFile(first, []byte("1234"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("5678"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (attachment.Limits{MaxBytes: 8, MaxEntries: 2}).ValidatePaths([]string{first, second}); err != nil {
		t.Fatalf("valid paths rejected: %v", err)
	}
	if err := (attachment.Limits{MaxBytes: 8, MaxEntries: 1}).ValidatePaths([]string{first, second}); !errors.Is(err, attachment.ErrTooMany) {
		t.Fatalf("entry overflow = %v, want ErrTooMany", err)
	}
	if err := (attachment.Limits{MaxBytes: 7, MaxEntries: 2}).ValidatePaths([]string{first, second}); !errors.Is(err, attachment.ErrTooLarge) {
		t.Fatalf("byte overflow = %v, want ErrTooLarge", err)
	}
}

func TestValidatePathsRejectsMissingDirectoriesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	limits := attachment.Limits{MaxBytes: 64, MaxEntries: 2}
	if err := limits.ValidatePaths([]string{filepath.Join(root, "missing")}); !errors.Is(err, attachment.ErrInvalidFile) {
		t.Fatalf("missing file = %v, want ErrInvalidFile", err)
	}
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := limits.ValidatePaths([]string{directory}); !errors.Is(err, attachment.ErrInvalidFile) {
		t.Fatalf("directory = %v, want ErrInvalidFile", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := limits.ValidatePaths([]string{link}); !errors.Is(err, attachment.ErrInvalidFile) {
		t.Fatalf("symlink = %v, want ErrInvalidFile", err)
	}
}

func TestValidateDataAllowsTextAndBoundsImagesAndFiles(t *testing.T) {
	limits := attachment.Limits{MaxBytes: 4, MaxEntries: 2}
	if err := limits.ValidateData(transfer.TextData("plain prompt")); err != nil {
		t.Fatalf("text paste rejected: %v", err)
	}
	image := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte{1, 2, 3, 4})))
	if !attachment.ContainsPayload(image) {
		t.Fatal("PNG payload was not identified")
	}
	if err := limits.ValidateData(image); err != nil {
		t.Fatalf("bounded image rejected: %v", err)
	}
	tooLarge := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte{1, 2, 3, 4, 5})))
	if err := limits.ValidateData(tooLarge); !errors.Is(err, attachment.ErrTooLarge) {
		t.Fatalf("large image = %v, want ErrTooLarge", err)
	}
	emptyImage := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, nil)))
	if err := limits.ValidateData(emptyImage); !errors.Is(err, attachment.ErrInvalidData) {
		t.Fatalf("empty image = %v, want ErrInvalidData", err)
	}
}

func TestValidateDataRejectsFilePayloadWithoutAnExactLimit(t *testing.T) {
	file := filepath.Join(t.TempDir(), "attachment.txt")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := transfer.FileData(file)
	if err != nil {
		t.Fatal(err)
	}
	if !attachment.ContainsPayload(data) {
		t.Fatal("file payload was not identified")
	}
	if err := (attachment.Limits{}).ValidateData(data); !errors.Is(err, attachment.ErrInvalidLimits) {
		t.Fatalf("missing limits = %v, want ErrInvalidLimits", err)
	}
}

func TestValidateDataRejectsUndeclaredCustomRepresentations(t *testing.T) {
	custom := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte("safe text")),
		transfer.Bytes(transfer.Format("application/x-agent-secret"), []byte("secret")),
	))
	if !attachment.ContainsPayload(custom) {
		t.Fatal("custom representation was not classified as a payload")
	}
	if err := (attachment.Limits{MaxBytes: 1024, MaxEntries: 1}).ValidateData(custom); !errors.Is(err, attachment.ErrInvalidData) {
		t.Fatalf("custom representation error = %v, want ErrInvalidData", err)
	}
}
