package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UploadPath only accepts server-generated names, never client filesystem paths.
func UploadPath(sessionID, storageName string) (string, error) {
	if sessionID == "" || sessionID == "." || sessionID == ".." || strings.ContainsAny(sessionID, `/\:`) ||
		storageName == "" || storageName == "." || storageName == ".." || strings.ContainsAny(storageName, `/\:`) {
		return "", fmt.Errorf("附件路徑無效")
	}
	return filepath.Abs(filepath.Join(WorkspaceDir, "uploads", sessionID, storageName))
}

func ExistingUploadPath(sessionID, storageName string) (string, error) {
	p, err := UploadPath(sessionID, storageName)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("附件無法讀取，請重新上傳")
	}
	// A symlinked workspace is supported; nested redirects out of its session aren't.
	root, err := filepath.Abs(WorkspaceDir)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("附件無法讀取")
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil || !strings.EqualFold(filepath.Dir(real), filepath.Join(root, "uploads", sessionID)) {
		return "", fmt.Errorf("附件路徑無效")
	}
	return p, nil
}
