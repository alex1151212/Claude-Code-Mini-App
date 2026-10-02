package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadPathRejectsTraversalAndMissingFiles(t *testing.T) {
	old := WorkspaceDir
	WorkspaceDir = t.TempDir()
	t.Cleanup(func() { WorkspaceDir = old })
	for _, pair := range [][2]string{{"..", "a.txt"}, {"session", "../a.txt"}, {"session", `..\a.txt`}, {"C:", "a.txt"}, {"session", "."}} {
		if _, err := UploadPath(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted traversal: %v", pair)
		}
	}
	p, err := SaveUpload("session", "a.txt", strings.NewReader("file"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExistingUploadPath("session", filepath.Base(p))
	if err != nil || got != p {
		t.Fatalf("got=%s err=%v", got, err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if _, err := ExistingUploadPath("session", filepath.Base(p)); err == nil {
		t.Fatal("accepted missing file")
	}
}
