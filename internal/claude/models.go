package claude

// ModelEntry 是一個可選模型（含空字串代表「預設」）。
type ModelEntry struct {
	ModelID string
	Label   string
}

// ModelOptions 回傳 Claude CLI 可用的模型別名。
// ponytail: Claude CLI 無 list-models 指令，手動維護；Anthropic 出新一代模型要手動加。
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
