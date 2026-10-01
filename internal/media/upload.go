package media

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxUploadBytes 是使用者上傳單檔上限（與 agent 截圖上限一致）。
const MaxUploadBytes = maxImageBytes

// WorkspaceDir 是 runtime 工作區，相對於工作目錄（桌面版為 exe 旁）。
// 上傳檔放這裡，不寫進使用者專案（work_dir）；也不放 internal/static/uploads——
// 那裡掛在 app.Static，未驗證就能讀。測試可覆寫。
var WorkspaceDir = "workspace"

// allowedUploadExt 是可上傳的副檔名白名單（小寫、含點）。
var allowedUploadExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true,
	".pdf": true, ".txt": true, ".md": true, ".log": true, ".json": true, ".csv": true,
}

// SaveUpload 把使用者上傳的檔案存到 <WorkspaceDir>/uploads/<sessionID>/<隨機名>.<ext>，回傳絕對路徑。
// sessionID 必須來自資料庫（已驗證存在），不可直接使用請求參數。
// 檔名一律由伺服器產生（原檔名只取副檔名），杜絕路徑穿越；超過上限或不在白名單即拒絕。
func SaveUpload(sessionID, origName string, r io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(filepath.Base(origName)))
	if !allowedUploadExt[ext] {
		return "", fmt.Errorf("不支援的檔案類型 %q", ext)
	}
	dir := filepath.Join(WorkspaceDir, "uploads", sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("media: mkdir upload dir: %w", err)
	}
	name, err := randomFilename(ext)
	if err != nil {
		return "", fmt.Errorf("media: generate filename: %w", err)
	}
	dst := filepath.Join(dir, name)
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("media: create upload file: %w", err)
	}
	// 多讀 1 byte 判斷是否超過上限（multipart 的宣告大小不可信）。
	n, copyErr := io.Copy(f, io.LimitReader(r, MaxUploadBytes+1))
	closeErr := f.Close()
	if copyErr == nil && n > MaxUploadBytes {
		copyErr = fmt.Errorf("檔案超過上限 %d MB", MaxUploadBytes/1024/1024)
	}
	if copyErr == nil && n == 0 {
		copyErr = fmt.Errorf("檔案是空的")
	}
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.Remove(dst)
		return "", copyErr
	}
	return filepath.Abs(dst)
}
