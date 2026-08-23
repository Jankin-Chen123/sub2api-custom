package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const openAIResponsesForceFullContextKey = "openai_responses_force_full"

// normalizeOpenAIResponsesLiteTools applies the Responses Lite request
// contract: reasoning must cover all turns, and private namespace declarations
// use the input.additional_tools carrier. Other top-level tools must belong to
// the small set accepted by the Lite endpoint; rejecting unsupported hosted
// tools is intentional because silently dropping them would change behavior.
func normalizeOpenAIResponsesLiteTools(reqBody map[string]any) (bool, error) {
	if reqBody == nil {
		return false, nil
	}
	if rawReasoning, exists := reqBody["reasoning"]; exists && rawReasoning != nil {
		if _, ok := rawReasoning.(map[string]any); !ok {
			return false, fmt.Errorf("responses Lite requires reasoning to be an object")
		}
	}
	rawTools, exists := reqBody["tools"]
	if !exists || rawTools == nil {
		return ensureOpenAIResponsesLiteReasoningContext(reqBody)
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false, fmt.Errorf("responses Lite requires tools to be an array")
	}

	topLevelTools := make([]any, 0, len(tools))
	namespaceTools := make([]any, 0, len(tools))
	for index, rawTool := range tools {
		if customTool, ok := rawTool.(string); ok {
			if strings.TrimSpace(customTool) == "" {
				return false, fmt.Errorf("responses Lite custom tool at index %d must not be empty", index)
			}
			topLevelTools = append(topLevelTools, rawTool)
			continue
		}
		tool, ok := rawTool.(map[string]any)
		if !ok {
			return false, fmt.Errorf("responses Lite tool at index %d must be an object", index)
		}
		toolType := strings.TrimSpace(firstNonEmptyString(tool["type"]))
		switch toolType {
		case "function", "custom", "tool_search":
			topLevelTools = append(topLevelTools, rawTool)
		case "namespace":
			namespaceTools = append(namespaceTools, rawTool)
		case "":
			return false, fmt.Errorf("responses Lite tool at index %d is missing type", index)
		default:
			return false, fmt.Errorf("responses Lite does not support top-level tool type %q at index %d", toolType, index)
		}
	}
	if len(namespaceTools) == 0 {
		return ensureOpenAIResponsesLiteReasoningContext(reqBody)
	}

	input, err := appendOpenAIResponsesLiteAdditionalTools(reqBody["input"], namespaceTools)
	if err != nil {
		return false, err
	}
	if _, err := ensureOpenAIResponsesLiteReasoningContext(reqBody); err != nil {
		return false, err
	}
	reqBody["input"] = input
	if len(topLevelTools) == 0 {
		delete(reqBody, "tools")
	} else {
		reqBody["tools"] = topLevelTools
	}
	return true, nil
}

func ensureOpenAIResponsesLiteReasoningContext(reqBody map[string]any) (bool, error) {
	rawReasoning, exists := reqBody["reasoning"]
	if !exists || rawReasoning == nil {
		reqBody["reasoning"] = map[string]any{"context": "all_turns"}
		return true, nil
	}
	reasoning, ok := rawReasoning.(map[string]any)
	if !ok {
		return false, fmt.Errorf("responses Lite requires reasoning to be an object")
	}
	if context, ok := reasoning["context"].(string); ok && context == "all_turns" {
		return false, nil
	}
	reasoning["context"] = "all_turns"
	return true, nil
}

func appendOpenAIResponsesLiteAdditionalTools(input any, namespaceTools []any) ([]any, error) {
	var items []any
	switch typed := input.(type) {
	case nil:
		items = make([]any, 0, 1)
	case string:
		items = []any{map[string]any{
			"type":    "message",
			"role":    "user",
			"content": typed,
		}}
	case []any:
		items = typed
	default:
		return nil, fmt.Errorf("responses Lite namespace tools require input to be a string or array")
	}

	var target map[string]any
	var targetTools []any
	var allAdditionalTools []any
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok || strings.TrimSpace(firstNonEmptyString(item["type"])) != "additional_tools" {
			continue
		}
		rawAdditionalTools, exists := item["tools"]
		additionalTools := []any(nil)
		toolsOK := true
		if exists && rawAdditionalTools != nil {
			additionalTools, toolsOK = rawAdditionalTools.([]any)
		}
		if !toolsOK {
			return nil, fmt.Errorf("responses Lite input.additional_tools tools must be an array")
		}
		if target == nil {
			target = item
			targetTools = additionalTools
		}
		allAdditionalTools = append(allAdditionalTools, additionalTools...)
	}

	merged, err := mergeOpenAIResponsesLiteAdditionalTools(allAdditionalTools, namespaceTools)
	if err != nil {
		return nil, err
	}
	newTools := merged[len(allAdditionalTools):]
	if target != nil {
		if len(newTools) > 0 {
			target["tools"] = append(append([]any(nil), targetTools...), newTools...)
		}
		return items, nil
	}

	items = append(items, map[string]any{
		"type":  "additional_tools",
		"role":  "developer",
		"tools": newTools,
	})
	return items, nil
}

func mergeOpenAIResponsesLiteAdditionalTools(existing []any, moved []any) ([]any, error) {
	merged := append([]any(nil), existing...)
	seen := make(map[string]any, len(existing)+len(moved))
	for _, rawTool := range existing {
		if identity := openAIResponsesLiteToolIdentity(rawTool); identity != "" {
			if previous, exists := seen[identity]; exists && !reflect.DeepEqual(previous, rawTool) {
				return nil, fmt.Errorf("responses Lite additional_tools contains conflicting definitions for %s", openAIResponsesLiteToolIdentityForError(rawTool))
			}
			seen[identity] = rawTool
		}
	}
	for _, rawTool := range moved {
		identity := openAIResponsesLiteToolIdentity(rawTool)
		if identity != "" {
			if previous, exists := seen[identity]; exists {
				if reflect.DeepEqual(previous, rawTool) {
					continue
				}
				return nil, fmt.Errorf("responses Lite additional_tools conflicts with migrated %s", openAIResponsesLiteToolIdentityForError(rawTool))
			}
			seen[identity] = rawTool
		}
		merged = append(merged, rawTool)
	}
	return merged, nil
}

func openAIResponsesLiteToolIdentity(rawTool any) string {
	tool, ok := rawTool.(map[string]any)
	if !ok {
		return ""
	}
	toolType := strings.TrimSpace(firstNonEmptyString(tool["type"]))
	name := strings.TrimSpace(firstNonEmptyString(tool["name"]))
	if toolType == "" || name == "" {
		return ""
	}
	return toolType + "\x00" + name
}

func openAIResponsesLiteToolIdentityForError(rawTool any) string {
	tool, _ := rawTool.(map[string]any)
	return fmt.Sprintf("tool type %q name %q", strings.TrimSpace(firstNonEmptyString(tool["type"])), strings.TrimSpace(firstNonEmptyString(tool["name"])))
}

func normalizeOpenAIResponsesLiteToolsPayload(body []byte) ([]byte, bool, error) {
	var requestBody map[string]any
	if err := json.Unmarshal(body, &requestBody); err != nil {
		return body, false, fmt.Errorf("decode responses Lite request body: %w", err)
	}
	changed, err := normalizeOpenAIResponsesLiteTools(requestBody)
	if err != nil || !changed {
		return body, false, err
	}
	rebuilt, err := marshalOpenAIUpstreamJSON(requestBody)
	if err != nil {
		return body, false, fmt.Errorf("encode responses Lite request body: %w", err)
	}
	return rebuilt, true, nil
}

// openAIResponsesLiteFullProtocolReason returns the capability that cannot be
// represented by Responses Lite. The public Responses contract allows these
// capabilities, so callers must preserve the request and use the full protocol
// instead of silently weakening it for a Lite-only upstream.
func openAIResponsesLiteFullProtocolReason(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	if gjson.GetBytes(body, "parallel_tool_calls").Type == gjson.True {
		return "parallel_tool_calls"
	}

	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return ""
	}
	for _, tool := range tools.Array() {
		if !tool.IsObject() {
			continue
		}
		switch toolType := strings.TrimSpace(tool.Get("type").String()); toolType {
		case "", "function", "custom", "tool_search", "namespace":
			// These are either supported by Lite or malformed. Keep malformed
			// declarations intact so the upstream can return its normal validation.
		default:
			return "tool type " + toolType
		}
	}
	return ""
}

func forceOpenAIFullResponsesForRequest(c *gin.Context) {
	if c != nil {
		c.Set(openAIResponsesForceFullContextKey, true)
	}
}

func isOpenAIFullResponsesForcedForRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	forced, exists := c.Get(openAIResponsesForceFullContextKey)
	value, ok := forced.(bool)
	return exists && ok && value
}

// isOpenAIResponsesLiteTransportRejection matches only a validation response
// that explicitly names the private Lite transport. It is safe to retry these
// responses once without the Lite marker because the upstream rejected the
// protocol before starting model execution.
func isOpenAIResponsesLiteTransportRejection(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest || len(body) == 0 {
		return false
	}

	candidates := [][]byte{body}
	for _, path := range []string{"detail", "error.message", "message"} {
		nested := strings.TrimSpace(gjson.GetBytes(body, path).String())
		if nested != "" && gjson.Valid(nested) {
			candidates = append(candidates, []byte(nested))
		}
	}
	for _, candidate := range candidates {
		if !strings.EqualFold(strings.TrimSpace(extractUpstreamErrorCode(candidate)), "unsupported_value") {
			continue
		}
		message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(candidate)))
		if strings.Contains(message, strings.ToLower(responsesLiteHeader)) {
			return true
		}
	}
	return false
}

// disableOpenAIResponsesLiteWebSocketPayloadWhenFullProtocolRequired removes
// only the private Lite transport marker. All user-facing request fields,
// including parallel_tool_calls and tool declarations, remain unchanged.
func disableOpenAIResponsesLiteWebSocketPayloadWhenFullProtocolRequired(body []byte) ([]byte, string, bool, error) {
	reason := openAIResponsesLiteFullProtocolReason(body)
	if reason == "" || !isOpenAIResponsesLiteWebSocketPayload(body) {
		return body, reason, false, nil
	}
	normalized, err := sjson.DeleteBytes(body, "client_metadata."+responsesLiteWSMetadataKey)
	if err != nil {
		return body, reason, false, fmt.Errorf("remove Responses Lite websocket marker: %w", err)
	}
	return normalized, reason, true, nil
}
