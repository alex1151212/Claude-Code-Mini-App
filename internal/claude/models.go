package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/proc"
)

// ModelEntry 是一個可選模型（含空字串代表「預設」）。
type ModelEntry struct {
	ModelID string
	Label   string
}

const fetchTimeout = 15 * time.Second

// FetchModelOptions 向 claude 要目前登入帳號可用的模型清單（即 /model 選單的內容）。
// 做法：stream-json 輸入模式下送 SDK 控制協定的 initialize，回應的 models 就是清單；
// 不送 prompt，所以不會呼叫模型、不耗額度。這不是公開文件化的 CLI 介面（POC 見 poc/claude-models），
// 回應結構不符預期時回 error，由呼叫端決定 fallback。
func FetchModelOptions(ctx context.Context) ([]ModelEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
		"--no-session-persistence", "--strict-mcp-config")
	cmd.Dir = os.TempDir() // 不載入任何專案的 CLAUDE.md／設定
	cmd.SysProcAttr = proc.SysProcAttr()
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return proc.KillTree(cmd.Process.Pid)
		}
		return nil
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// 拿到回應就收，不等 claude 自己結束（它會一直等下一則 stdin 輸入）。
	defer func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
	}()

	requestID := fmt.Sprintf("models-%d", time.Now().UnixNano())
	req := fmt.Sprintf(`{"type":"control_request","request_id":%q,"request":{"subtype":"initialize"}}`+"\n", requestID)
	if _, err := io.WriteString(stdin, req); err != nil {
		return nil, err
	}

	// 回應是單行大 JSON（連所有 skill 描述都在內，數十 KB），會超過 bufio.Scanner 預設的 64KB 上限。
	r := bufio.NewReader(stdout)
	for {
		line, err := r.ReadBytes('\n')
		if entries, matched, perr := parseInitializeModels(line, requestID); matched {
			return entries, perr
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("claude 結束前沒有回 initialize: %w", err)
		}
	}
}

// parseInitializeModels 解析一行 stdout。matched=false 代表這行不是我們要的 control_response（例如 hook 事件），
// 呼叫端應繼續讀；matched=true 時以 entries／err 為最終結果。
func parseInitializeModels(line []byte, requestID string) (entries []ModelEntry, matched bool, err error) {
	if !bytes.Contains(line, []byte(`"control_response"`)) {
		return nil, false, nil
	}
	var msg struct {
		Type     string `json:"type"`
		Response struct {
			Subtype   string `json:"subtype"`
			RequestID string `json:"request_id"`
			Error     string `json:"error"`
			Response  struct {
				Models []struct {
					Value         string `json:"value"`
					ResolvedModel string `json:"resolvedModel"`
					DisplayName   string `json:"displayName"`
				} `json:"models"`
			} `json:"response"`
		} `json:"response"`
	}
	if json.Unmarshal(line, &msg) != nil || msg.Type != "control_response" || msg.Response.RequestID != requestID {
		return nil, false, nil
	}
	if msg.Response.Subtype != "success" {
		return nil, true, fmt.Errorf("initialize 失敗: %s", msg.Response.Error)
	}
	seen := make(map[string]bool)
	for _, m := range msg.Response.Response.Models {
		// default 只是某個模型（目前是 sonnet）的重複項；同一個 resolvedModel 只留第一筆。
		if m.Value == "default" || m.ResolvedModel == "" || seen[m.ResolvedModel] {
			continue
		}
		seen[m.ResolvedModel] = true
		label := m.DisplayName
		if label == "" {
			label = m.ResolvedModel
		}
		entries = append(entries, ModelEntry{ModelID: m.ResolvedModel, Label: label})
	}
	// 空清單不能當成功：SyncModelOptions 會把該 agent 的選項全刪光。
	if len(entries) == 0 {
		return nil, true, errors.New("initialize 回應沒有 models（claude 版本可能改了協定）")
	}
	return entries, true, nil
}

// ModelOptions 回傳內建的 Claude 模型清單。
// 只當 fallback：FetchModelOptions 抓不到且 DB 還沒有任何 Claude 選項（首次安裝）時拿來當種子。
func ModelOptions() []ModelEntry {
	return []ModelEntry{
		{"claude-fable-5-1", "Fable 5.1"},
		{"claude-opus-5-5", "Opus 5.5"},
		{"claude-sonnet-5-5", "Sonnet 5.5"},
		{"claude-haiku-4-5", "Haiku 4.5"},
		{"claude-fable-5", "Fable 5"},
		{"claude-opus-5", "Opus 5"},
		{"claude-opus-4-8", "Opus 4.8"},
		{"claude-opus-4-7", "Opus 4.7"},
		{"claude-opus-4-6", "Opus 4.6"},
		{"claude-opus-4-5", "Opus 4.5"},
		{"claude-sonnet-5", "Sonnet 5"},
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-sonnet-4-5", "Sonnet 4.5"},
	}
}

// EffortOptions 回傳 --effort 支援的等級。
func EffortOptions() []string {
	return []string{"low", "medium", "high", "xhigh", "max"}
}
