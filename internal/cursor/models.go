package cursor

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/proc"
)

// ModelEntry 是一個可選模型。
type ModelEntry struct {
	ModelID string
	Label   string
}

// FetchModelOptions 呼叫 `cursor-agent --list-models` 並解析純文字輸出。
// 輸出格式為每行 "<model_id> - <label>"，部分行帶 " (current)" 尾綴會一併去掉。
func FetchModelOptions(ctx context.Context) ([]ModelEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "cursor-agent", "--list-models")
	cmd.SysProcAttr = proc.SysProcAttr()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var entries []ModelEntry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		id, label, ok := strings.Cut(line, " - ")
		if !ok || id == "" {
			continue
		}
		label = strings.TrimSuffix(strings.TrimSpace(label), " (current)")
		entries = append(entries, ModelEntry{ModelID: strings.TrimSpace(id), Label: label})
	}
	return entries, scanner.Err()
}
