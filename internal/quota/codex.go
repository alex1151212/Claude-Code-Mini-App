package quota

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/usage"
)

// CodexFetcher 讀 codex 每回合寫進本機 rollout 檔的 rate_limits（token_count 事件），
// 不呼叫模型、不耗額度。實測 codex-cli 0.159.2。
type CodexFetcher struct{}

func (f *CodexFetcher) Provider() string { return agent.TypeCodex }

func (f *CodexFetcher) Fetch(_ context.Context) (Snapshot, error) {
	return codexSnapshot(latestCodexRateLimits(codexSessionsDir(), time.Now())), nil
}

func codexSnapshot(info *usage.QuotaInfo) Snapshot {
	if info == nil {
		info = &usage.QuotaInfo{Provider: agent.TypeCodex, Source: "codex rollout"}
	}
	return Snapshot{
		Provider:    agent.TypeCodex,
		DisplayText: FormatDisplay(agent.TypeCodex, info),
		UpdatedAt:   time.Now(),
	}
}

func codexSessionsDir() string {
	if h := strings.TrimSpace(os.Getenv("CODEX_HOME")); h != "" {
		return filepath.Join(h, "sessions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "sessions")
}

// latestCodexRateLimits 依 mtime 由新到舊找第一個帶 rate_limits 的 rollout 檔。
// 必須全樹依 mtime 排序：resume 舊 thread 會寫回它建立當天的日期目錄，不能只看今天。
// ponytail: 每次 fetch 全樹 WalkDir，檔案數達數萬才需要改成只掃最近 N 天＋快取。
func latestCodexRateLimits(root string, now time.Time) *usage.QuotaInfo {
	if root == "" {
		return nil
	}
	type rollout struct {
		path string
		mod  time.Time
	}
	var files []rollout
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files = append(files, rollout{p, info.ModTime()})
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	for _, f := range files {
		if q := lastRateLimitsInFile(f.path, now); q != nil {
			return q
		}
	}
	return nil
}

// lastRateLimitsInFile 回傳檔案中最後一筆 rate_limits；用 ReadBytes 不受單行長度限制（工具輸出可能很大）。
// ponytail: 單行記憶體上限＝該行長度（最壞＝整檔）；檔案由本機 codex 產生，視為可信，未設硬上限。
func lastRateLimitsInFile(path string, now time.Time) *usage.QuotaInfo {
	fh, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer fh.Close()
	var last *usage.QuotaInfo
	br := bufio.NewReader(fh)
	marker := []byte(`"rate_limits"`)
	for {
		line, err := br.ReadBytes('\n')
		if bytes.Contains(line, marker) {
			// codex 可能正在寫入，最後一行不完整時解析失敗會被略過。
			if q := usage.FromCodexRolloutLine(line, now); q != nil {
				last = q
			}
		}
		if err != nil {
			break
		}
	}
	return last
}
