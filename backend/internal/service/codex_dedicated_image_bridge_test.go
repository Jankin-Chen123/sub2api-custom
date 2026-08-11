package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildCodexDedicatedImagePlannerBody_ReplacesClientImageToolWithPrivatePlanner(t *testing.T) {
	body := []byte(`{"model":"gpt-5","stream":false,"tools":[{"type":"image_generation"},{"type":"namespace","name":"image_gen"},{"type":"function","name":"image_gen__imagegen"},{"type":"function","name":"exec_command","parameters":{"type":"object"}}],"tool_choice":{"type":"image_generation"}}`)

	got, err := buildCodexDedicatedImagePlannerBody(body)
	require.NoError(t, err)
	require.NotContains(t, string(got), `"type":"image_generation"`)
	require.NotContains(t, string(got), `"name":"image_gen"`)
	require.NotContains(t, string(got), `"name":"image_gen__imagegen"`)
	require.Contains(t, string(got), codexDedicatedImagePlannerToolName)
	require.Equal(t, int64(2), gjson.GetBytes(got, "tools.#").Int())
	require.Contains(t, string(got), `"tool_choice":"auto"`)
	require.Contains(t, string(got), "self-contained prompt")
}

func TestBuildCodexDedicatedImagePlannerBody_ReplacesRemovedImageToolChoice(t *testing.T) {
	choices := map[string]any{
		"native image generation": map[string]any{"type": "image_generation"},
		"native image string":     "image_generation",
		"nested native tool":      map[string]any{"tool": map[string]any{"type": "image_generation"}},
	}

	for name, choice := range choices {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"model":       "gpt-5",
				"input":       "draw a diagram",
				"tools":       []any{map[string]any{"type": "image_generation"}},
				"tool_choice": choice,
			})
			require.NoError(t, err)

			got, err := buildCodexDedicatedImagePlannerBody(body)
			require.NoError(t, err)
			require.Equal(t, "auto", gjson.GetBytes(got, "tool_choice").String())
		})
	}
}

func TestBuildCodexDedicatedImagePlannerBody_ReplacesClientImageToolChoice(t *testing.T) {
	body := []byte(`{"model":"gpt-5","tools":[{"type":"namespace","name":"image_gen"}],"tool_choice":{"type":"namespace","name":"image_gen"}}`)

	got, err := buildCodexDedicatedImagePlannerBody(body)
	require.NoError(t, err)
	require.Equal(t, "auto", gjson.GetBytes(got, "tool_choice").String())
	require.NotContains(t, string(got), `"name":"image_gen"`)
	require.Contains(t, string(got), codexDedicatedImagePlannerToolName)
}

func TestCodexDedicatedImagePlannerToolUsesCompatibleNonStrictSchema(t *testing.T) {
	raw, err := json.Marshal(codexDedicatedImagePlannerTool())
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(raw, "strict").Bool())
	require.Equal(t, int64(1), gjson.GetBytes(raw, "parameters.required.#").Int())
	require.Equal(t, "prompt", gjson.GetBytes(raw, "parameters.required.0").String())
	require.Equal(t, int64(4), gjson.GetBytes(raw, "parameters.properties.quality.enum.#").Int())
}

func TestNormalizeCodexDedicatedImagePlanAcceptsOpenAIQualityAliases(t *testing.T) {
	tests := map[string]string{
		"standard": "auto",
		"default":  "auto",
		"normal":   "auto",
		"HD":       "high",
	}
	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			plan := &codexDedicatedImagePlan{
				Prompt:     "绘制中文 TCP 三次握手知识笔记",
				Resolution: "1K",
				Quality:    input,
			}
			require.NoError(t, normalizeAndValidateCodexDedicatedImagePlan(plan))
			require.Equal(t, expected, plan.Quality)
		})
	}
}

func TestBuildCodexDedicatedImagePlannerBody_PreservesLongConversationContext(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5",
		"stream":false,
		"instructions":"Answer in Chinese.",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"知识点一：光合作用发生在叶绿体。"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"已记录知识点一。"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"请把之前所有知识点整理成思维导图。"}]}
		],
		"tools":[{"type":"image_generation"},{"type":"function","name":"exec_command"}],
		"tool_choice":{"type":"function","function":{"name":"image_generation"}}
	}`)

	got, err := buildCodexDedicatedImagePlannerBody(body)
	require.NoError(t, err)
	require.Equal(t, int64(3), gjson.GetBytes(got, "input.#").Int())
	require.Contains(t, string(got), "光合作用发生在叶绿体")
	require.Contains(t, string(got), "已记录知识点一")
	require.Contains(t, string(got), "思维导图")
	require.Equal(t, "auto", gjson.GetBytes(got, "tool_choice").String())
	require.Contains(t, string(got), "Answer in Chinese.")
}

func TestCodexDedicatedImageLongContextPlanIsSelfContainedAndDropsUnrelatedCanary(t *testing.T) {
	const privateCanary = "unrelated-private-context-canary"
	input := make([]map[string]any, 0, 20)
	for index := 0; index < 20; index++ {
		text := "ordinary discussion turn"
		if index == 3 {
			text = "知识点一：光合作用发生在叶绿体。"
		}
		if index == 11 {
			text = "知识点二：生态系统包含生产者、消费者和分解者。"
		}
		if index == 17 {
			text = "私人旁支信息：" + privateCanary
		}
		input = append(input, map[string]any{
			"type": "message", "role": "user",
			"content": []any{map[string]any{"type": "input_text", "text": text}},
		})
	}
	body, err := json.Marshal(map[string]any{
		"model": "gpt-5", "stream": false, "input": input,
		"tools": []any{map[string]any{"type": "image_generation"}},
	})
	require.NoError(t, err)
	plannerBody, err := buildCodexDedicatedImagePlannerBody(body)
	require.NoError(t, err)
	require.Equal(t, int64(20), gjson.GetBytes(plannerBody, "input.#").Int())
	require.Contains(t, string(plannerBody), "光合作用发生在叶绿体")
	require.Contains(t, string(plannerBody), "生态系统包含生产者")

	planResponse := []byte(`{"id":"resp_planner_long","output":[{"type":"function_call","call_id":"call_long","name":"sub2api_generate_image","arguments":"{\"visual_prompt\":\"中文知识思维导图：光合作用与生态系统的关系\",\"summary\":\"整理两个已讨论知识点\",\"must_include\":[\"叶绿体\",\"生产者\"],\"must_not_invent\":[\"未讨论的事实\"],\"layout\":\"mind_map\",\"language\":\"zh-cn\",\"resolution\":\"2K\",\"model\":\"gpt-image-2-2k\",\"size\":\"2048x2048\"}"} ]}`)
	plan, found, err := extractCodexDedicatedImagePlan(planResponse, plannerBody)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, normalizeAndValidateCodexDedicatedImagePlan(plan))
	finalPrompt := codexDedicatedImagePlanPrompt(plan)
	require.Contains(t, finalPrompt, "光合作用")
	require.Contains(t, finalPrompt, "生态系统")
	require.NotContains(t, finalPrompt, privateCanary)
}

func TestPrepareCodexDedicatedImagePlannerHTTPBodyKeepsPreviousResponseID(t *testing.T) {
	body, err := prepareCodexDedicatedImagePlannerHTTPBody([]byte(`{
		"type":"response.create",
		"generate":true,
		"model":"gpt-5",
		"previous_response_id":"resp_planner_1",
		"input":"continue",
		"stream":false
	}`))
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(body, "type").Exists())
	require.False(t, gjson.GetBytes(body, "generate").Exists())
	require.Equal(t, "resp_planner_1", gjson.GetBytes(body, "previous_response_id").String())
	require.True(t, gjson.GetBytes(body, "stream").Bool())
}

func TestExtractCodexDedicatedImagePlan_NonStreaming(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":false}`)
	response := []byte(`{"id":"resp_1","output":[{"type":"function_call","name":"sub2api_generate_image","arguments":"{\"prompt\":\"a dog\",\"model\":\"2K\",\"size\":\"2048x2048\"}"}]}`)

	plan, found, err := extractCodexDedicatedImagePlan(response, request)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, CangyuanImageModel2K, plan.Model)
	require.Equal(t, "2048x2048", plan.Size)
	require.Equal(t, "a dog", plan.Prompt)
}

func TestExtractCodexDedicatedImagePlan_StreamingOutputItem(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":true}`)
	response := []byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"name\":\"sub2api_generate_image\",\"arguments\":\"{\\\"prompt\\\":\\\"a map\\\",\\\"model\\\":\\\"gpt-image-2-4k\\\"}\"}}\n\n")

	plan, found, err := extractCodexDedicatedImagePlan(response, request)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, CangyuanImageModel4K, plan.Model)
	require.Equal(t, "a map", plan.Prompt)
}

func TestExtractCodexDedicatedImagePlan_StreamingArgumentDeltas(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":true}`)
	response := strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","call_id":"call_1","type":"function_call","name":"sub2api_generate_image"}}`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","call_id":"call_1","delta":"{\"prompt\":\"knowledge map\",\"model\":\"gpt-image-2-2k\"}"}`,
		`data: [DONE]`,
	}, "\n\n")

	plan, found, err := extractCodexDedicatedImagePlan([]byte(response), request)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, CangyuanImageModel2K, plan.Model)
	require.Equal(t, "knowledge map", plan.Prompt)
	require.Equal(t, "call_1", plan.CallID)
}

func TestExtractCodexDedicatedImagePlanStructuredKnowledgeMap(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":false}`)
	response := []byte(`{"output":[{"type":"function_call","name":"sub2api_generate_image","arguments":"{\"prompt\":\"请把之前讨论的内容整理成思维导图\",\"visual_prompt\":\"以中心主题为核心，绘制光合作用知识思维导图\",\"title\":\"光合作用\",\"summary\":\"光合作用发生在叶绿体，产物包括有机物和氧气。\",\"sections\":[\"场所：叶绿体\",\"产物：有机物和氧气\"],\"relationships\":[\"叶绿体→发生场所\",\"光能→驱动反应\"],\"must_include\":[\"叶绿体\",\"光能\"],\"must_not_invent\":[\"不得添加未讨论的实验数据\"],\"layout\":\"mind_map\",\"language\":\"zh-CN\",\"resolution\":\"2K\",\"aspect_ratio\":\"16:9\",\"model\":\"gpt-image-2-2k\"}"}]}`)

	plan, found, err := extractCodexDedicatedImagePlan(response, request)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, CangyuanImageModel2K, plan.Model)
	require.Equal(t, "2K", plan.Resolution)
	require.Equal(t, "16:9", plan.AspectRatio)
	require.Equal(t, "mind_map", plan.Layout)
	require.Contains(t, plan.Prompt, "叶绿体")
	require.Contains(t, plan.Prompt, "不得添加未讨论的实验数据")
}

func TestExtractCodexDedicatedImagePlanRejectsUnexpandedReference(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":false}`)
	response := []byte(`{"output":[{"type":"function_call","name":"sub2api_generate_image","arguments":"{\"prompt\":\"按之前讨论的内容生成图片\"}"}]}`)

	_, found, err := extractCodexDedicatedImagePlan(response, request)
	require.True(t, found)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_plan_invalid", adapterErr.Code)
}

func TestExtractCodexDedicatedImagePlanRejectsReferenceInStructuredField(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":false}`)
	response := []byte(`{"output":[{"type":"function_call","name":"sub2api_generate_image","arguments":"{\"visual_prompt\":\"draw a knowledge map\",\"summary\":\"include the points discussed earlier\"}"}]}`)

	_, found, err := extractCodexDedicatedImagePlan(response, request)
	require.True(t, found)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_plan_invalid", adapterErr.Code)
	require.Contains(t, err.Error(), "unexpanded conversation reference")
}

func TestExtractCodexDedicatedImagePlanRejectsResolutionConflict(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":false}`)
	response := []byte(`{"output":[{"type":"function_call","name":"sub2api_generate_image","arguments":"{\"prompt\":\"draw a dog\",\"model\":\"gpt-image-2-2k\",\"resolution\":\"4K\"}"}]}`)

	_, found, err := extractCodexDedicatedImagePlan(response, request)
	require.True(t, found)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_plan_invalid", adapterErr.Code)
}

func TestExtractCodexDedicatedImagePlan_NormalTextIsReplayed(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":false}`)
	response := []byte(`{"id":"resp_text","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"普通回答"}]}]}`)

	plan, found, err := extractCodexDedicatedImagePlan(response, request)
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, plan)

	replay, err := extractCodexPlannerReplayInput(response, request)
	require.NoError(t, err)
	require.Len(t, replay, 1)
	require.Equal(t, "message", gjson.GetBytes(replay[0], "type").String())
	require.Equal(t, "普通回答", gjson.GetBytes(replay[0], "content.0.text").String())
}

func TestExtractCodexPlannerReplayInput_StreamingDeduplicatesOutputItems(t *testing.T) {
	request := []byte(`{"model":"gpt-5","stream":true}`)
	response := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"第一轮回答"}]}}`,
		`data: {"type":"response.completed","response":{"output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"第一轮回答"}]},{"id":"rs_1","type":"reasoning","summary":[]}]}}`,
		`data: [DONE]`,
	}, "\n\n")

	replay, err := extractCodexPlannerReplayInput([]byte(response), request)
	require.NoError(t, err)
	require.Len(t, replay, 2)
	require.Equal(t, "msg_1", gjson.GetBytes(replay[0], "id").String())
	require.Equal(t, "rs_1", gjson.GetBytes(replay[1], "id").String())
}

func TestCodexDedicatedImageBridgeForward_ReplaysNormalPlannerText(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.5",
		"stream":false,
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"之前讨论过光合作用和生态系统，请继续回答。"}]}
		],
		"tools":[{"type":"image_generation"}]
	}`)
	upstreamResponse := `{"id":"resp_planner_text","model":"gpt-5.5","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"普通回答"}]}],"usage":{"input_tokens":10,"output_tokens":3}}`
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newOpenAIRejectedFieldTestResponse(200, upstreamResponse),
	}}
	bridge := &CodexDedicatedImageBridge{
		gateway: newOpenAIRejectedFieldTestService(upstream),
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")

	result, err := bridge.Forward(
		context.Background(),
		c,
		newOpenAIRejectedFieldTestAccount(),
		&APIKey{ID: 77, UserID: 88},
		nil,
		body,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_planner_text", result.ResponseID)
	require.JSONEq(t, upstreamResponse, recorder.Body.String())
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, int64(1), gjson.GetBytes(upstream.bodies[0], "tools.#").Int())
	require.Equal(t, codexDedicatedImagePlannerToolName, gjson.GetBytes(upstream.bodies[0], "tools.0.name").String())
	require.Contains(t, string(upstream.bodies[0]), "光合作用")
}

type codexDedicatedImageBridgeJobRepo struct {
	*imageOrchestratorRepo
	completed *ImageGenerationJob
}

func (r *codexDedicatedImageBridgeJobRepo) GetImageGenerationJobForOwner(context.Context, int64, int64, string) (*ImageGenerationJob, error) {
	return r.completed, nil
}

type codexDedicatedImageBridgeResultReader struct{}

func (codexDedicatedImageBridgeResultReader) Open(context.Context, string) (io.ReadCloser, string, int64, error) {
	data := []byte("fake-png-result")
	return io.NopCloser(bytes.NewReader(data)), "image/png", int64(len(data)), nil
}

func newCodexDedicatedImageBridgeImageFixture(upstreamResponse string) (*CodexDedicatedImageBridge, *APIKey, *codexDedicatedImageBridgeJobRepo, *httpUpstreamRecorder) {
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newOpenAIRejectedFieldTestResponse(http.StatusOK, upstreamResponse),
	}}
	jobID := "imgjob_bridge_completed"
	job := &ImageGenerationJob{
		JobID:            jobID,
		Status:           ImageGenerationJobStatusCompleted,
		ResultObjectRefs: []string{"objects/" + jobID + "/0.png"},
		PublicModel:      CangyuanImageModel1K,
		PayloadObjectRef: stringPointer(ImageGenerationPayloadRef(jobID)),
		EstimatedCost:    0.1,
		SettledCost:      0.1,
		Source:           ImageGenerationJobSourceCodex,
		Operation:        ImageGenerationJobOperationGeneration,
	}
	repo := &codexDedicatedImageBridgeJobRepo{
		imageOrchestratorRepo: &imageOrchestratorRepo{existing: job, replayed: true},
		completed:             job,
	}
	groupID := int64(99)
	apiKey := &APIKey{
		ID: 77, UserID: 88, GroupID: &groupID,
		Group: &Group{ID: groupID, Platform: PlatformOpenAI, AllowImageGeneration: true, RateMultiplier: 1},
	}
	bridge := &CodexDedicatedImageBridge{
		gateway:      newOpenAIRejectedFieldTestService(upstream),
		orchestrator: NewImageGenerationOrchestrator(repo, &imageOrchestratorPayloadStore{}, time.Hour),
		repo:         repo,
		results:      codexDedicatedImageBridgeResultReader{},
		billing:      NewBillingService(nil, nil),
		syncTimeout:  time.Second,
		maxReadBytes: 1 << 20,
	}
	return bridge, apiKey, repo, upstream
}

func TestCodexDedicatedImageBridgeForward_CompletesImageAndStoresReplay(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.5",
		"stream":false,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"经过二十轮讨论，知识点包括光合作用、叶绿体、光能和二氧化碳；请保留这些关系。PRIVATE_CONTEXT_CANARY"}]}],
		"tools":[{"type":"image_generation"}]
	}`)
	upstreamResponse := `{"id":"resp_planner_image","model":"gpt-5.5","output":[{"type":"function_call","id":"call_image_1","call_id":"call_image_1","name":"sub2api_generate_image","arguments":"{\"visual_prompt\":\"a friendly dog beside a clean educational knowledge map\",\"summary\":\"photosynthesis connects light energy, chloroplasts, carbon dioxide, and oxygen\",\"sections\":[\"leaf\",\"light energy\",\"carbon dioxide\"],\"relationships\":[\"light energy is captured by chloroplasts\",\"carbon dioxide becomes part of the process\"],\"must_include\":[\"photosynthesis\",\"chloroplast\"],\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\",\"resolution\":\"1K\"}"}],"usage":{"input_tokens":10,"output_tokens":5}}`
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newOpenAIRejectedFieldTestResponse(http.StatusOK, upstreamResponse),
	}}
	jobID := "imgjob_bridge_completed"
	job := &ImageGenerationJob{
		JobID:            jobID,
		Status:           ImageGenerationJobStatusCompleted,
		ResultObjectRefs: []string{"objects/" + jobID + "/0.png"},
		PublicModel:      CangyuanImageModel1K,
		PayloadObjectRef: stringPointer(ImageGenerationPayloadRef(jobID)),
		EstimatedCost:    0.1,
		SettledCost:      0.1,
		Source:           ImageGenerationJobSourceCodex,
		Operation:        ImageGenerationJobOperationGeneration,
	}
	repo := &codexDedicatedImageBridgeJobRepo{
		imageOrchestratorRepo: &imageOrchestratorRepo{existing: job, replayed: true},
		completed:             job,
	}
	payloads := &imageOrchestratorPayloadStore{}
	bridge := &CodexDedicatedImageBridge{
		gateway:      newOpenAIRejectedFieldTestService(upstream),
		orchestrator: NewImageGenerationOrchestrator(repo, payloads, time.Hour),
		repo:         repo,
		results:      codexDedicatedImageBridgeResultReader{},
		billing:      NewBillingService(nil, nil),
		syncTimeout:  time.Second,
		maxReadBytes: 1 << 20,
	}
	groupID := int64(99)
	apiKey := &APIKey{
		ID: 77, UserID: 88, GroupID: &groupID,
		Group: &Group{ID: groupID, Platform: PlatformOpenAI, AllowImageGeneration: true, RateMultiplier: 1},
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")
	result, err := bridge.Forward(context.Background(), c, newOpenAIRejectedFieldTestAccount(), apiKey, nil, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, strings.HasPrefix(result.ResponseID, codexDedicatedImageResponsePrefix))
	require.Equal(t, result.ResponseID, result.RequestID)
	require.Equal(t, 0, result.ImageCount)
	require.Equal(t, "completed", gjson.Get(recorder.Body.String(), "status").String())
	require.Equal(t, "image_generation_call", gjson.Get(recorder.Body.String(), "output.0.type").String())
	require.Equal(t, "ZmFrZS1wbmctcmVzdWx0", gjson.Get(recorder.Body.String(), "output.0.result").String())
	require.Equal(t, int64(10), gjson.Get(recorder.Body.String(), "usage.input_tokens").Int())
	require.Equal(t, int64(5), gjson.Get(recorder.Body.String(), "usage.output_tokens").Int())
	require.Equal(t, int64(15), gjson.Get(recorder.Body.String(), "usage.total_tokens").Int())
	require.Len(t, upstream.bodies, 1)
	require.Len(t, repo.params, 1)
	require.Contains(t, string(upstream.bodies[0]), "PRIVATE_CONTEXT_CANARY", "the general planner receives the long context")
	plan, found, planErr := extractCodexDedicatedImagePlan([]byte(upstreamResponse), body)
	require.NoError(t, planErr)
	require.True(t, found)
	require.Contains(t, plan.Prompt, "chloroplast", "the image plan must carry the relevant knowledge point")
	require.NotContains(t, plan.Prompt, "PRIVATE_CONTEXT_CANARY", "private unrelated context must not be copied into Cangyuan prompt")

	replayedBody, err := bridge.resolveDedicatedImageReplay(context.Background(), []byte(`{"previous_response_id":"`+result.ResponseID+`","input":"continue"}`))
	require.NoError(t, err)
	require.Equal(t, "resp_planner_image", gjson.GetBytes(replayedBody, "previous_response_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(replayedBody, "input.0.type").String())
}

func TestCodexDedicatedImageBridgeForward_ReplayFailureDoesNotWriteResponse(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":"draw a dog","tools":[{"type":"image_generation"}]}`)
	upstreamResponse := `{"id":"resp_planner_image_failure","model":"gpt-5.5","output":[{"type":"function_call","id":"call_image_failure","call_id":"call_image_failure","name":"sub2api_generate_image","arguments":"{\"prompt\":\"draw a dog\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\",\"resolution\":\"1K\"}"}],"usage":{"input_tokens":10,"output_tokens":5}}`
	bridge, apiKey, _, _ := newCodexDedicatedImageBridgeImageFixture(upstreamResponse)
	bridge.replayStore = &codexDedicatedImageReplayStoreStub{values: make(map[string][]byte), setErr: errors.New("replay store unavailable")}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")

	_, err := bridge.Forward(context.Background(), c, newOpenAIRejectedFieldTestAccount(), apiKey, nil, body)

	require.ErrorContains(t, err, "replay store unavailable")
	require.Empty(t, recorder.Body.Bytes(), "the synthetic response must not be written before replay persistence")
}

func TestCodexDedicatedImageBridgeForwardWebSocket_CompletesImageEventsAndReplay(t *testing.T) {
	sseResponse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"id":"call_image_ws","type":"function_call","call_id":"call_image_ws","name":"sub2api_generate_image","arguments":"{\"prompt\":\"draw a dog\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\",\"resolution\":\"1K\"}"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_planner_ws","model":"gpt-5.5","output":[]}}`,
		`data: [DONE]`,
	}, "\n\n")
	bridge, apiKey, _, _ := newCodexDedicatedImageBridgeImageFixture(sseResponse)
	body := []byte(`{"type":"response.create","model":"gpt-5.5","stream":true,"input":"draw a dog","tools":[{"type":"image_generation"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")

	result, events, err := bridge.ForwardWebSocket(context.Background(), c, newOpenAIRejectedFieldTestAccount(), apiKey, nil, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, strings.HasPrefix(result.ResponseID, codexDedicatedImageResponsePrefix))
	require.Len(t, events, 8)
	require.Equal(t, "response.created", gjson.GetBytes(events[0], "type").String())
	require.Equal(t, "response.image_generation_call.in_progress", gjson.GetBytes(events[3], "type").String())
	require.Equal(t, "response.image_generation_call.completed", gjson.GetBytes(events[5], "type").String())
	require.Equal(t, "response.output_item.done", gjson.GetBytes(events[6], "type").String())
	require.Equal(t, "completed", gjson.GetBytes(events[6], "item.status").String())
	require.Equal(t, "response.completed", gjson.GetBytes(events[7], "type").String())
	require.True(t, result.wsReplayInputExists)
}

func TestCodexDedicatedImageBridgeForwardSSE_CompletesImageEvents(t *testing.T) {
	sseResponse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"id":"call_image_sse","type":"function_call","call_id":"call_image_sse","name":"sub2api_generate_image","arguments":"{\"prompt\":\"draw a dog\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\",\"resolution\":\"1K\"}"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_planner_sse","model":"gpt-5.5","output":[]}}`,
		`data: [DONE]`,
	}, "\n\n")
	bridge, apiKey, _, _ := newCodexDedicatedImageBridgeImageFixture(sseResponse)
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":"draw a dog","tools":[{"type":"image_generation"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")

	result, err := bridge.Forward(context.Background(), c, newOpenAIRejectedFieldTestAccount(), apiKey, nil, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, strings.HasPrefix(result.ResponseID, codexDedicatedImageResponsePrefix))
	require.Contains(t, recorder.Body.String(), "event: response.created")
	require.Contains(t, recorder.Body.String(), "event: response.completed")
	require.Contains(t, recorder.Body.String(), `"sequence_number":7`)
}

func TestCodexDedicatedImagePlannerPreservesPartialTextBeforeImageCall(t *testing.T) {
	raw := []byte(`{"id":"resp_planner_partial","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"我会把刚才的要点整理成思维导图。"}]},{"type":"function_call","id":"call_partial","call_id":"call_partial","name":"sub2api_generate_image","arguments":"{\"prompt\":\"一张包含光合作用关键知识点的思维导图\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\",\"resolution\":\"1K\"}"}]}`)
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":"生成图片","tools":[{"type":"image_generation"}]}`)
	plan, found, err := extractCodexDedicatedImagePlan(raw, body)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []string{"我会把刚才的要点整理成思维导图。"}, plan.PartialText)

	response := map[string]any{
		"id": "resp_img_partial", "model": "gpt-5.5",
		"output": []any{
			codexDedicatedImageMessageItem("resp_img_partial", 0, plan.PartialText[0], true),
			map[string]any{"id": "ig_partial", "type": "image_generation_call", "status": "completed"},
		},
	}
	outputItems, ok := response["output"].([]any)
	require.True(t, ok)
	imageCall, ok := outputItems[1].(map[string]any)
	require.True(t, ok)
	events, err := buildCodexDedicatedImageEvents(response, imageCall)
	require.NoError(t, err)
	require.Len(t, events, 10)
	messageRaw, err := json.Marshal(events[3].data)
	require.NoError(t, err)
	imageRaw, err := json.Marshal(events[4].data)
	require.NoError(t, err)
	require.Equal(t, "message", gjson.GetBytes(messageRaw, "item.type").String())
	require.Equal(t, "image_generation_call", gjson.GetBytes(imageRaw, "item.type").String())
}

func TestExtractCodexDedicatedImagePlanRejectsDistinctDuplicateToolCalls(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":"生成两张图","tools":[{"type":"image_generation"}]}`)
	raw := []byte(`{"output":[{"type":"function_call","id":"call_one","call_id":"call_one","name":"sub2api_generate_image","arguments":"{\"prompt\":\"a blue dog\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\"}"},{"type":"function_call","id":"call_two","call_id":"call_two","name":"sub2api_generate_image","arguments":"{\"prompt\":\"a red cat\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\"}"}]}`)
	_, found, err := extractCodexDedicatedImagePlan(raw, body)
	require.True(t, found)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_plan_invalid", adapterErr.Code)
}

func TestExtractCodexDedicatedImagePlanDeduplicatesExactRepeatedToolCall(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":"生成一张图","tools":[{"type":"image_generation"}]}`)
	raw := []byte(`{"output":[{"type":"function_call","id":"call_same","call_id":"call_same","name":"sub2api_generate_image","arguments":"{\"prompt\":\"a blue dog\",\"model\":\"gpt-image-2-1k\",\"size\":\"1024x1024\"}"},{"type":"function_call","id":"call_same","call_id":"call_same","name":"sub2api_generate_image","arguments":"{\"size\":\"1024x1024\",\"model\":\"gpt-image-2-1k\",\"prompt\":\"a blue dog\"}"}]}`)
	plan, found, err := extractCodexDedicatedImagePlan(raw, body)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, plan)
	require.Equal(t, "call_same", plan.CallID)
	require.Equal(t, CangyuanImageModel1K, plan.Model)
}

func TestNormalizeDedicatedImageModel_UnknownIsRejected(t *testing.T) {
	require.Empty(t, normalizeDedicatedImageModel("gpt-image-2"))
}

func TestCodexDedicatedImageReplayReplacesSyntheticPreviousResponse(t *testing.T) {
	bridge := &CodexDedicatedImageBridge{}
	plan := &codexDedicatedImagePlan{Model: CangyuanImageModel2K, CallID: "call_image_1"}
	require.NoError(t, bridge.rememberDedicatedImageReplay(context.Background(), "resp_sub2api_image", "resp_planner_1", plan, nil, 0))

	resolved, err := bridge.resolveDedicatedImageReplay(context.Background(), []byte(`{
		"model":"gpt-5",
		"previous_response_id":"resp_sub2api_image",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]
	}`))
	require.NoError(t, err)

	require.Equal(t, "resp_planner_1", gjson.GetBytes(resolved, "previous_response_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(resolved, "input.0.type").String())
	require.Equal(t, "call_image_1", gjson.GetBytes(resolved, "input.0.call_id").String())
	require.Equal(t, "continue", gjson.GetBytes(resolved, "input.1.content.0.text").String())
}

func TestCodexDedicatedImageReplayLoadsFromCrossInstanceStore(t *testing.T) {
	store := &codexDedicatedImageReplayStoreStub{values: make(map[string][]byte)}
	first := &CodexDedicatedImageBridge{replayStore: store}
	plan := &codexDedicatedImagePlan{Model: CangyuanImageModel2K, CallID: "call_cross_instance"}
	require.NoError(t, first.rememberDedicatedImageReplay(context.Background(), "resp_sub2api_cross", "resp_planner_cross", plan, nil, 0))
	require.Equal(t, 7*24*time.Hour, store.ttl)

	second := &CodexDedicatedImageBridge{replayStore: store}
	resolved, err := second.resolveDedicatedImageReplay(context.Background(), []byte(`{
		"model":"gpt-5",
		"previous_response_id":"resp_sub2api_cross",
		"input":"continue"
	}`))
	require.NoError(t, err)

	require.Equal(t, "resp_planner_cross", gjson.GetBytes(resolved, "previous_response_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(resolved, "input.0.type").String())
}

func TestCodexDedicatedImageReplayBindsSyntheticResponseToPlannerAccount(t *testing.T) {
	groupID := int64(7)
	gateway := &OpenAIGatewayService{}
	bridge := &CodexDedicatedImageBridge{gateway: gateway, replays: make(map[string]codexDedicatedImageReplay)}
	plan := &codexDedicatedImagePlan{CallID: "call_bind", Prompt: "a dog"}

	require.NoError(t, bridge.rememberDedicatedImageReplay(context.Background(), "resp_img_bind", "resp_planner_bind", plan, &groupID, 42))
	accountID, err := gateway.getOpenAIWSStateStore().GetResponseAccount(context.Background(), groupID, "resp_img_bind")
	require.NoError(t, err)
	require.Equal(t, int64(42), accountID)
}

func TestCodexDedicatedImageReplayMissLeavesBodyUnchanged(t *testing.T) {
	bridge := &CodexDedicatedImageBridge{}
	body := []byte(`{"previous_response_id":"resp_unknown","input":"hello"}`)
	resolved, err := bridge.resolveDedicatedImageReplay(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(resolved))
}

func TestHasCodexDedicatedImageReplayReferenceOnlyMatchesSyntheticIDs(t *testing.T) {
	require.True(t, hasCodexDedicatedImageReplayReference([]byte(`{"previous_response_id":"resp_img_123"}`)))
	require.False(t, hasCodexDedicatedImageReplayReference([]byte(`{"previous_response_id":"resp_planner_123"}`)))
	require.False(t, hasCodexDedicatedImageReplayReference([]byte(`{"input":"ordinary text"}`)))
}

func TestCodexDedicatedImageBridgeHandlesOrdinaryFollowUpWithSyntheticPreviousResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := &ImageGenerationWorkerRuntime{cancel: func() {}}
	bridge := NewCodexDedicatedImageBridge(
		&OpenAIGatewayService{}, nil, nil, nil, nil, runtime,
		&config.Config{DedicatedImage: config.DedicatedImageConfig{
			Enabled: true, WorkerEnabled: true, CodexBridgeEnabled: true,
		}, Gateway: config.GatewayConfig{ForceCodexCLI: true}},
	)
	apiKey := &APIKey{Group: &Group{Platform: PlatformOpenAI, AllowImageGeneration: true}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","previous_response_id":"resp_img_123","input":"continue the discussion"}`))

	require.True(t, bridge.ShouldHandle(c, []byte(`{"model":"gpt-5","previous_response_id":"resp_img_123","input":"continue the discussion"}`), apiKey))
	require.True(t, bridge.AllowsHTTPContinuation(c, []byte(`{"model":"gpt-5","previous_response_id":"resp_img_123","input":"continue the discussion"}`), apiKey))
	require.True(t, bridge.AllowsHTTPContinuation(c, []byte(`{"model":"gpt-5","previous_response_id":"resp_planner_123","tools":[{"type":"image_generation"}],"input":"generate an image"}`), apiKey))
	ordinary := []byte(`{"model":"gpt-5","input":"ordinary text"}`)
	require.True(t, bridge.ShouldHandle(c, ordinary, apiKey), "the planner bridge must be selected before the client declares image_generation")
	require.True(t, bridge.AllowsHTTPContinuation(c, []byte(`{"model":"gpt-5","previous_response_id":"resp_upstream_123","input":"continue"}`), apiKey))
}

func TestCodexDedicatedImageBridgeDoesNotRequireNativeImageToolDeclaration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := &ImageGenerationWorkerRuntime{cancel: func() {}}
	bridge := NewCodexDedicatedImageBridge(
		&OpenAIGatewayService{}, nil, nil, nil, nil, runtime,
		&config.Config{DedicatedImage: config.DedicatedImageConfig{
			Enabled: true, WorkerEnabled: true, CodexBridgeEnabled: true,
		}, Gateway: config.GatewayConfig{ForceCodexCLI: false}},
	)
	apiKey := &APIKey{Group: &Group{Platform: PlatformOpenAI, AllowImageGeneration: true}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","stream":true,"input":"请继续刚才的讨论"}`))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0 (linux; x86_64)")

	require.True(t, bridge.ShouldHandle(c, []byte(`{"model":"gpt-5.5","stream":true,"input":"请继续刚才的讨论"}`), apiKey))
}

func TestCodexDedicatedImageGroupIDFallsBackToLoadedGroup(t *testing.T) {
	groupID := int64(19)
	apiKey := &APIKey{Group: &Group{ID: groupID}}
	resolved := codexDedicatedImageGroupID(apiKey)
	require.NotNil(t, resolved)
	require.Equal(t, groupID, *resolved)
}

func TestCodexDedicatedImageGroupIDPrefersAPIKeyColumn(t *testing.T) {
	columnGroupID := int64(19)
	loadedGroupID := int64(91)
	apiKey := &APIKey{GroupID: &columnGroupID, Group: &Group{ID: loadedGroupID}}
	resolved := codexDedicatedImageGroupID(apiKey)
	require.NotNil(t, resolved)
	require.Equal(t, columnGroupID, *resolved)
}

func TestCodexDedicatedImageReplayStoreFailureIsReturned(t *testing.T) {
	bridge := &CodexDedicatedImageBridge{replayStore: &codexDedicatedImageReplayStoreStub{
		getErr: errors.New("redis unavailable"),
	}}
	body := []byte(`{"previous_response_id":"resp_sub2api_image","input":"hello"}`)
	_, err := bridge.resolveDedicatedImageReplay(context.Background(), body)
	require.Error(t, err)
	require.ErrorContains(t, err, "redis unavailable")
}

func TestCodexDedicatedImageReplayCorruptRecordIsDeletedAndReturned(t *testing.T) {
	store := &codexDedicatedImageReplayStoreStub{values: map[string][]byte{
		"resp_sub2api_corrupt": []byte("not-json"),
	}}
	bridge := &CodexDedicatedImageBridge{replayStore: store}
	body := []byte(`{"previous_response_id":"resp_sub2api_corrupt","input":"hello"}`)
	_, err := bridge.resolveDedicatedImageReplay(context.Background(), body)
	require.ErrorIs(t, err, ErrCodexDedicatedImageReplayCorrupt)
	_, exists := store.values["resp_sub2api_corrupt"]
	require.False(t, exists)
}

func TestBuildCodexDedicatedImageFunctionCallOutputUsesCallID(t *testing.T) {
	bridge := &CodexDedicatedImageBridge{}
	raw := bridge.buildDedicatedImageFunctionCallOutput(&codexDedicatedImagePlan{
		Model:  CangyuanImageModel4K,
		CallID: "call_4k",
	})
	require.Equal(t, "function_call_output", gjson.GetBytes(raw, "type").String())
	require.Equal(t, "call_4k", gjson.GetBytes(raw, "call_id").String())
	require.Contains(t, gjson.GetBytes(raw, "output").String(), "4K")
}

func TestBuildCodexDedicatedImageFunctionCallReplayPairMatchesCallID(t *testing.T) {
	bridge := &CodexDedicatedImageBridge{}
	plan := &codexDedicatedImagePlan{Model: CangyuanImageModel2K, Prompt: "a dog", CallID: "call_image_2"}
	contextItem := bridge.buildDedicatedImageFunctionCallContext(plan)
	outputItem := bridge.buildDedicatedImageFunctionCallOutput(plan)
	require.Equal(t, "function_call", gjson.GetBytes(contextItem, "type").String())
	require.Equal(t, "call_image_2", gjson.GetBytes(contextItem, "call_id").String())
	require.Equal(t, "sub2api_generate_image", gjson.GetBytes(contextItem, "name").String())
	require.Equal(t, "call_image_2", gjson.GetBytes(outputItem, "call_id").String())
}

func TestBuildCodexDedicatedImageEventPayloadsIncludeSequenceNumbers(t *testing.T) {
	events, err := buildCodexDedicatedImageEventPayloads(
		map[string]any{"id": "resp_image", "model": "gpt-5"},
		map[string]any{"id": "ig_1", "type": "image_generation_call"},
	)
	require.NoError(t, err)
	require.Len(t, events, 8)
	for index, event := range events {
		require.Equal(t, index, int(gjson.GetBytes(event, "sequence_number").Int()))
	}
}

func TestCodexDedicatedImageIdempotencyKeyPrefersToolCallID(t *testing.T) {
	plan := &codexDedicatedImagePlan{CallID: "call_same_image"}
	first := codexDedicatedImageIdempotencyKey(plan, []byte(`{"stream":false}`))
	second := codexDedicatedImageIdempotencyKey(plan, []byte(`{"stream":true,"input":"replay"}`))
	require.Equal(t, first, second)
	require.True(t, strings.HasPrefix(first, "codex_call_"))
}

func TestCodexDedicatedImageIdempotencyKeySeparatesReusedCallIDWithDifferentPlan(t *testing.T) {
	firstPlan := &codexDedicatedImagePlan{CallID: "call_reused", Prompt: "draw a puppy", Model: CangyuanImageModel2K, Size: "2048x2048"}
	secondPlan := &codexDedicatedImagePlan{CallID: "call_reused", Prompt: "draw a mind map", Model: CangyuanImageModel2K, Size: "2048x2048"}

	first := codexDedicatedImageIdempotencyKey(firstPlan, []byte(`{"input":"first"}`))
	second := codexDedicatedImageIdempotencyKey(secondPlan, []byte(`{"input":"second"}`))

	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(first, "codex_call_"))
	require.True(t, strings.HasPrefix(second, "codex_call_"))
}

func TestCodexDedicatedImageIdempotencyKeyFallsBackToRequestHash(t *testing.T) {
	plan := &codexDedicatedImagePlan{CallID: codexDedicatedImagePlannerToolName}
	first := codexDedicatedImageIdempotencyKey(plan, []byte(`{"input":"one"}`))
	second := codexDedicatedImageIdempotencyKey(plan, []byte(`{"input":"two"}`))
	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(first, "codex_request_"))
}

func TestCodexDedicatedImagePlanPromptDoesNotCopyOriginalConversation(t *testing.T) {
	originalConversation := "用户私密信息：不应发送到图片上游"
	plan := &codexDedicatedImagePlan{
		VisualPrompt: "绘制一个包含光合作用关键步骤的中文思维导图",
		Summary:      "叶绿体、光能、二氧化碳和氧气之间的关系",
		MustInclude:  []string{"叶绿体", "光能"},
	}
	finalPrompt := codexDedicatedImagePlanPrompt(plan)
	require.Contains(t, finalPrompt, "叶绿体")
	require.NotContains(t, finalPrompt, originalConversation)
}

type codexImageWaitRepositoryStub struct {
	ImageGenerationJobRepository
	job   *ImageGenerationJob
	calls chan struct{}
}

func (r *codexImageWaitRepositoryStub) GetImageGenerationJobForOwner(context.Context, int64, int64, string) (*ImageGenerationJob, error) {
	select {
	case r.calls <- struct{}{}:
	default:
	}
	return r.job, nil
}

func TestCodexDedicatedImageWaitCancellationKeepsDurableJob(t *testing.T) {
	job := &ImageGenerationJob{JobID: "imgjob_wait", Status: ImageGenerationJobStatusSubmitted}
	repo := &codexImageWaitRepositoryStub{job: job, calls: make(chan struct{}, 1)}
	bridge := &CodexDedicatedImageBridge{repo: repo, syncTimeout: time.Hour}
	userID, apiKeyID := int64(1), int64(2)
	apiKey := &APIKey{UserID: userID, ID: apiKeyID}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := bridge.waitForCompletion(ctx, apiKey, job)
		done <- err
	}()
	<-repo.calls
	cancel()

	require.ErrorIs(t, <-done, context.Canceled)
	require.Equal(t, ImageGenerationJobStatusSubmitted, job.Status)
}

func TestCodexDedicatedImageWaitMapsDurableFailureToStableAdapterError(t *testing.T) {
	code := "image_upstream_rate_limited"
	message := "provider failure details"
	job := &ImageGenerationJob{JobID: "imgjob_failed", Status: ImageGenerationJobStatusFailed, ErrorCode: &code, ErrorMessage: &message}
	bridge := &CodexDedicatedImageBridge{
		repo:        &codexImageWaitRepositoryStub{job: job, calls: make(chan struct{}, 1)},
		syncTimeout: time.Second,
	}
	_, err := bridge.waitForCompletion(context.Background(), &APIKey{UserID: 1, ID: 2}, job)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_upstream_rate_limited", adapterErr.Code)
	require.Equal(t, http.StatusBadGateway, adapterErr.HTTPStatus)
	require.Contains(t, err.Error(), "provider failure details")
}

type codexDedicatedImageReplayStoreStub struct {
	values map[string][]byte
	getErr error
	setErr error
	ttl    time.Duration
}

func (s *codexDedicatedImageReplayStoreStub) GetCodexDedicatedImageReplay(_ context.Context, responseID string) ([]byte, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	value, ok := s.values[responseID]
	if !ok {
		return nil, ErrCodexDedicatedImageReplayNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s *codexDedicatedImageReplayStoreStub) SetCodexDedicatedImageReplay(_ context.Context, responseID string, value []byte, ttl time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.ttl = ttl
	s.values[responseID] = append([]byte(nil), value...)
	return nil
}

func (s *codexDedicatedImageReplayStoreStub) DeleteCodexDedicatedImageReplay(_ context.Context, responseID string) error {
	delete(s.values, responseID)
	return nil
}
