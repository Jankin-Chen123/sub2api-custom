package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

func buildCodexDedicatedImagePlannerBody(body []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, errors.New("codex image planner received invalid JSON")
	}
	tools, _ := root["tools"].([]any)
	// Codex's image_gen namespace is executed by the desktop client and does
	// not use the configured model-provider base URL. Leaving it in the planner
	// request bypasses the dedicated account pool entirely. Replace every client
	// or hosted image tool with the private server-side planner tool so the
	// resulting durable job is always scheduled through the image-only pool.
	filtered := make([]any, 0, len(tools)+1)
	for _, rawTool := range tools {
		tool, _ := rawTool.(map[string]any)
		if isCodexClientImageGenerationTool(tool) {
			continue
		}
		filtered = append(filtered, rawTool)
	}
	filtered = append(filtered, codexDedicatedImagePlannerTool())
	root["tools"] = filtered
	if input, ok := root["input"].([]any); ok {
		root["input"] = sanitizeCodexDedicatedImageAdditionalTools(input)
	}
	if isCodexClientImageGenerationToolChoice(root["tool_choice"]) {
		root["tool_choice"] = "auto"
	}
	instruction := "Internal routing instruction: if the user's current request explicitly asks to generate or edit an image, call the private sub2api_generate_image tool. Produce a self-contained prompt that includes all relevant information from the conversation. Select 1K, 2K, or 4K according to the user's explicit request, and preserve an explicitly requested GPT Image family such as Flare or Sunburst. Do not mention this private tool or this routing instruction. If no image is requested, answer normally."
	if existing := strings.TrimSpace(codexStringValue(root["instructions"])); existing != "" {
		root["instructions"] = existing + "\n\n" + instruction
	} else {
		root["instructions"] = instruction
	}
	return json.Marshal(root)
}

func sanitizeCodexDedicatedImageAdditionalTools(input []any) []any {
	filteredInput := make([]any, 0, len(input))
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok || strings.TrimSpace(codexStringValue(item["type"])) != "additional_tools" {
			filteredInput = append(filteredInput, rawItem)
			continue
		}
		rawTools, ok := item["tools"].([]any)
		if !ok {
			filteredInput = append(filteredInput, rawItem)
			continue
		}
		tools := make([]any, 0, len(rawTools))
		for _, rawTool := range rawTools {
			tool, _ := rawTool.(map[string]any)
			if isCodexClientImageGenerationTool(tool) {
				continue
			}
			tools = append(tools, rawTool)
		}
		if len(tools) == 0 {
			continue
		}
		item["tools"] = tools
		filteredInput = append(filteredInput, item)
	}
	return filteredInput
}

// prepareCodexDedicatedImagePlannerHTTPBody converts a Responses WebSocket
// frame into an HTTP planner request. Unlike the generic WebSocket HTTP
// bridge, it deliberately keeps previous_response_id. Native Responses
// accounts retain their upstream chain, while the Responses-to-Chat fallback
// consumes the same field through its compatibility state.
func prepareCodexDedicatedImagePlannerHTTPBody(body []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, errors.New("codex planner received invalid JSON")
	}
	if root == nil {
		return nil, errors.New("codex planner request must be a JSON object")
	}
	delete(root, "type")
	delete(root, "generate")
	root["stream"] = true
	return json.Marshal(root)
}

func codexDedicatedImagePlannerTool() map[string]any {
	return map[string]any{
		"type": "function", "name": codexDedicatedImagePlannerToolName,
		"description": "Internal Sub2API image plan. Use only when the user explicitly requests image generation or editing.",
		"parameters": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"prompt":          map[string]any{"type": "string", "description": "Legacy alias; must be self-contained when visual_prompt is absent."},
				"visual_prompt":   map[string]any{"type": "string", "description": "Self-contained visual prompt that can execute without conversation history."},
				"title":           map[string]any{"type": "string"},
				"summary":         map[string]any{"type": "string"},
				"sections":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"relationships":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"must_include":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"must_not_invent": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"layout":          map[string]any{"type": "string", "enum": []string{"auto", "mind_map", "flowchart", "timeline", "infographic", "poster", "diagram"}},
				"language":        map[string]any{"type": "string", "enum": []string{"auto", "zh", "zh-cn", "zh-tw", "en", "ja", "ko"}},
				"resolution":      map[string]any{"type": "string", "enum": []string{"1K", "2K", "4K", "1k", "2k", "4k"}},
				"aspect_ratio":    map[string]any{"type": "string", "description": "Optional WIDTH:HEIGHT ratio."},
				"model": map[string]any{"type": "string", "enum": []string{
					CangyuanImageModel1K, CangyuanImageModel2K, CangyuanImageModel4K,
					CangyuanImage25FlareModel1K, CangyuanImage25FlareModel2K, CangyuanImage25FlareModel4K,
					CangyuanImage25SunburstModel1K, CangyuanImage25SunburstModel2K, CangyuanImage25SunburstModel4K,
				}},
				"size":    map[string]any{"type": "string", "description": "Optional WIDTHxHEIGHT output dimensions."},
				"quality": map[string]any{"type": "string", "enum": []string{"auto", "low", "medium", "high", "xhigh", "max"}},
			},
			"required": []string{"prompt"},
		},
		// This planner intentionally has optional enrichment fields. Responses
		// strict mode requires every property to be listed in required (optional
		// values must be nullable), so strict=true with only prompt required is
		// rejected by conforming upstreams before the model runs. The bridge
		// validates and normalizes every returned plan itself.
		"strict": false,
	}
}

func isCodexClientImageGenerationTool(tool map[string]any) bool {
	if tool == nil {
		return false
	}
	toolType := strings.TrimSpace(codexStringValue(tool["type"]))
	name := strings.TrimSpace(codexStringValue(tool["name"]))
	namespace := strings.TrimSpace(codexStringValue(tool["namespace"]))
	if toolType == "image_generation" {
		return true
	}
	if toolType == "namespace" && (isOpenAIImageGenNamespaceName(name) || isOpenAIImageGenNamespaceName(namespace)) {
		return true
	}
	if isOpenAIImageGenFunctionReference(namespace, name) {
		return true
	}
	if function, ok := tool["function"].(map[string]any); ok {
		return isOpenAIImageGenFunctionReference(
			strings.TrimSpace(codexStringValue(function["namespace"])),
			strings.TrimSpace(codexStringValue(function["name"])),
		)
	}
	return false
}

func isCodexClientImageGenerationToolChoice(choice any) bool {
	if openAIAnyToolChoiceSelectsImageGeneration(choice) {
		return true
	}
	switch value := choice.(type) {
	case string:
		value = strings.TrimSpace(value)
		return isOpenAIImageGenNamespaceName(value) || isOpenAIImageGenFunctionReference("", value)
	case map[string]any:
		if isOpenAIImageGenFunctionReference(
			strings.TrimSpace(codexStringValue(value["namespace"])),
			strings.TrimSpace(codexStringValue(value["name"])),
		) {
			return true
		}
		if tool, ok := value["tool"].(map[string]any); ok && isCodexClientImageGenerationToolChoice(tool) {
			return true
		}
		if function, ok := value["function"].(map[string]any); ok {
			return isCodexClientImageGenerationToolChoice(function)
		}
	}
	return false
}

type codexDedicatedImagePlan struct {
	Prompt        string   `json:"prompt"`
	VisualPrompt  string   `json:"visual_prompt"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Sections      []string `json:"sections"`
	Relationships []string `json:"relationships"`
	MustInclude   []string `json:"must_include"`
	MustNotInvent []string `json:"must_not_invent"`
	Layout        string   `json:"layout"`
	Language      string   `json:"language"`
	Resolution    string   `json:"resolution"`
	AspectRatio   string   `json:"aspect_ratio"`
	Model         string   `json:"model"`
	Size          string   `json:"size"`
	Quality       string   `json:"quality"`
	CallID        string   `json:"-"`
	PartialText   []string `json:"-"`
}

func extractCodexDedicatedImagePlan(raw []byte, requestBody []byte) (*codexDedicatedImagePlan, bool, error) {
	stream := codexDedicatedImageRequestStream(requestBody)
	var candidates []map[string]any
	if stream {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		argumentDeltas := make(map[string]*codexDedicatedImageArgumentsAccumulator)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var value map[string]any
			if json.Unmarshal([]byte(payload), &value) == nil {
				candidates = append(candidates, value)
				collectCodexDedicatedImageArgumentDelta(value, argumentDeltas)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, false, err
		}
		for _, accumulator := range argumentDeltas {
			if accumulator == nil || accumulator.name != codexDedicatedImagePlannerToolName || accumulator.arguments.Len() == 0 {
				continue
			}
			candidate := map[string]any{
				"name":      codexDedicatedImagePlannerToolName,
				"arguments": accumulator.arguments.String(),
			}
			if accumulator.callID != "" {
				candidate["call_id"] = accumulator.callID
			}
			candidates = append(candidates, candidate)
		}
	} else {
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false, nil // an opaque non-plan response is replayed.
		}
		candidates = append(candidates, value)
	}
	var selectedArgs []byte
	var selectedCallID string
	var selectedPlan *codexDedicatedImagePlan
	for _, value := range candidates {
		for _, candidate := range findAllCodexDedicatedImageArguments(value) {
			args, callID := candidate.arguments, candidate.callID
			callID = strings.TrimSpace(callID)
			callIDsDiffer := selectedCallID != "" && callID != "" && callID != selectedCallID
			if selectedArgs != nil && (callIDsDiffer || !codexDedicatedImageArgumentsEqual(args, selectedArgs)) {
				return nil, true, codexDedicatedImagePlanError("multiple image tool calls cannot be completed in one response")
			}
			if selectedArgs != nil {
				continue
			}
			selectedArgs = append([]byte(nil), args...)
			selectedCallID = callID
			var plan codexDedicatedImagePlan
			if err := json.Unmarshal(args, &plan); err != nil {
				return nil, true, errors.New("codex image planner returned invalid arguments")
			}
			if err := normalizeAndValidateCodexDedicatedImagePlan(&plan); err != nil {
				return nil, true, err
			}
			plan.CallID = callID
			if plan.CallID == "" {
				plan.CallID = codexDedicatedImagePlannerToolName
			}
			plan.PartialText = extractCodexPlannerPartialText(raw, stream)
			selectedPlan = &plan
		}
	}
	if selectedPlan != nil {
		return selectedPlan, true, ValidateCangyuanImageRequest(CangyuanImageOperationGeneration, CangyuanImageRequest{Model: selectedPlan.Model, Prompt: selectedPlan.Prompt, Size: codexDedicatedImagePlanSize(selectedPlan), Quality: selectedPlan.Quality, N: 1, ResponseFormat: "b64_json", Async: true})
	}
	return nil, false, nil
}

func codexDedicatedImageArgumentsEqual(left, right []byte) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil {
		leftCanonical, leftErr := json.Marshal(leftValue)
		rightCanonical, rightErr := json.Marshal(rightValue)
		if leftErr == nil && rightErr == nil {
			return bytes.Equal(leftCanonical, rightCanonical)
		}
	}
	return bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right))
}

const (
	codexDedicatedImagePlanMaxPromptBytes = 16 << 10
	codexDedicatedImagePlanMaxFieldBytes  = 4 << 10
	codexDedicatedImagePlanMaxItems       = 64
)

func normalizeAndValidateCodexDedicatedImagePlan(plan *codexDedicatedImagePlan) error {
	if plan == nil {
		return codexDedicatedImagePlanError("plan is missing")
	}
	plan.Prompt = strings.TrimSpace(plan.Prompt)
	plan.VisualPrompt = strings.TrimSpace(plan.VisualPrompt)
	plan.Title = strings.TrimSpace(plan.Title)
	plan.Summary = strings.TrimSpace(plan.Summary)
	plan.Layout = strings.ToLower(strings.TrimSpace(plan.Layout))
	plan.Language = strings.TrimSpace(plan.Language)
	plan.Resolution = strings.ToUpper(strings.TrimSpace(plan.Resolution))
	plan.AspectRatio = strings.TrimSpace(plan.AspectRatio)
	plan.Size = strings.TrimSpace(plan.Size)
	// Codex planners often use the public 1K/2K/4K labels in `size`, even
	// though Cangyuan's documented request uses `size` for WIDTHxHEIGHT or an
	// allowed ratio. Also, Cangyuan rejects a request
	// that carries both an explicit size and aspect_ratio. Normalize these
	// equivalent planner representations before validating the provider request.
	switch strings.ToUpper(plan.Size) {
	case "1K", "2K", "4K":
		plan.Size = ""
	}
	if plan.AspectRatio != "" {
		plan.Size = ""
	}
	plan.Quality = normalizeCodexDedicatedImageQuality(plan.Quality)
	rawModel := strings.TrimSpace(plan.Model)
	plan.Model = normalizeDedicatedImageModel(rawModel)
	if rawModel != "" && plan.Model == "" {
		return codexDedicatedImagePlanError("model is not an allowed Cangyuan tier")
	}

	if plan.Model == "" {
		switch plan.Resolution {
		case "1K":
			plan.Model = CangyuanImageModel1K
		case "2K":
			plan.Model = CangyuanImageModel2K
		case "4K":
			plan.Model = CangyuanImageModel4K
		default:
			plan.Model = CangyuanImageModel1K
		}
	}
	if plan.Resolution == "" {
		plan.Resolution = dedicatedImageTierForModel(plan.Model)
	}
	if plan.Resolution != dedicatedImageTierForModel(plan.Model) {
		return codexDedicatedImagePlanError("resolution conflicts with model")
	}
	if plan.Prompt == "" && plan.VisualPrompt == "" {
		return codexDedicatedImagePlanError("prompt or visual_prompt is required")
	}
	if len(plan.Prompt) > codexDedicatedImagePlanMaxPromptBytes || len(plan.VisualPrompt) > codexDedicatedImagePlanMaxPromptBytes {
		return codexDedicatedImagePlanError("prompt is too long")
	}
	if len(plan.Title) > codexDedicatedImagePlanMaxFieldBytes || len(plan.Summary) > codexDedicatedImagePlanMaxFieldBytes {
		return codexDedicatedImagePlanError("plan field is too long")
	}
	if err := validateCodexDedicatedImagePlanItems(plan.Sections); err != nil {
		return err
	}
	if err := validateCodexDedicatedImagePlanItems(plan.Relationships); err != nil {
		return err
	}
	if err := validateCodexDedicatedImagePlanItems(plan.MustInclude); err != nil {
		return err
	}
	if err := validateCodexDedicatedImagePlanItems(plan.MustNotInvent); err != nil {
		return err
	}
	if !validCodexDedicatedImageLayout(plan.Layout) {
		return codexDedicatedImagePlanError("layout is not supported")
	}
	if !validCodexDedicatedImageLanguage(plan.Language) {
		return codexDedicatedImagePlanError("language is not supported")
	}
	if plan.AspectRatio != "" {
		capabilities, ok := cangyuanImageCapabilities(plan.Model)
		if !ok || validateCangyuanAspectRatio(plan.AspectRatio, capabilities) != nil {
			return codexDedicatedImagePlanError("aspect_ratio is invalid")
		}
	}
	effectiveVisualPrompt := plan.VisualPrompt
	if effectiveVisualPrompt == "" {
		effectiveVisualPrompt = plan.Prompt
	}
	if codexDedicatedImagePlanHasUnexpandedReference(effectiveVisualPrompt) {
		return codexDedicatedImagePlanError("visual prompt contains an unexpanded conversation reference")
	}
	plan.Prompt = codexDedicatedImagePlanPrompt(plan)
	if len(plan.Prompt) == 0 || len(plan.Prompt) > codexDedicatedImagePlanMaxPromptBytes {
		return codexDedicatedImagePlanError("self-contained visual prompt is invalid")
	}
	// Structured fields are appended after the primary visual_prompt check.
	// Recheck the assembled prompt so a planner cannot hide an unresolved
	// conversation reference in summary, sections, or relationships.
	if codexDedicatedImagePlanHasUnexpandedReference(plan.Prompt) {
		return codexDedicatedImagePlanError("self-contained visual prompt contains an unexpanded conversation reference")
	}
	return nil
}

func normalizeCodexDedicatedImageQuality(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "standard", "default", "normal":
		// OpenAI-compatible planners commonly use the legacy Images API
		// spelling "standard" even when a provider exposes the newer
		// low/medium/high/auto vocabulary. Preserve the user's lack of an
		// explicit quality preference by delegating the choice to Cangyuan.
		return "auto"
	case "hd":
		return "high"
	default:
		return value
	}
}

func validateCodexDedicatedImagePlanItems(items []string) error {
	if len(items) > codexDedicatedImagePlanMaxItems {
		return codexDedicatedImagePlanError("plan contains too many items")
	}
	for _, item := range items {
		if len(strings.TrimSpace(item)) == 0 || len(item) > codexDedicatedImagePlanMaxFieldBytes {
			return codexDedicatedImagePlanError("plan item is invalid")
		}
	}
	return nil
}

func validCodexDedicatedImageLayout(layout string) bool {
	switch layout {
	case "", "auto", "mind_map", "flowchart", "timeline", "infographic", "poster", "diagram":
		return true
	default:
		return false
	}
}

func validCodexDedicatedImageLanguage(language string) bool {
	switch strings.ToLower(language) {
	case "", "auto", "zh", "zh-cn", "zh-tw", "en", "ja", "ko":
		return true
	default:
		return false
	}
}

func codexDedicatedImagePlanHasUnexpandedReference(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, pattern := range codexDedicatedImageUnexpandedReferencePatterns {
		matches := pattern.FindAllStringIndex(value, -1)
		for _, match := range matches {
			if !codexDedicatedImageReferenceHasConcreteContext(value, match[0], match[1]) {
				return true
			}
		}
	}
	return false
}

var codexDedicatedImageUnexpandedReferencePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:按|根据|依照)?(?:我们)?(?:之前|上述|刚才|前面|上文)(?:讨论|提到|说过|提供)?(?:过的)?(?:内容|信息|要点|观点|细节|资料|需求)?`),
	regexp.MustCompile(`(?:based\s+on\s+)?(?:the\s+)?(?:previous|earlier|above)(?:ly)?(?:\s+(?:discussion|conversation|content|points?|information|messages?|details?|requirements?))?`),
	regexp.MustCompile(`as\s+discussed(?:\s+(?:above|earlier|previously))?`),
}

func codexDedicatedImageReferenceHasConcreteContext(value string, start, end int) bool {
	nearby := value[:start] + " " + value[end:]
	nearby = codexDedicatedImageGenericReferenceWords.ReplaceAllString(nearby, " ")
	concrete := 0
	for _, token := range codexDedicatedImageConcreteTokenPattern.FindAllString(nearby, -1) {
		if len([]rune(token)) >= 2 {
			concrete += len([]rune(token))
		}
	}
	// A short subject name alone does not expand a conversational reference.
	// Requiring several concrete terms still accepts compact technical prompts
	// such as "TCP 三次握手: SYN, SYN-ACK, ACK".
	return concrete >= 18
}

var codexDedicatedImageGenericReferenceWords = regexp.MustCompile(`(?i)\b(?:create|draw|generate|make|image|diagram|visual|visualize|include|show|summarize|summary|mind|map|using|with|from|about|please|the|a|an|of|and|or|all|those|these|them|it|this|that|our|we|discussed|mentioned|provided|content|points?|information|details?|requirements?)\b|(?:生成|创建|绘制|制作|图片|图像|图表|示意图|思维导图|可视化|展示|包含|总结|所有|这些|那些|这个|那个|内容|信息|要点|观点|细节|资料|需求|讨论|提到|说过|提供|请|按照|根据|基于|关于|以及|并且|用于)`)
var codexDedicatedImageConcreteTokenPattern = regexp.MustCompile(`[a-z0-9][a-z0-9_+./:-]*|[\p{Han}]+`)

func codexDedicatedImagePlanError(message string) error {
	return &CangyuanAdapterError{Code: "image_plan_invalid", HTTPStatus: http.StatusBadRequest, Err: errors.New(message)}
}

func codexDedicatedImagePlanPrompt(plan *codexDedicatedImagePlan) string {
	if plan == nil {
		return ""
	}
	parts := make([]string, 0, 10)
	base := strings.TrimSpace(plan.VisualPrompt)
	if base == "" {
		base = strings.TrimSpace(plan.Prompt)
	}
	if base != "" {
		parts = append(parts, base)
	}
	if plan.Title != "" {
		parts = append(parts, "Title: "+plan.Title)
	}
	if plan.Summary != "" {
		parts = append(parts, "Summary: "+plan.Summary)
	}
	appendItems := func(label string, values []string) {
		if len(values) == 0 {
			return
		}
		parts = append(parts, label+": "+strings.Join(values, "; "))
	}
	appendItems("Sections", plan.Sections)
	appendItems("Relationships", plan.Relationships)
	appendItems("Must include", plan.MustInclude)
	appendItems("Do not invent", plan.MustNotInvent)
	if plan.Layout != "" {
		parts = append(parts, "Layout: "+plan.Layout)
	}
	if plan.Language != "" {
		parts = append(parts, "Language: "+plan.Language)
	}
	if plan.AspectRatio != "" {
		parts = append(parts, "Aspect ratio: "+plan.AspectRatio)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// extractCodexPlannerPartialText keeps user-visible planner text that was
// emitted before the private image tool call. A planner is allowed to say
// something short such as "I will turn those points into a mind map" before
// invoking the tool; dropping that text makes a streamed Codex response look
// truncated. Only text emitted by the planner is collected here. It is never
// copied into the Cangyuan prompt (the prompt is still built exclusively from
// the validated, self-contained image plan).
func extractCodexPlannerPartialText(raw []byte, stream bool) []string {
	if !stream {
		var value map[string]any
		if json.Unmarshal(raw, &value) != nil {
			return nil
		}
		return responseMessageText(value)
	}

	var completed []string
	var deltas []string
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 8<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var value map[string]any
		if json.Unmarshal([]byte(payload), &value) != nil {
			continue
		}
		if text := responseMessageText(value); len(text) > 0 {
			completed = text
		}
		if strings.TrimSpace(codexStringValue(value["type"])) == "response.output_text.delta" {
			if delta := strings.TrimSpace(codexStringValue(value["delta"])); delta != "" {
				deltas = append(deltas, delta)
			}
		}
	}
	if len(completed) > 0 {
		return completed
	}
	if len(deltas) == 0 {
		return nil
	}
	return []string{strings.TrimSpace(strings.Join(deltas, ""))}
}

func responseMessageText(value map[string]any) []string {
	if value == nil {
		return nil
	}
	if response, ok := value["response"].(map[string]any); ok {
		if text := responseMessageText(response); len(text) > 0 {
			return text
		}
	}
	if item, ok := value["item"].(map[string]any); ok {
		if text := responseMessageText(item); len(text) > 0 {
			return text
		}
	}
	if output, ok := value["output"].([]any); ok {
		result := make([]string, 0)
		for _, rawItem := range output {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, responseMessageText(item)...)
		}
		return result
	}
	if itemType := strings.TrimSpace(codexStringValue(value["type"])); itemType != "message" && itemType != "output_item" {
		return nil
	}
	content, ok := value["content"].([]any)
	if !ok {
		if text := strings.TrimSpace(codexStringValue(value["text"])); text != "" {
			return []string{text}
		}
		return nil
	}
	var result []string
	for _, rawPart := range content {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		partType := strings.TrimSpace(codexStringValue(part["type"]))
		if partType != "output_text" && partType != "text" {
			continue
		}
		text := strings.TrimSpace(codexStringValue(part["text"]))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

type codexDedicatedImageArgumentsAccumulator struct {
	name      string
	callID    string
	arguments strings.Builder
}

type codexDedicatedImageArgumentCandidate struct {
	arguments []byte
	callID    string
}

func collectCodexDedicatedImageArgumentDelta(value map[string]any, accumulators map[string]*codexDedicatedImageArgumentsAccumulator) {
	if value == nil {
		return
	}
	eventType := strings.TrimSpace(codexStringValue(value["type"]))
	if eventType == "response.output_item.added" || eventType == "response.output_item.done" {
		item, ok := value["item"].(map[string]any)
		if !ok || strings.TrimSpace(codexStringValue(item["type"])) != "function_call" {
			return
		}
		key := strings.TrimSpace(codexStringValue(item["id"]))
		if key == "" {
			key = strings.TrimSpace(codexStringValue(item["call_id"]))
		}
		if key == "" {
			key = strings.TrimSpace(codexStringValue(value["output_index"]))
		}
		if key == "" {
			return
		}
		accumulator := accumulators[key]
		if accumulator == nil {
			accumulator = &codexDedicatedImageArgumentsAccumulator{}
			accumulators[key] = accumulator
		}
		if accumulator.name == "" {
			accumulator.name = strings.TrimSpace(codexStringValue(item["name"]))
		}
		if accumulator.callID == "" {
			accumulator.callID = strings.TrimSpace(codexStringValue(item["call_id"]))
			if accumulator.callID == "" {
				accumulator.callID = strings.TrimSpace(codexStringValue(item["id"]))
			}
		}
		return
	}
	if eventType != "response.function_call_arguments.delta" && eventType != "response.function_call_arguments.done" {
		return
	}
	key := strings.TrimSpace(codexStringValue(value["item_id"]))
	if key == "" {
		key = strings.TrimSpace(codexStringValue(value["call_id"]))
	}
	if key == "" {
		key = strings.TrimSpace(codexStringValue(value["output_index"]))
	}
	if key == "" {
		return
	}
	accumulator := accumulators[key]
	if accumulator == nil {
		accumulator = &codexDedicatedImageArgumentsAccumulator{}
		accumulators[key] = accumulator
	}
	if accumulator.name == "" {
		accumulator.name = strings.TrimSpace(codexStringValue(value["name"]))
	}
	if accumulator.name == "" {
		if item, ok := value["item"].(map[string]any); ok {
			accumulator.name = strings.TrimSpace(codexStringValue(item["name"]))
		}
	}
	if accumulator.callID == "" {
		accumulator.callID = strings.TrimSpace(codexStringValue(value["call_id"]))
		if accumulator.callID == "" {
			accumulator.callID = strings.TrimSpace(codexStringValue(value["item_id"]))
		}
	}
	if eventType == "response.function_call_arguments.done" {
		if arguments := codexStringValue(value["arguments"]); arguments != "" {
			accumulator.arguments.Reset()
			_, _ = accumulator.arguments.WriteString(arguments)
		}
		return
	}
	if delta := codexStringValue(value["delta"]); delta != "" {
		_, _ = accumulator.arguments.WriteString(delta)
	}
}

func findAllCodexDedicatedImageArguments(value map[string]any) []codexDedicatedImageArgumentCandidate {
	if value == nil {
		return nil
	}
	result := make([]codexDedicatedImageArgumentCandidate, 0)
	if strings.TrimSpace(codexStringValue(value["name"])) == codexDedicatedImagePlannerToolName {
		callID := strings.TrimSpace(codexStringValue(value["call_id"]))
		if callID == "" {
			callID = strings.TrimSpace(codexStringValue(value["id"]))
		}
		var arguments []byte
		switch args := value["arguments"].(type) {
		case string:
			arguments = []byte(args)
		case map[string]any:
			arguments, _ = json.Marshal(args)
		}
		if len(arguments) > 0 {
			result = append(result, codexDedicatedImageArgumentCandidate{arguments: arguments, callID: callID})
		}
	}
	if item, ok := value["item"].(map[string]any); ok {
		result = append(result, findAllCodexDedicatedImageArguments(item)...)
	}
	if output, ok := value["output"].([]any); ok {
		for _, rawItem := range output {
			if item, ok := rawItem.(map[string]any); ok {
				result = append(result, findAllCodexDedicatedImageArguments(item)...)
			}
		}
	}
	return result
}
