package kiro

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/proc"
)

// ModelEntry 是一個可選模型。
type ModelEntry struct {
	ModelID string
	Label   string
}

type listModelsOutput struct {
	Models []struct {
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
	} `json:"models"`
}

// FetchModelOptions 呼叫 `kiro-cli chat --list-models --format json` 取得目前可用模型。
// kiroacp（ACP 協定版）與此共用同一個 kiro-cli 二進位，模型清單視為相同，故也共用本函式。
func FetchModelOptions(ctx context.Context) ([]ModelEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kiro-cli", "chat", "--list-models", "--format", "json")
	cmd.SysProcAttr = proc.SysProcAttr()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var parsed listModelsOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	entries := make([]ModelEntry, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		entries = append(entries, ModelEntry{ModelID: m.ModelID, Label: m.ModelName})
	}
	return entries, nil
}
