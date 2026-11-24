package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	codexDedicatedImageReplayTTL        = 7 * 24 * time.Hour
	codexDedicatedImageReplayMaxEntries = 65536
)

func (b *CodexDedicatedImageBridge) buildDedicatedImageFunctionCallOutput(plan *codexDedicatedImagePlan) json.RawMessage {
	callID := codexDedicatedImagePlannerToolName
	if plan != nil && strings.TrimSpace(plan.CallID) != "" {
		callID = strings.TrimSpace(plan.CallID)
	}
	output := "Dedicated image generation completed. The generated image was returned to the user."
	if plan != nil {
		output = fmt.Sprintf(
			"Dedicated image generation completed for model %s at %s. The generated image was returned to the user.",
			normalizeDedicatedImageModel(plan.Model), dedicatedImageTierForModel(plan.Model),
		)
	}
	value := map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  output,
	}
	raw, _ := json.Marshal(value)
	return raw
}

func (b *CodexDedicatedImageBridge) buildDedicatedImageFunctionCallContext(plan *codexDedicatedImagePlan) json.RawMessage {
	callID := codexDedicatedImagePlannerToolName
	if plan != nil && strings.TrimSpace(plan.CallID) != "" {
		callID = strings.TrimSpace(plan.CallID)
	}
	arguments := map[string]any{}
	if plan != nil {
		arguments["prompt"] = plan.Prompt
		arguments["model"] = normalizeDedicatedImageModel(plan.Model)
		if strings.TrimSpace(plan.VisualPrompt) != "" {
			arguments["visual_prompt"] = plan.VisualPrompt
		}
		if strings.TrimSpace(plan.Title) != "" {
			arguments["title"] = plan.Title
		}
		if strings.TrimSpace(plan.Summary) != "" {
			arguments["summary"] = plan.Summary
		}
		if len(plan.Sections) > 0 {
			arguments["sections"] = plan.Sections
		}
		if len(plan.Relationships) > 0 {
			arguments["relationships"] = plan.Relationships
		}
		if len(plan.MustInclude) > 0 {
			arguments["must_include"] = plan.MustInclude
		}
		if len(plan.MustNotInvent) > 0 {
			arguments["must_not_invent"] = plan.MustNotInvent
		}
		if strings.TrimSpace(plan.Layout) != "" {
			arguments["layout"] = plan.Layout
		}
		if strings.TrimSpace(plan.Language) != "" {
			arguments["language"] = plan.Language
		}
		if strings.TrimSpace(plan.Resolution) != "" {
			arguments["resolution"] = plan.Resolution
		}
		if strings.TrimSpace(plan.AspectRatio) != "" {
			arguments["aspect_ratio"] = plan.AspectRatio
		}
		if strings.TrimSpace(plan.Size) != "" {
			arguments["size"] = plan.Size
		}
		if strings.TrimSpace(plan.Quality) != "" {
			arguments["quality"] = plan.Quality
		}
	}
	argumentBytes, _ := json.Marshal(arguments)
	value := map[string]any{
		"type":      "function_call",
		"id":        callID,
		"call_id":   callID,
		"name":      codexDedicatedImagePlannerToolName,
		"arguments": string(argumentBytes),
		"status":    "completed",
	}
	raw, _ := json.Marshal(value)
	return raw
}

func (b *CodexDedicatedImageBridge) rememberDedicatedImageReplay(ctx context.Context, responseID, upstreamResponseID string, plan *codexDedicatedImagePlan, groupID *int64, accountID int64) error {
	if b == nil {
		return nil
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil
	}
	replay := codexDedicatedImageReplay{
		UpstreamResponseID: strings.TrimSpace(upstreamResponseID),
		FunctionCallOutput: b.buildDedicatedImageFunctionCallOutput(plan),
		ExpiresAt:          time.Now().Add(codexDedicatedImageReplayTTL),
	}
	if b.replayStore != nil {
		raw, err := json.Marshal(replay)
		if err != nil {
			return err
		}
		if err := b.replayStore.SetCodexDedicatedImageReplay(ctx, responseID, raw, codexDedicatedImageReplayTTL); err != nil {
			return fmt.Errorf("persist Codex image replay: %w", err)
		}
	}
	b.replayMu.Lock()
	if b.replays == nil {
		b.replays = make(map[string]codexDedicatedImageReplay)
	}
	if len(b.replays) >= codexDedicatedImageReplayMaxEntries {
		for key := range b.replays {
			delete(b.replays, key)
			break
		}
	}
	b.replays[responseID] = replay
	b.replayMu.Unlock()
	// The public synthetic response ID is also used by the normal scheduler on
	// the next HTTP turn. Bind it to the original planner account so that a
	// follow-up text request cannot land on a different general account before
	// resolveDedicatedImageReplay restores the real upstream response ID.
	if b.gateway != nil && groupID != nil && *groupID > 0 && accountID > 0 {
		store := b.gateway.getOpenAIWSStateStore()
		if store != nil {
			bindingTTL := b.gateway.openAIWSResponseStickyTTL()
			if bindingTTL < codexDedicatedImageReplayTTL {
				bindingTTL = codexDedicatedImageReplayTTL
			}
			if err := store.BindResponseAccount(ctx, *groupID, responseID, accountID, bindingTTL); err != nil {
				return fmt.Errorf("persist Codex image planner account binding: %w", err)
			}
		}
	}
	return nil
}

func (b *CodexDedicatedImageBridge) dedicatedImageReplay(ctx context.Context, responseID string) (codexDedicatedImageReplay, bool, error) {
	if b == nil {
		return codexDedicatedImageReplay{}, false, nil
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return codexDedicatedImageReplay{}, false, nil
	}
	b.replayMu.RLock()
	replay, ok := b.replays[responseID]
	b.replayMu.RUnlock()
	if ok {
		if !replay.ExpiresAt.IsZero() && time.Now().After(replay.ExpiresAt) {
			b.replayMu.Lock()
			delete(b.replays, responseID)
			b.replayMu.Unlock()
			if b.replayStore != nil {
				_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
			}
			return codexDedicatedImageReplay{}, false, nil
		}
		return replay, true, nil
	}
	if b.replayStore == nil {
		return codexDedicatedImageReplay{}, false, nil
	}
	raw, err := b.replayStore.GetCodexDedicatedImageReplay(ctx, responseID)
	if err != nil {
		if errors.Is(err, ErrCodexDedicatedImageReplayNotFound) {
			return codexDedicatedImageReplay{}, false, nil
		}
		return codexDedicatedImageReplay{}, false, fmt.Errorf("load Codex image replay: %w", err)
	}
	if len(raw) == 0 {
		_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
		return codexDedicatedImageReplay{}, false, ErrCodexDedicatedImageReplayCorrupt
	}
	if err := json.Unmarshal(raw, &replay); err != nil {
		_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
		return codexDedicatedImageReplay{}, false, ErrCodexDedicatedImageReplayCorrupt
	}
	if !replay.ExpiresAt.IsZero() && time.Now().After(replay.ExpiresAt) {
		_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
		return codexDedicatedImageReplay{}, false, nil
	}
	b.replayMu.Lock()
	if b.replays == nil {
		b.replays = make(map[string]codexDedicatedImageReplay)
	}
	if len(b.replays) < codexDedicatedImageReplayMaxEntries {
		b.replays[responseID] = replay
	}
	b.replayMu.Unlock()
	return replay, true, nil
}

func (b *CodexDedicatedImageBridge) resolveDedicatedImageReplay(ctx context.Context, body []byte) ([]byte, error) {
	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if previousResponseID == "" {
		return body, nil
	}
	replay, ok, err := b.dedicatedImageReplay(ctx, previousResponseID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return body, nil
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil || root == nil {
		return body, nil
	}
	delete(root, "previous_response_id")
	if replay.UpstreamResponseID != "" {
		root["previous_response_id"] = replay.UpstreamResponseID
	}
	var input []any
	if current, ok := root["input"]; ok {
		switch value := current.(type) {
		case []any:
			input = append(input, value...)
		case nil:
		default:
			input = append(input, value)
		}
	}
	var functionCallOutput any
	if len(replay.FunctionCallOutput) > 0 && json.Unmarshal(replay.FunctionCallOutput, &functionCallOutput) == nil {
		input = append([]any{functionCallOutput}, input...)
	}
	root["input"] = input
	resolved, err := json.Marshal(root)
	if err != nil {
		return body, nil
	}
	return resolved, nil
}

func hasCodexDedicatedImageReplayReference(body []byte) bool {
	responseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	return strings.HasPrefix(responseID, codexDedicatedImageResponsePrefix)
}

func codexStringValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
