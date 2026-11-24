package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func writeCodexDedicatedImageSSE(c *gin.Context, response, item map[string]any) error {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	events, err := buildCodexDedicatedImageEvents(response, item)
	if err != nil {
		return err
	}
	for _, event := range events {
		raw, err := json.Marshal(event.data)
		if err != nil {
			return err
		}
		if object, ok := event.data.(map[string]any); ok {
			object["sequence_number"] = event.sequence
			raw, err = json.Marshal(object)
			if err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.kind, raw); err != nil {
			return err
		}
	}
	return nil
}

type codexDedicatedImageEvent struct {
	kind     string
	data     any
	sequence int
}

func buildCodexDedicatedImageEvents(response, item map[string]any) ([]codexDedicatedImageEvent, error) {
	if response == nil || item == nil {
		return nil, errors.New("codex image response is incomplete")
	}
	created := cloneMap(response)
	created["status"] = "in_progress"
	created["output"] = []any{}
	events := []codexDedicatedImageEvent{
		{kind: "response.created", data: map[string]any{"type": "response.created", "response": created}, sequence: 0},
		{kind: "response.in_progress", data: map[string]any{"type": "response.in_progress", "response": created}, sequence: 1},
	}
	outputItems, _ := response["output"].([]any)
	sequence := 2
	for index, rawOutput := range outputItems {
		outputItem, ok := rawOutput.(map[string]any)
		if !ok || strings.TrimSpace(codexStringValue(outputItem["type"])) != "message" {
			continue
		}
		added := map[string]any{"type": "response.output_item.added", "output_index": index, "item": map[string]any{
			"id": outputItem["id"], "type": "message", "role": "assistant", "status": "in_progress",
		}}
		done := map[string]any{"type": "response.output_item.done", "output_index": index, "item": outputItem}
		events = append(events,
			codexDedicatedImageEvent{kind: "response.output_item.added", data: added, sequence: sequence},
			codexDedicatedImageEvent{kind: "response.output_item.done", data: done, sequence: sequence + 1},
		)
		sequence += 2
	}
	imageIndex := len(outputItems) - 1
	if imageIndex < 0 {
		imageIndex = 0
	}
	imageID := codexStringValue(item["id"])
	added := map[string]any{"type": "response.output_item.added", "output_index": imageIndex, "item": map[string]any{
		"id": item["id"], "type": "image_generation_call", "status": "in_progress",
	}}
	imageInProgress := map[string]any{
		"type": "response.image_generation_call.in_progress", "output_index": imageIndex, "item_id": imageID,
	}
	imageGenerating := map[string]any{
		"type": "response.image_generation_call.generating", "output_index": imageIndex, "item_id": imageID,
	}
	done := map[string]any{"type": "response.output_item.done", "output_index": imageIndex, "item": item}
	imageCompleted := map[string]any{
		"type": "response.image_generation_call.completed", "output_index": imageIndex, "item_id": imageID,
	}
	completed := map[string]any{"type": "response.completed", "response": response}
	events = append(events,
		codexDedicatedImageEvent{kind: "response.output_item.added", data: added, sequence: sequence},
		codexDedicatedImageEvent{kind: "response.image_generation_call.in_progress", data: imageInProgress, sequence: sequence + 1},
		codexDedicatedImageEvent{kind: "response.image_generation_call.generating", data: imageGenerating, sequence: sequence + 2},
		codexDedicatedImageEvent{kind: "response.image_generation_call.completed", data: imageCompleted, sequence: sequence + 3},
		codexDedicatedImageEvent{kind: "response.output_item.done", data: done, sequence: sequence + 4},
		codexDedicatedImageEvent{kind: "response.completed", data: completed, sequence: sequence + 5},
	)
	return events, nil
}

func codexDedicatedImageMessageItem(responseID string, index int, text string, completed bool) map[string]any {
	status := "in_progress"
	if completed {
		status = "completed"
	}
	return map[string]any{
		"id": "msg_" + responseID + "_" + strconv.Itoa(index), "type": "message", "role": "assistant", "status": status,
		"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
	}
}

func buildCodexDedicatedImageEventPayloads(response, item map[string]any) ([]json.RawMessage, error) {
	events, err := buildCodexDedicatedImageEvents(response, item)
	if err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, 0, len(events))
	for _, event := range events {
		data := event.data
		if object, ok := event.data.(map[string]any); ok {
			cloned := cloneMap(object)
			cloned["sequence_number"] = event.sequence
			data = cloned
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		result = append(result, raw)
	}
	return result, nil
}

func extractCodexPlannerEventPayloads(raw, requestBody []byte) ([]json.RawMessage, error) {
	if codexDedicatedImageRequestStream(requestBody) {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		result := make([]json.RawMessage, 0)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			if !json.Valid([]byte(payload)) {
				return nil, errors.New("codex planner returned invalid SSE JSON")
			}
			result = append(result, json.RawMessage(payload))
		}
		return result, scanner.Err()
	}
	if !json.Valid(raw) {
		return nil, errors.New("codex planner returned invalid JSON")
	}
	return []json.RawMessage{json.RawMessage(bytes.TrimSpace(raw))}, nil
}

// extractCodexPlannerReplayInput returns the completed planner output items
// that must be carried into the next HTTP-bridged WebSocket turn. This is
// required when the selected general account is an OpenAI-compatible account
// whose gateway path does not retain Responses state upstream: the next turn
// may contain only a previous_response_id and the new user input.
func extractCodexPlannerReplayInput(raw, requestBody []byte) ([]json.RawMessage, error) {
	seen := make(map[string]struct{})
	result := make([]json.RawMessage, 0)
	appendItem := func(item json.RawMessage) {
		item = json.RawMessage(bytes.TrimSpace(item))
		if len(item) == 0 || !json.Valid(item) {
			return
		}
		itemType := strings.TrimSpace(gjson.GetBytes(item, "type").String())
		if !isCodexPlannerReplayItemType(itemType) {
			return
		}
		key := strings.TrimSpace(gjson.GetBytes(item, "id").String())
		if key == "" {
			key = strings.TrimSpace(gjson.GetBytes(item, "call_id").String())
		}
		if key == "" {
			key = string(item)
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	appendOutput := func(value gjson.Result) {
		if !value.Exists() || !value.IsArray() {
			return
		}
		for _, item := range value.Array() {
			appendItem(json.RawMessage(item.Raw))
		}
	}

	if codexDedicatedImageRequestStream(requestBody) {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" || !json.Valid([]byte(payload)) {
				continue
			}
			value := gjson.Parse(payload)
			if value.Get("type").String() == "response.output_item.done" {
				appendItem(json.RawMessage(value.Get("item").Raw))
			}
			if value.Get("type").String() == "response.completed" {
				appendOutput(value.Get("response.output"))
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return result, nil
	}
	if !json.Valid(raw) {
		return nil, errors.New("codex planner returned invalid JSON")
	}
	appendOutput(gjson.GetBytes(raw, "output"))
	return result, nil
}

func isCodexPlannerReplayItemType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "message", "reasoning", "function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output", "tool_search_call", "tool_search_output", "mcp_tool_call", "mcp_tool_call_output":
		return true
	default:
		return false
	}
}

func cloneGinContextForCodexPlanner(parent *gin.Context, ctx context.Context, body []byte, recorder *httptest.ResponseRecorder) (*gin.Context, error) {
	if parent == nil || parent.Request == nil {
		return nil, errors.New("codex planner request context is unavailable")
	}
	planner, _ := gin.CreateTestContext(recorder)
	planner.Request = parent.Request.Clone(ctx)
	planner.Request.Body = io.NopCloser(bytes.NewReader(body))
	planner.Request.ContentLength = int64(len(body))
	planner.Params = append(planner.Params[:0], parent.Params...)
	for key, value := range parent.Keys {
		planner.Set(key, value)
	}
	return planner, nil
}

func replayCodexPlannerResponse(c *gin.Context, recorder *httptest.ResponseRecorder) error {
	if c == nil || recorder == nil {
		return errors.New("codex planner response context is unavailable")
	}
	for key, values := range recorder.Header() {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	status := recorder.Code
	if status == 0 {
		status = http.StatusOK
	}
	c.Status(status)
	_, err := c.Writer.Write(recorder.Body.Bytes())
	return err
}

func writeJSONBytes(c *gin.Context, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.Status(http.StatusOK)
	_, err = c.Writer.Write(raw)
	return err
}
