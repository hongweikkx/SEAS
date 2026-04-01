package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pb "seas/api/seas/v1"
	"seas/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	defaultChatTimeout   = 10000 * time.Second
	defaultChatMaxRounds = 8
)

type ChatHandler struct {
	llm    *conf.LLM
	tools  *AnalysisToolBridge
	logger *log.Helper
	client *http.Client
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ChatRequest struct {
	Message string        `json:"message"`
	History []ChatMessage `json:"history,omitempty"`
}

type ChatResponse struct {
	Model     string           `json:"model"`
	Answer    string           `json:"answer"`
	Blocks    []map[string]any `json:"blocks,omitempty"`
	ToolCalls []ToolCall       `json:"toolCalls,omitempty"`
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []map[string]any    `json:"messages"`
	Tools       []openAIRequestTool `json:"tools,omitempty"`
	ToolChoice  string              `json:"tool_choice,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
}

type openAIRequestTool struct {
	Type     string                `json:"type"`
	Function openAIRequestFunction `json:"function"`
}

type openAIRequestFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewChatHandler(llm *conf.LLM, tools *AnalysisToolBridge, logger log.Logger) *ChatHandler {
	return &ChatHandler{
		llm:    llm,
		tools:  tools,
		logger: log.NewHelper(logger),
		client: &http.Client{Timeout: defaultChatTimeout},
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

	answer, toolCalls, blocks, err := h.chat(r.Context(), req)
	if err != nil {
		h.logger.Errorf("chat failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Model:     h.llm.GetModel(),
		Answer:    answer,
		Blocks:    blocks,
		ToolCalls: toolCalls,
	})
}

type toolResultRecord struct {
	Name      string
	Arguments map[string]any
	Result    any
}

func (h *ChatHandler) chat(ctx context.Context, req ChatRequest) (string, []ToolCall, []map[string]any, error) {
	if h.llm == nil {
		return "", nil, nil, fmt.Errorf("llm config is missing")
	}
	if strings.TrimSpace(h.llm.GetApiKey()) == "" || strings.TrimSpace(h.llm.GetApiBase()) == "" || strings.TrimSpace(h.llm.GetModel()) == "" {
		return "", nil, nil, fmt.Errorf("llm config requires model, api_key and api_base")
	}

	messages := make([]map[string]any, 0, len(req.History)+defaultChatMaxRounds+2)
	messages = append(messages, map[string]any{
		"role":    "system",
		"content": "You are a data analysis assistant for school exam reporting. Use tools whenever user requests depend on exam, subject, class, or rating data. If a tool result is insufficient, explain what is missing instead of guessing.",
	})
	for _, item := range req.History {
		msg, err := toOpenAIMessage(item)
		if err != nil {
			return "", nil, nil, err
		}
		messages = append(messages, msg)
	}
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": req.Message,
	})

	usedToolCalls := make([]ToolCall, 0)
	toolResults := make([]toolResultRecord, 0)
	for round := 0; round < defaultChatMaxRounds; round++ {
		resp, err := h.callModel(ctx, messages)
		if err != nil {
			return "", usedToolCalls, nil, err
		}
		if len(resp.Choices) == 0 {
			return "", usedToolCalls, nil, fmt.Errorf("llm returned no choices")
		}

		message := resp.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			answer := strings.TrimSpace(message.Content)
			if answer == "" {
				return "", usedToolCalls, nil, fmt.Errorf("llm returned empty response")
			}
			return answer, usedToolCalls, buildBlocks(toolResults), nil
		}

		assistantMessage := map[string]any{
			"role": "assistant",
		}
		toolCalls := make([]map[string]any, 0, len(message.ToolCalls))
		for _, toolCall := range message.ToolCalls {
			args := map[string]any{}
			if strings.TrimSpace(toolCall.Function.Arguments) != "" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					return "", usedToolCalls, nil, fmt.Errorf("invalid tool arguments for %s: %w", toolCall.Function.Name, err)
				}
			}
			toolCalls = append(toolCalls, map[string]any{
				"id":   toolCall.ID,
				"type": toolCall.Type,
				"function": map[string]any{
					"name":      toolCall.Function.Name,
					"arguments": toolCall.Function.Arguments,
				},
			})
			usedToolCalls = append(usedToolCalls, ToolCall{
				Name:      toolCall.Function.Name,
				Arguments: args,
			})
		}
		assistantMessage["tool_calls"] = toolCalls
		messages = append(messages, assistantMessage)

		for _, toolCall := range message.ToolCalls {
			args := map[string]any{}
			if strings.TrimSpace(toolCall.Function.Arguments) != "" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					return "", usedToolCalls, nil, fmt.Errorf("invalid tool arguments for %s: %w", toolCall.Function.Name, err)
				}
			}
			result, err := h.tools.Call(ctx, toolCall.Function.Name, args)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			}
			toolResults = append(toolResults, toolResultRecord{
				Name:      toolCall.Function.Name,
				Arguments: args,
				Result:    result,
			})
			payload, err := json.Marshal(result)
			if err != nil {
				return "", usedToolCalls, nil, fmt.Errorf("marshal tool result for %s: %w", toolCall.Function.Name, err)
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": toolCall.ID,
				"content":      string(payload),
			})
		}
	}

	return "", usedToolCalls, buildBlocks(toolResults), fmt.Errorf("tool call rounds exceeded limit")
}

func (h *ChatHandler) callModel(ctx context.Context, messages []map[string]any) (*openAIChatResponse, error) {
	body, err := json.Marshal(openAIChatRequest{
		Model:       h.llm.GetModel(),
		Messages:    messages,
		Tools:       h.openAITools(),
		ToolChoice:  "auto",
		Temperature: 0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal llm request: %w", err)
	}

	endpoint := strings.TrimRight(h.llm.GetApiBase(), "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create llm request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+h.llm.GetApiKey())
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call llm api: %w, req:%s", err, endpoint)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode llm response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return nil, fmt.Errorf("llm api error: %s", parsed.Error.Message)
		}
		return nil, fmt.Errorf("llm api returned status %d", resp.StatusCode)
	}

	return &parsed, nil
}

func (h *ChatHandler) openAITools() []openAIRequestTool {
	defs := h.tools.Definitions()
	tools := make([]openAIRequestTool, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, openAIRequestTool{
			Type: "function",
			Function: openAIRequestFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.InputSchema,
			},
		})
	}
	return tools
}

func toOpenAIMessage(msg ChatMessage) (map[string]any, error) {
	role := strings.TrimSpace(msg.Role)
	if role == "" {
		return nil, fmt.Errorf("history role is required")
	}

	out := map[string]any{"role": role}
	if msg.Content != "" {
		out["content"] = msg.Content
	}
	if msg.ToolCallID != "" {
		out["tool_call_id"] = msg.ToolCallID
	}
	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
		for index, toolCall := range msg.ToolCalls {
			argBytes, err := json.Marshal(toolCall.Arguments)
			if err != nil {
				return nil, fmt.Errorf("marshal history tool arguments: %w", err)
			}
			toolCalls = append(toolCalls, map[string]any{
				"id":   fmt.Sprintf("history_tool_%d", index),
				"type": "function",
				"function": map[string]any{
					"name":      toolCall.Name,
					"arguments": string(argBytes),
				},
			})
		}
		out["tool_calls"] = toolCalls
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
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
	case *pb.ListExamsReply:
		return buildListExamsBlocks(data)
	case *pb.ListSubjectsByExamReply:
		return buildListSubjectsBlocks(data)
	case *pb.GetSubjectSummaryReply:
		return buildSubjectSummaryBlocks(data)
	case *pb.GetClassSummaryReply:
		return buildClassSummaryBlocks(data)
	case *pb.GetRatingDistributionReply:
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

func buildListExamsBlocks(reply *pb.ListExamsReply) []map[string]any {
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

func buildListSubjectsBlocks(reply *pb.ListSubjectsByExamReply) []map[string]any {
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

func buildSubjectSummaryBlocks(reply *pb.GetSubjectSummaryReply) []map[string]any {
	blocks := make([]map[string]any, 0)

	rows := make([]map[string]any, 0, len(reply.GetSubjects()))
	labels := make([]string, 0, len(reply.GetSubjects()))
	avgScores := make([]float64, 0, len(reply.GetSubjects()))
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
		labels = append(labels, subject.GetName())
		avgScores = append(avgScores, subject.GetAvgScore())
	}

	blocks = append(blocks, map[string]any{
		"type":        "table",
		"title":       "学科情况汇总",
		"description": fmt.Sprintf("考试 %s，共 %d 名参考学生", reply.GetExamName(), reply.GetTotalParticipants()),
		"columns": []map[string]any{
			{"key": "name", "label": "学科"},
			{"key": "fullScore", "label": "满分", "align": "right"},
			{"key": "avgScore", "label": "平均分", "align": "right"},
			{"key": "highestScore", "label": "最高分", "align": "right"},
			{"key": "lowestScore", "label": "最低分", "align": "right"},
			{"key": "difficulty", "label": "难度", "align": "right"},
			{"key": "studentCount", "label": "参考人数", "align": "right"},
		},
		"rows": rows,
	})

	if len(labels) > 0 {
		blocks = append(blocks, map[string]any{
			"type":        "chart",
			"title":       "学科平均分",
			"description": "按学科展示平均分对比",
			"chartType":   "bar",
			"labels":      labels,
			"datasets": []map[string]any{
				{"label": "平均分", "data": avgScores},
			},
		})
	}

	return blocks
}

func buildClassSummaryBlocks(reply *pb.GetClassSummaryReply) []map[string]any {
	blocks := make([]map[string]any, 0)

	rows := make([]map[string]any, 0, len(reply.GetClassDetails())+1)
	labels := make([]string, 0, len(reply.GetClassDetails())+1)
	avgScores := make([]float64, 0, len(reply.GetClassDetails())+1)

	if reply.GetOverallGrade() != nil {
		overall := reply.GetOverallGrade()
		rows = append(rows, map[string]any{
			"className":      overall.GetClassName(),
			"totalStudents":  overall.GetTotalStudents(),
			"avgScore":       overall.GetAvgScore(),
			"highestScore":   overall.GetHighestScore(),
			"lowestScore":    overall.GetLowestScore(),
			"scoreDeviation": overall.GetScoreDeviation(),
			"difficulty":     overall.GetDifficulty(),
			"stdDev":         overall.GetStdDev(),
		})
		labels = append(labels, overall.GetClassName())
		avgScores = append(avgScores, overall.GetAvgScore())
	}

	for _, class := range reply.GetClassDetails() {
		rows = append(rows, map[string]any{
			"className":      class.GetClassName(),
			"totalStudents":  class.GetTotalStudents(),
			"avgScore":       class.GetAvgScore(),
			"highestScore":   class.GetHighestScore(),
			"lowestScore":    class.GetLowestScore(),
			"scoreDeviation": class.GetScoreDeviation(),
			"difficulty":     class.GetDifficulty(),
			"stdDev":         class.GetStdDev(),
		})
		labels = append(labels, class.GetClassName())
		avgScores = append(avgScores, class.GetAvgScore())
	}

	blocks = append(blocks, map[string]any{
		"type":        "table",
		"title":       "班级情况汇总",
		"description": fmt.Sprintf("考试 %s，共 %d 名参考学生", reply.GetExamName(), reply.GetTotalParticipants()),
		"columns": []map[string]any{
			{"key": "className", "label": "班级"},
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

	if len(labels) > 0 {
		blocks = append(blocks, map[string]any{
			"type":        "chart",
			"title":       "班级平均分对比",
			"description": "按班级展示平均分",
			"chartType":   "bar",
			"labels":      labels,
			"datasets": []map[string]any{
				{"label": "平均分", "data": avgScores},
			},
		})
	}

	return blocks
}

func buildRatingDistributionBlocks(reply *pb.GetRatingDistributionReply) []map[string]any {
	blocks := make([]map[string]any, 0)

	rows := make([]map[string]any, 0, len(reply.GetClassDetails())+1)
	labels := make([]string, 0, len(reply.GetClassDetails())+1)
	excellent := make([]float64, 0, len(reply.GetClassDetails())+1)
	good := make([]float64, 0, len(reply.GetClassDetails())+1)
	pass := make([]float64, 0, len(reply.GetClassDetails())+1)
	fail := make([]float64, 0, len(reply.GetClassDetails())+1)

	if reply.GetOverallGrade() != nil {
		overall := reply.GetOverallGrade()
		rows = append(rows, ratingRow(overall))
		labels = append(labels, overall.GetClassName())
		excellent = append(excellent, overall.GetExcellent().GetPercentage())
		good = append(good, overall.GetGood().GetPercentage())
		pass = append(pass, overall.GetPass().GetPercentage())
		fail = append(fail, overall.GetFail().GetPercentage())
	}

	for _, class := range reply.GetClassDetails() {
		rows = append(rows, ratingRow(class))
		labels = append(labels, class.GetClassName())
		excellent = append(excellent, class.GetExcellent().GetPercentage())
		good = append(good, class.GetGood().GetPercentage())
		pass = append(pass, class.GetPass().GetPercentage())
		fail = append(fail, class.GetFail().GetPercentage())
	}

	blocks = append(blocks, map[string]any{
		"type":        "table",
		"title":       "四率分析",
		"description": fmt.Sprintf("考试 %s，优秀 %.0f / 良好 %.0f / 合格 %.0f 分", reply.GetExamName(), reply.GetConfig().GetExcellentThreshold(), reply.GetConfig().GetGoodThreshold(), reply.GetConfig().GetPassThreshold()),
		"columns": []map[string]any{
			{"key": "className", "label": "班级"},
			{"key": "avgScore", "label": "平均分", "align": "right"},
			{"key": "excellentCount", "label": "优秀人数", "align": "right"},
			{"key": "excellentPercentage", "label": "优秀率", "align": "right"},
			{"key": "goodCount", "label": "良好人数", "align": "right"},
			{"key": "goodPercentage", "label": "良好率", "align": "right"},
			{"key": "passCount", "label": "合格人数", "align": "right"},
			{"key": "passPercentage", "label": "合格率", "align": "right"},
			{"key": "failCount", "label": "低分人数", "align": "right"},
			{"key": "failPercentage", "label": "低分率", "align": "right"},
		},
		"rows": rows,
	})

	if len(labels) > 0 {
		blocks = append(blocks, map[string]any{
			"type":        "chart",
			"title":       "四率分布",
			"description": "按班级展示优秀、良好、合格、低分百分比",
			"chartType":   "bar",
			"labels":      labels,
			"datasets": []map[string]any{
				{"label": "优秀", "data": excellent},
				{"label": "良好", "data": good},
				{"label": "合格", "data": pass},
				{"label": "低分", "data": fail},
			},
		})
	}

	return blocks
}

func ratingRow(item interface {
	GetClassName() string
	GetAvgScore() float64
	GetExcellent() *pb.RatingItem
	GetGood() *pb.RatingItem
	GetPass() *pb.RatingItem
	GetFail() *pb.RatingItem
}) map[string]any {
	return map[string]any{
		"className":           item.GetClassName(),
		"avgScore":            item.GetAvgScore(),
		"excellentCount":      item.GetExcellent().GetCount(),
		"excellentPercentage": item.GetExcellent().GetPercentage(),
		"goodCount":           item.GetGood().GetCount(),
		"goodPercentage":      item.GetGood().GetPercentage(),
		"passCount":           item.GetPass().GetCount(),
		"passPercentage":      item.GetPass().GetPercentage(),
		"failCount":           item.GetFail().GetCount(),
		"failPercentage":      item.GetFail().GetPercentage(),
	}
}
