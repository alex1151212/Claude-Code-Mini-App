package ws

import (
	"strings"
	"time"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/model"
)

// resolveStreamless 解析 stream 不含 model 的 agent（codex / kiro）實際會用的 model：
// 下拉選的 sess.Model 優先（與 run 時 extra[ArgModel] 的優先序一致），否則 cli_extra_args，最後才是全域設定。
func resolveStreamless(sess *db.Session, agentType string) model.Info {
	args := sess.CliExtraArgs
	if m := strings.TrimSpace(sess.Model); m != "" {
		args = []string{"--model", m}
	}
	return model.ResolveForSession(agentType, args, "", "")
}

func isStreamless(agentType string) bool {
	return agentType == agent.TypeCodex || agentType == agent.TypeKiro
}

func sessionModelPayload(sess *db.Session) *model.Payload {
	if sess == nil {
		return nil
	}
	agentType := sess.AgentType
	if agentType == "" {
		agentType = agent.TypeClaude
	}
	if agentType == agent.TypeAntigravity || agentType == agent.TypeGemini {
		return nil
	}
	storedModel, storedSource := sess.ActiveModel, sess.ActiveModelSource
	// 這類 agent 的 ActiveModel 只是推論值（可能是舊版寫入的全域預設），有明確選擇時以選擇為準。
	if isStreamless(agentType) && strings.TrimSpace(sess.Model) != "" {
		storedModel, storedSource = sess.Model, string(model.SourceCliFlag)
	}
	info := model.ResolveForSession(agentType, sess.CliExtraArgs, storedModel, storedSource)
	if !info.Ok && info.DisplayText == "—" {
		return nil
	}
	p := info.ToPayload()
	if sess.ActiveModelAt != "" {
		p.UpdatedAt = sess.ActiveModelAt
	}
	return &p
}

func persistModelUpdate(database *db.DB, sessionID string, snap *agent.ModelSnapshot) model.Payload {
	if snap == nil || snap.DisplayText == "" || snap.DisplayText == "—" {
		return model.Payload{}
	}
	modelName := snap.Model
	if modelName == "" {
		modelName = snap.DisplayText
	}
	_ = database.UpdateSessionActiveModel(sessionID, modelName, snap.Source)
	return model.Payload{
		DisplayText: snap.DisplayText,
		Source:      snap.Source,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

func persistInfoUpdate(database *db.DB, sessionID string, info model.Info) *model.Payload {
	if !info.Ok || info.DisplayText == "" || info.DisplayText == "—" {
		return nil
	}
	modelName := info.Model
	if modelName == "" {
		modelName = info.DisplayText
	}
	_ = database.UpdateSessionActiveModel(sessionID, modelName, string(info.Source))
	p := info.ToPayloadAt(time.Now())
	return &p
}
