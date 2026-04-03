package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	v1 "seas/api/seas/v1"
	"seas/internal/conf"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	defaultChatTimeout       = 10 * time.Minute
	defaultChatMaxIterations = 8
	defaultSystemPrompt      = "You are a data analysis assistant for school exam reporting. Use tools whenever user requests depend on exam, subject, class, or rating data. If a tool result is insufficient, explain what is missing instead of guessing."
)

type ChatHandler struct {
	llm    *conf.LLM
	tools  *AnalysisToolBridge
	logger *log.Helper

	runnerOnce sync.Once
	runner     *adk.Runner
	runnerErr  error
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ChatRatingConfig struct {
	ExcellentThreshold float64 `json:"excellent_threshold,omitempty"`
	GoodThreshold      float64 `json:"good_threshold,omitempty"`
	PassThreshold      float64 `json:"pass_threshold,omitempty"`
}

type ChatContext struct {
	Scope        string            `json:"scope,omitempty"`
	ExamID       string            `json:"examId,omitempty"`
	ExamName     string            `json:"examName,omitempty"`
	SubjectID    string            `json:"subjectId,omitempty"`
	SubjectName  string            `json:"subjectName,omitempty"`
	RatingConfig *ChatRatingConfig `json:"ratingConfig,omitempty"`
}

type ChatRequest struct {
	Message string        `json:"message"`
	History []ChatMessage `json:"history,omitempty"`
	Context ChatContext   `json:"context,omitempty"`
}

type toolResultRecorder struct {
	mu      sync.Mutex
	records []toolResultRecord
}

type toolRecorderContextKey struct{}

func NewChatHandler(llm *conf.LLM, tools *AnalysisToolBridge, logger log.Logger) *ChatHandler {
	return &ChatHandler{
		llm:    llm,
		tools:  tools,
		logger: log.NewHelper(logger),
	}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	h.logChatRequest(req)
	h.streamChat(w, r, req)
}

func (h *ChatHandler) streamChat(w http.ResponseWriter, r *http.Request, req ChatRequest) {
	if err := h.ensureRunner(); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported by this response writer"})
		return
	}

	writeSSEHeaders(w)

	surfaceID := fmt.Sprintf("chat-%d", time.Now().UnixNano())
	rootID := surfaceID + "-root"
	bodyID := surfaceID + "-body"
	assistantTextID := surfaceID + "-assistant-text"
	toolSectionID := surfaceID + "-tool-section"
	resultSectionID := surfaceID + "-result-section"

	send := func(msg A2UIMessage) bool {
		if err := writeSSEMessage(w, msg); err != nil {
			h.logger.Errorf("send a2ui message failed: %v", err)
			return false
		}
		flusher.Flush()
		return true
	}

	if !send(A2UIMessage{
		BeginRendering: &BeginRenderingMsg{
			SurfaceID:       surfaceID,
			RootComponentID: rootID,
			Title:           "分析助手",
		},
	}) {
		return
	}

	if !send(A2UIMessage{
		SurfaceUpdate: &SurfaceUpdateMsg{
			SurfaceID: surfaceID,
			Components: []A2UIComponent{
				surfaceComponentLayout("card", rootID, map[string]any{
					"variant": "assistant",
				}, bodyID),
				surfaceComponentLayout("column", bodyID, map[string]any{
					"gap": "md",
				}, assistantTextID, toolSectionID, resultSectionID),
				surfaceComponentText(assistantTextID, map[string]any{
					"usageHint": "body",
					"content":   "",
					"dataKey":   "assistant.content",
				}),
				surfaceComponentLayout("column", toolSectionID, map[string]any{
					"title": "工具调用",
				}),
				surfaceComponentLayout("column", resultSectionID, map[string]any{
					"title": "分析结果",
				}),
			},
		},
	}) {
		return
	}

	messages, err := buildMessages(req)
	if err != nil {
		_ = send(A2UIMessage{Error: &A2UIErrorMsg{Message: err.Error()}})
		return
	}

	recorder := &toolResultRecorder{}
	ctx := context.WithValue(r.Context(), toolRecorderContextKey{}, recorder)
	iter := h.runner.Run(ctx, messages)

	var assistantContent strings.Builder
	toolCallIDs := make([]string, 0)
	toolCallComponents := make([]A2UIComponent, 0)
	resultComponents := make([]A2UIComponent, 0)

	for {
		if err := ctx.Err(); err != nil {
			_ = send(A2UIMessage{Error: &A2UIErrorMsg{Message: err.Error()}})
			return
		}

		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			_ = send(A2UIMessage{Error: &A2UIErrorMsg{Message: event.Err.Error()}})
			return
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			_ = send(A2UIMessage{
				InterruptRequest: interruptRequestFromEvent(surfaceID, event.Action.Interrupted),
			})
			return
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput
		if mv.IsStreaming {
			if err := streamAssistantMessage(mv.MessageStream, &assistantContent, func(content string) bool {
				return send(A2UIMessage{
					DataModelUpdate: &DataModelUpdateMsg{
						SurfaceID: surfaceID,
						Data: map[string]any{
							"assistant.content": content,
						},
					},
				})
			}); err != nil {
				_ = send(A2UIMessage{Error: &A2UIErrorMsg{Message: err.Error()}})
				return
			}
			continue
		}

		msg, err := mv.GetMessage()
		if err != nil {
			_ = send(A2UIMessage{Error: &A2UIErrorMsg{Message: err.Error()}})
			return
		}
		if msg == nil {
			continue
		}

		switch msg.Role {
		case schema.Assistant:
			if content := strings.TrimSpace(msg.Content); content != "" && assistantContent.Len() == 0 {
				assistantContent.WriteString(content)
				if !send(A2UIMessage{
					DataModelUpdate: &DataModelUpdateMsg{
						SurfaceID: surfaceID,
						Data: map[string]any{
							"assistant.content": assistantContent.String(),
						},
					},
				}) {
					return
				}
			}

			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					componentID := fmt.Sprintf("%s-tool-%d", surfaceID, len(toolCallIDs))
					toolCallIDs = append(toolCallIDs, componentID)
					toolCallComponents = append(toolCallComponents, surfaceComponentLayout("card", componentID, map[string]any{
						"title":       tc.Function.Name,
						"description": formatArguments(tc.Function.Arguments),
					}))
				}

				updater := []A2UIComponent{
					surfaceComponentLayout("column", toolSectionID, map[string]any{
						"title": "工具调用",
					}, toolCallIDs...),
				}
				updater = append(updater, toolCallComponents...)
				if !send(A2UIMessage{
					SurfaceUpdate: &SurfaceUpdateMsg{
						SurfaceID:  surfaceID,
						Components: updater,
					},
				}) {
					return
				}
			}
		case schema.Tool:
			// Tool results are collected through the recorder and rendered after the run completes.
		}
	}

	if text := strings.TrimSpace(assistantContent.String()); text != "" {
		if !send(A2UIMessage{
			DataModelUpdate: &DataModelUpdateMsg{
				SurfaceID: surfaceID,
				Data: map[string]any{
					"assistant.content": text,
				},
			},
		}) {
			return
		}
	}

	resultComponents = append(resultComponents, blocksToA2UIComponents(buildBlocks(recorder.records), surfaceID+"-block")...)
	if len(resultComponents) > 0 {
		resultIDs := make([]string, 0, len(resultComponents))
		for _, component := range resultComponents {
			resultIDs = append(resultIDs, component.ID)
		}
		updates := []A2UIComponent{
			surfaceComponentLayout("column", resultSectionID, map[string]any{
				"title": "分析结果",
			}, resultIDs...),
		}
		updates = append(updates, resultComponents...)
		_ = send(A2UIMessage{
			SurfaceUpdate: &SurfaceUpdateMsg{
				SurfaceID:  surfaceID,
				Components: updates,
			},
		})
	}
}

func (h *ChatHandler) ensureRunner() error {
	h.runnerOnce.Do(func() {
		h.runnerErr = h.buildRunner()
	})
	return h.runnerErr
}

func (h *ChatHandler) buildRunner() error {
	if h.llm == nil {
		return fmt.Errorf("llm config is missing")
	}
	if strings.TrimSpace(h.llm.GetModel()) == "" || strings.TrimSpace(h.llm.GetApiKey()) == "" {
		return fmt.Errorf("llm config requires model and api_key")
	}
	if strings.TrimSpace(h.llm.GetProvider()) != "" && strings.TrimSpace(strings.ToLower(h.llm.GetProvider())) != "ark" {
		return fmt.Errorf("unsupported llm provider: %s", h.llm.GetProvider())
	}
	if h.tools == nil {
		return fmt.Errorf("analysis tools are missing")
	}

	baseTools, err := h.tools.EinoTools()
	if err != nil {
		return err
	}

	chatModel, err := ark.NewChatModel(context.Background(), &ark.ChatModelConfig{
		Timeout:     durationPtr(defaultChatTimeout),
		BaseURL:     strings.TrimSpace(h.llm.GetApiBase()),
		Region:      strings.TrimSpace(h.llm.GetRegion()),
		APIKey:      strings.TrimSpace(h.llm.GetApiKey()),
		AccessKey:   strings.TrimSpace(h.llm.GetAccessKey()),
		SecretKey:   strings.TrimSpace(h.llm.GetSecretKey()),
		Model:       strings.TrimSpace(h.llm.GetModel()),
		Temperature: float32Ptr(float32(defaultTemperature(h.llm.GetTemperature()))),
	})
	if err != nil {
		return fmt.Errorf("create ark chat model: %w", err)
	}

	maxIterations := int(h.llm.GetMaxIterations())
	if maxIterations <= 0 {
		maxIterations = defaultChatMaxIterations
	}

	instruction := strings.TrimSpace(h.llm.GetSystemPrompt())
	if instruction == "" {
		instruction = defaultSystemPrompt
	}

	agent, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name:        "seas-analysis-agent",
		Description: "A school exam analysis assistant that uses structured exam, subject, class, and rating tools.",
		Instruction: instruction,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: baseTools,
			},
		},
		MaxIterations: maxIterations,
	})
	if err != nil {
		return fmt.Errorf("create chat model agent: %w", err)
	}

	h.runner = adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})
	return nil
}

func buildMessages(req ChatRequest) ([]*schema.Message, error) {
	messages := make([]*schema.Message, 0, len(req.History)+2)

	if systemPrompt := buildContextSystemPrompt(req.Context); strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}

	for _, item := range req.History {
		msg, err := toSchemaMessage(item)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	messages = append(messages, schema.UserMessage(req.Message))
	return messages, nil
}

func (h *ChatHandler) logChatRequest(req ChatRequest) {
	if h == nil || h.logger == nil {
		return
	}

	historyPreview := make([]string, 0, len(req.History))
	for _, item := range req.History {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			content = "<empty>"
		}
		if len(content) > 120 {
			content = content[:120] + "…"
		}
		historyPreview = append(historyPreview, fmt.Sprintf("%s:%s", item.Role, content))
	}

	h.logger.Infof(
		"chat request received: message=%q history_count=%d history=%v context=%+v",
		strings.TrimSpace(req.Message),
		len(req.History),
		historyPreview,
		req.Context,
	)
}

func buildContextSystemPrompt(ctx ChatContext) string {
	var b strings.Builder
	base := strings.TrimSpace(defaultSystemPrompt)
	if base != "" {
		b.WriteString(base)
		b.WriteString("\n\n")
	}

	b.WriteString("你会收到一个来自前端界面的分析上下文。它是权威信息，优先于用户的自然语言描述。\n")
	b.WriteString("除非用户明确要求切换或浏览列表，否则不要为了“确认”而调用 list_exams 或 list_subjects_by_exam。\n")

	scope := strings.TrimSpace(ctx.Scope)
	examName := strings.TrimSpace(ctx.ExamName)
	examID := strings.TrimSpace(ctx.ExamID)
	subjectName := strings.TrimSpace(ctx.SubjectName)
	subjectID := strings.TrimSpace(ctx.SubjectID)

	switch scope {
	case "exam_list":
		b.WriteString("当前范围：考试列表。\n")
		b.WriteString("只有当用户明确要求查看、筛选或切换考试时，才调用 list_exams。\n")
	case "all_subjects":
		b.WriteString("当前范围：某次考试的全科分析。\n")
		if examName != "" || examID != "" {
			b.WriteString(fmt.Sprintf("当前考试：%s%s。\n", examName, formatIDSuffix(examID)))
		}
		b.WriteString("不要调用 list_exams 来确认考试，也不要调用 list_subjects_by_exam 来枚举科目；直接基于当前考试执行分析。\n")
	case "single_subject":
		b.WriteString("当前范围：某次考试的单科分析。\n")
		if examName != "" || examID != "" {
			b.WriteString(fmt.Sprintf("当前考试：%s%s。\n", examName, formatIDSuffix(examID)))
		}
		if subjectName != "" || subjectID != "" {
			b.WriteString(fmt.Sprintf("当前学科：%s%s。\n", subjectName, formatIDSuffix(subjectID)))
		}
		b.WriteString("不要调用 list_exams，也不要调用 list_subjects_by_exam 来重新枚举科目；直接基于已选学科进行分析。\n")
	default:
		b.WriteString("当前范围未知，但如果前端已经提供考试或学科，请优先使用这些上下文。\n")
	}

	if cfg := ctx.RatingConfig; cfg != nil {
		b.WriteString(fmt.Sprintf("四率阈值：优秀 %.2f，良好 %.2f，合格 %.2f。\n", cfg.ExcellentThreshold, cfg.GoodThreshold, cfg.PassThreshold))
	}

	return b.String()
}

func formatIDSuffix(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	return fmt.Sprintf(" (ID: %s)", strings.TrimSpace(id))
}

func toSchemaMessage(msg ChatMessage) (*schema.Message, error) {
	role := strings.TrimSpace(msg.Role)
	if role == "" {
		return nil, fmt.Errorf("history role is required")
	}

	switch role {
	case string(schema.System):
		return schema.SystemMessage(msg.Content), nil
	case string(schema.User):
		return schema.UserMessage(msg.Content), nil
	case string(schema.Tool):
		return schema.ToolMessage(msg.Content, msg.ToolCallID), nil
	case string(schema.Assistant):
		toolCalls := make([]schema.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			argBytes, err := json.Marshal(tc.Arguments)
			if err != nil {
				return nil, fmt.Errorf("marshal history tool arguments: %w", err)
			}
			toolCalls = append(toolCalls, schema.ToolCall{
				ID:       fmt.Sprintf("history_tool_%s", tc.Name),
				Type:     "function",
				Function: schema.FunctionCall{Name: tc.Name, Arguments: string(argBytes)},
			})
		}
		return schema.AssistantMessage(msg.Content, toolCalls), nil
	default:
		return nil, fmt.Errorf("unsupported history role: %s", msg.Role)
	}
}

func appendToolResultRecord(ctx context.Context, record toolResultRecord) {
	recorder, _ := ctx.Value(toolRecorderContextKey{}).(*toolResultRecorder)
	if recorder == nil {
		return
	}
	recorder.mu.Lock()
	recorder.records = append(recorder.records, record)
	recorder.mu.Unlock()
}

func durationPtr(v time.Duration) *time.Duration {
	return &v
}

func float32Ptr(v float32) *float32 {
	return &v
}

func defaultTemperature(v float64) float64 {
	if v == 0 {
		return 0.2
	}
	return v
}

func streamAssistantMessage(stream *schema.StreamReader[*schema.Message], content *strings.Builder, emit func(string) bool) error {
	if stream == nil {
		return nil
	}
	defer stream.Close()

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if chunk == nil {
			continue
		}
		if piece := chunk.Content; piece != "" {
			content.WriteString(piece)
			if !emit(content.String()) {
				return fmt.Errorf("failed to emit assistant stream update")
			}
		}
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func interruptRequestFromEvent(surfaceID string, info *adk.InterruptInfo) *InterruptRequestMsg {
	if info == nil {
		return &InterruptRequestMsg{SurfaceID: surfaceID}
	}

	contexts := make([]A2UIInterruptContext, 0, len(info.InterruptContexts))
	for _, ctx := range info.InterruptContexts {
		if ctx == nil {
			continue
		}

		contexts = append(contexts, A2UIInterruptContext{
			ID:      ctx.ID,
			Name:    interruptContextName(ctx),
			Address: fmt.Sprint(ctx.Address),
			Type:    interruptContextType(ctx),
		})
	}

	prompt := ""
	if len(info.InterruptContexts) > 0 && info.InterruptContexts[0] != nil && info.InterruptContexts[0].Info != nil {
		prompt = fmt.Sprint(info.InterruptContexts[0].Info)
	}

	return &InterruptRequestMsg{
		SurfaceID: surfaceID,
		Prompt:    prompt,
		Contexts:  contexts,
	}
}

func interruptContextName(ctx *adk.InterruptCtx) string {
	if ctx == nil || ctx.Info == nil {
		return ""
	}
	return fmt.Sprint(ctx.Info)
}

func interruptContextType(ctx *adk.InterruptCtx) string {
	if ctx == nil || len(ctx.Address) == 0 {
		return ""
	}
	return fmt.Sprint(ctx.Address[len(ctx.Address)-1].Type)
}

func formatArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return ""
	}

	var parsed any
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return arguments
	}

	payload, err := json.Marshal(parsed)
	if err != nil {
		return arguments
	}
	return string(payload)
}

func buildBlocks(results []toolResultRecord) []map[string]any {
	blocks := make([]map[string]any, 0)
	for _, result := range results {
		blocks = append(blocks, buildBlocksForTool(result)...)
	}
	return blocks
}

func buildBlocksForTool(result toolResultRecord) []map[string]any {
	switch data := result.Result.(type) {
	case *v1.ListExamsReply:
		return buildListExamsBlocks(data)
	case *v1.ListSubjectsByExamReply:
		return buildListSubjectsBlocks(data)
	case *v1.GetSubjectSummaryReply:
		return buildSubjectSummaryBlocks(data)
	case *v1.GetClassSummaryReply:
		return buildClassSummaryBlocks(data)
	case *v1.GetRatingDistributionReply:
		return buildRatingDistributionBlocks(data)
	case map[string]any:
		if msg, ok := data["error"].(string); ok && strings.TrimSpace(msg) != "" {
			return []map[string]any{
				{
					"type":    "text",
					"content": fmt.Sprintf("工具 %s 调用失败：%s", result.Name, msg),
				},
			}
		}
	}

	return nil
}

func buildListExamsBlocks(reply *v1.ListExamsReply) []map[string]any {
	rows := make([]map[string]any, 0, len(reply.GetExams()))
	for _, exam := range reply.GetExams() {
		rows = append(rows, map[string]any{
			"id":        exam.GetId(),
			"name":      exam.GetName(),
			"examDate":  exam.GetExamDate(),
			"createdAt": exam.GetCreatedAt(),
		})
	}

	return []map[string]any{
		{
			"type":        "table",
			"title":       "考试列表",
			"description": fmt.Sprintf("共 %d 场考试", reply.GetTotalCount()),
			"columns": []map[string]any{
				{"key": "id", "label": "ID", "align": "right"},
				{"key": "name", "label": "考试名称"},
				{"key": "examDate", "label": "考试时间"},
				{"key": "createdAt", "label": "创建时间"},
			},
			"rows": rows,
		},
	}
}

func buildListSubjectsBlocks(reply *v1.ListSubjectsByExamReply) []map[string]any {
	rows := make([]map[string]any, 0, len(reply.GetSubjects()))
	for _, subject := range reply.GetSubjects() {
		rows = append(rows, map[string]any{
			"id":        subject.GetId(),
			"name":      subject.GetName(),
			"fullScore": subject.GetFullScore(),
		})
	}

	return []map[string]any{
		{
			"type":        "table",
			"title":       "考试学科",
			"description": fmt.Sprintf("考试 %d 共 %d 个学科", reply.GetExamId(), reply.GetTotalCount()),
			"columns": []map[string]any{
				{"key": "id", "label": "ID", "align": "right"},
				{"key": "name", "label": "学科名称"},
				{"key": "fullScore", "label": "满分", "align": "right"},
			},
			"rows": rows,
		},
	}
}

func buildSubjectSummaryBlocks(reply *v1.GetSubjectSummaryReply) []map[string]any {
	columns := []map[string]any{
		{"key": "id", "label": "ID", "align": "right"},
		{"key": "name", "label": "学科名称"},
		{"key": "fullScore", "label": "满分", "align": "right"},
		{"key": "avgScore", "label": "平均分", "align": "right"},
		{"key": "highestScore", "label": "最高分", "align": "right"},
		{"key": "lowestScore", "label": "最低分", "align": "right"},
		{"key": "difficulty", "label": "难度", "align": "right"},
		{"key": "studentCount", "label": "人数", "align": "right"},
	}

	rows := make([]map[string]any, 0, len(reply.GetSubjects()))
	for _, subject := range reply.GetSubjects() {
		rows = append(rows, map[string]any{
			"id":           subject.GetId(),
			"name":         subject.GetName(),
			"fullScore":    subject.GetFullScore(),
			"avgScore":     subject.GetAvgScore(),
			"highestScore": subject.GetHighestScore(),
			"lowestScore":  subject.GetLowestScore(),
			"difficulty":   subject.GetDifficulty(),
			"studentCount": subject.GetStudentCount(),
		})
	}

	return []map[string]any{
		{
			"type":        "text",
			"title":       "学科情况汇总",
			"description": fmt.Sprintf("考试 %s, 范围 %s, 参与人数 %d", reply.GetExamName(), reply.GetScope(), reply.GetTotalParticipants()),
		},
		{
			"type":    "table",
			"title":   "学科详情",
			"columns": columns,
			"rows":    rows,
		},
	}
}

func buildClassSummaryBlocks(reply *v1.GetClassSummaryReply) []map[string]any {
	blocks := []map[string]any{
		{
			"type":        "text",
			"title":       "班级情况汇总",
			"description": fmt.Sprintf("考试 %s, 范围 %s, 参与人数 %d", reply.GetExamName(), reply.GetScope(), reply.GetTotalParticipants()),
		},
	}

	if overall := reply.GetOverallGrade(); overall != nil {
		blocks = append(blocks, map[string]any{
			"type":  "text",
			"title": "年级概览",
			"content": fmt.Sprintf(
				"%s | 平均分 %.2f | 最高分 %.2f | 最低分 %.2f | 离均差 %.2f | 难度 %.2f | 标准差 %.2f",
				overall.GetClassName(),
				overall.GetAvgScore(),
				overall.GetHighestScore(),
				overall.GetLowestScore(),
				overall.GetScoreDeviation(),
				overall.GetDifficulty(),
				overall.GetStdDev(),
			),
		})
	}

	rows := make([]map[string]any, 0, len(reply.GetClassDetails()))
	for _, class := range reply.GetClassDetails() {
		rows = append(rows, map[string]any{
			"classId":        class.GetClassId(),
			"className":      class.GetClassName(),
			"totalStudents":  class.GetTotalStudents(),
			"avgScore":       class.GetAvgScore(),
			"highestScore":   class.GetHighestScore(),
			"lowestScore":    class.GetLowestScore(),
			"scoreDeviation": class.GetScoreDeviation(),
			"difficulty":     class.GetDifficulty(),
			"stdDev":         class.GetStdDev(),
		})
	}

	if len(rows) > 0 {
		blocks = append(blocks, map[string]any{
			"type":  "table",
			"title": "班级详情",
			"columns": []map[string]any{
				{"key": "classId", "label": "班级ID", "align": "right"},
				{"key": "className", "label": "班级名称"},
				{"key": "totalStudents", "label": "人数", "align": "right"},
				{"key": "avgScore", "label": "平均分", "align": "right"},
				{"key": "highestScore", "label": "最高分", "align": "right"},
				{"key": "lowestScore", "label": "最低分", "align": "right"},
				{"key": "scoreDeviation", "label": "离均差", "align": "right"},
				{"key": "difficulty", "label": "难度", "align": "right"},
				{"key": "stdDev", "label": "标准差", "align": "right"},
			},
			"rows": rows,
		})
	}

	return blocks
}

func buildRatingDistributionBlocks(reply *v1.GetRatingDistributionReply) []map[string]any {
	blocks := []map[string]any{
		{
			"type":        "text",
			"title":       "四率分析",
			"description": fmt.Sprintf("考试 %s, 范围 %s, 参与人数 %d", reply.GetExamName(), reply.GetScope(), reply.GetTotalParticipants()),
		},
	}

	if config := reply.GetConfig(); config != nil {
		blocks = append(blocks, map[string]any{
			"type":  "text",
			"title": "阈值配置",
			"content": fmt.Sprintf(
				"优秀 %.2f | 良好 %.2f | 合格 %.2f",
				config.GetExcellentThreshold(),
				config.GetGoodThreshold(),
				config.GetPassThreshold(),
			),
		})
	}

	rows := make([]map[string]any, 0, len(reply.GetClassDetails()))
	for _, class := range reply.GetClassDetails() {
		rows = append(rows, map[string]any{
			"classId":       class.GetClassId(),
			"className":     class.GetClassName(),
			"totalStudents": class.GetTotalStudents(),
			"avgScore":      class.GetAvgScore(),
			"excellent":     class.GetExcellent().GetPercentage(),
			"good":          class.GetGood().GetPercentage(),
			"pass":          class.GetPass().GetPercentage(),
			"fail":          class.GetFail().GetPercentage(),
		})
	}

	if len(rows) > 0 {
		blocks = append(blocks, map[string]any{
			"type":  "table",
			"title": "四率详情",
			"columns": []map[string]any{
				{"key": "classId", "label": "班级ID", "align": "right"},
				{"key": "className", "label": "班级名称"},
				{"key": "totalStudents", "label": "人数", "align": "right"},
				{"key": "avgScore", "label": "平均分", "align": "right"},
				{"key": "excellent", "label": "优秀%", "align": "right"},
				{"key": "good", "label": "良好%", "align": "right"},
				{"key": "pass", "label": "合格%", "align": "right"},
				{"key": "fail", "label": "低分%", "align": "right"},
			},
			"rows": rows,
		})
	}

	return blocks
}
