package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"

	v1 "seas/api/seas/v1"
	"seas/internal/conf"
	"seas/internal/service"
)

// 链接标记正则：[[显示文字|目标视图|{"param":"value"}]]
var linkPattern = regexp.MustCompile(`(?s)\[\[(.+?)\|(.+?)\|\{(.+?)\}\]\]`)

type AIAnalysisRequest struct {
	View   string            `json:"view"`
	ExamID string            `json:"examId"`
	Params map[string]string `json:"params,omitempty"`
}

type AILink struct {
	Label      string            `json:"label"`
	TargetView string            `json:"targetView"`
	Params     map[string]string `json:"params,omitempty"`
}

type AISegment struct {
	Type    string  `json:"type"` // "text" or "link"
	Content string  `json:"content"`
	Link    *AILink `json:"link,omitempty"`
}

type AIAnalysisResponse struct {
	Segments    []AISegment `json:"segments"`
	GeneratedAt int64       `json:"generatedAt"`
}

type AIAnalysisHandler struct {
	analysis *service.AnalysisService
	llmConf  *conf.LLM
	logger   *log.Helper

	modelOnce sync.Once
	model     einomodel.ChatModel
	modelErr  error
}

func NewAIAnalysisHandler(analysis *service.AnalysisService, llm *conf.LLM, logger log.Logger) *AIAnalysisHandler {
	return &AIAnalysisHandler{
		analysis: analysis,
		llmConf:  llm,
		logger:   log.NewHelper(logger),
	}
}

func (h *AIAnalysisHandler) chatModel() (einomodel.ChatModel, error) {
	h.modelOnce.Do(func() {
		if h.llmConf == nil {
			h.modelErr = fmt.Errorf("llm config is missing")
			return
		}
		model := strings.TrimSpace(h.llmConf.GetModel())
		apiKey := strings.TrimSpace(h.llmConf.GetApiKey())
		if model == "" || apiKey == "" {
			h.modelErr = fmt.Errorf("llm config requires model and api_key")
			return
		}
		var temp float32
		if t := h.llmConf.GetTemperature(); t != 0 {
			temp = float32(t)
		} else {
			temp = 0.2
		}
		h.model, h.modelErr = ark.NewChatModel(context.Background(), &ark.ChatModelConfig{
			BaseURL:     strings.TrimSpace(h.llmConf.GetApiBase()),
			Region:      strings.TrimSpace(h.llmConf.GetRegion()),
			APIKey:      apiKey,
			AccessKey:   strings.TrimSpace(h.llmConf.GetAccessKey()),
			SecretKey:   strings.TrimSpace(h.llmConf.GetSecretKey()),
			Model:       model,
			Temperature: &temp,
		})
	})
	return h.model, h.modelErr
}

func (h *AIAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req AIAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.View) == "" || strings.TrimSpace(req.ExamID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "view and examId are required"})
		return
	}

	result, err := h.generateAnalysis(r.Context(), req)
	if err != nil {
		h.logger.Errorf("ai analysis failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "智能分析服务暂时不可用"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AIAnalysisHandler) generateAnalysis(ctx context.Context, req AIAnalysisRequest) (*AIAnalysisResponse, error) {
	prompt, err := h.buildPrompt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	chatModel, err := h.chatModel()
	if err != nil {
		return nil, fmt.Errorf("init chat model: %w", err)
	}

	msg, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(prompt),
	})
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	segments := parseSegments(msg.Content)
	return &AIAnalysisResponse{
		Segments:    segments,
		GeneratedAt: time.Now().UnixMilli(),
	}, nil
}

func (h *AIAnalysisHandler) buildPrompt(ctx context.Context, req AIAnalysisRequest) (string, error) {
	examID := parseInt64(req.ExamID)

	switch req.View {
	case "class-summary":
		return h.buildClassSummaryPrompt(ctx, examID, req.Params)
	case "subject-summary":
		return h.buildSubjectSummaryPrompt(ctx, examID, req.Params)
	case "rating-analysis":
		return h.buildRatingAnalysisPrompt(ctx, examID, req.Params)
	case "class-subject-summary":
		return h.buildClassSubjectSummaryPrompt(ctx, examID, req.Params)
	case "single-class-summary":
		return h.buildSingleClassSummaryPrompt(ctx, examID, req.Params)
	case "single-class-question":
		return h.buildSingleClassQuestionPrompt(ctx, examID, req.Params)
	case "single-question-summary":
		return h.buildSingleQuestionSummaryPrompt(ctx, examID, req.Params)
	case "single-question-detail":
		return h.buildSingleQuestionDetailPrompt(ctx, examID, req.Params)
	default:
		return "", fmt.Errorf("unsupported view: %s", req.View)
	}
}

func (h *AIAnalysisHandler) buildClassSummaryPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	reply, err := h.analysis.GetClassSummary(ctx, &v1.GetClassSummaryRequest{
		ExamId: strconv.FormatInt(examID, 10),
		Scope:  "all_subjects",
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下班级汇总数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "考试名称：%s\n", reply.GetExamName())
	fmt.Fprintf(&b, "参与人数：%d\n", reply.GetTotalParticipants())
	if overall := reply.GetOverallGrade(); overall != nil {
		fmt.Fprintf(&b, "年级概览：平均分 %.2f，最高分 %.2f，最低分 %.2f，标准差 %.2f\n",
			overall.GetAvgScore(), overall.GetHighestScore(), overall.GetLowestScore(), overall.GetStdDev())
	}
	fmt.Fprintf(&b, "各班明细：\n")
	for _, class := range reply.GetClassDetails() {
		fmt.Fprintf(&b, "  %s：%d人，均分%.2f，最高%.2f，最低%.2f，标准差%.2f\n",
			class.GetClassName(), class.GetTotalStudents(), class.GetAvgScore(),
			class.GetHighestScore(), class.GetLowestScore(), class.GetStdDev())
	}
	fmt.Fprintf(&b, "\n【分析要求】\n")
	fmt.Fprintf(&b, "1. 指出表现突出和需要关注的班级\n")
	fmt.Fprintf(&b, "2. 从教学管理角度给出1-2条具体建议\n")
	fmt.Fprintf(&b, "3. 语气专业、简洁\n")
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "当你提到具体班级名称时，请使用以下格式包裹：\n")
	fmt.Fprintf(&b, "[[班级名称|single-class-summary|{\"classId\":\"班级ID\"}]]\n")
	fmt.Fprintf(&b, "\n只输出分析文本，不要加标题、不要加总结。")
	return b.String(), nil
}

// 其余视图的 prompt 方法参照 class-summary 实现
// 每个视图调用对应的 analysis.GetXxx 方法获取数据，格式化后组装 prompt

func (h *AIAnalysisHandler) buildSubjectSummaryPrompt(ctx context.Context, examID int64, _ map[string]string) (string, error) {
	reply, err := h.analysis.GetSubjectSummary(ctx, &v1.GetSubjectSummaryRequest{
		ExamId: strconv.FormatInt(examID, 10),
		Scope:  "all_subjects",
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下学科汇总数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "考试名称：%s\n", reply.GetExamName())
	fmt.Fprintf(&b, "参与人数：%d\n", reply.GetTotalParticipants())
	fmt.Fprintf(&b, "学科明细：\n")
	for _, s := range reply.GetSubjects() {
		fmt.Fprintf(&b, "  %s：满分%.0f，均分%.2f，难度%.2f，区分度%.2f\n",
			s.GetName(), s.GetFullScore(), s.GetAvgScore(), s.GetDifficulty(), s.GetDiscrimination())
	}
	fmt.Fprintf(&b, "\n【分析要求】\n")
	fmt.Fprintf(&b, "1. 指出优势和薄弱学科\n")
	fmt.Fprintf(&b, "2. 从教学安排角度给出建议\n")
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "当你提到具体学科名称时，请使用以下格式包裹：\n")
	fmt.Fprintf(&b, "[[学科名称|single-class-summary|{\"subjectId\":\"学科ID\"}]]\n")
	fmt.Fprintf(&b, "\n只输出分析文本，不要加标题、不要加总结。")
	return b.String(), nil
}

func (h *AIAnalysisHandler) buildRatingAnalysisPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	req := &v1.GetRatingDistributionRequest{
		ExamId: strconv.FormatInt(examID, 10),
		Scope:  "all_subjects",
	}
	reply, err := h.analysis.GetRatingDistribution(ctx, req)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下四率分析数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "考试名称：%s\n", reply.GetExamName())
	if cfg := reply.GetConfig(); cfg != nil {
		fmt.Fprintf(&b, "阈值：优秀%.0f，良好%.0f，合格%.0f\n",
			cfg.GetExcellentThreshold(), cfg.GetGoodThreshold(), cfg.GetPassThreshold())
	}
	if overall := reply.GetOverallGrade(); overall != nil {
		fmt.Fprintf(&b, "年级整体：优秀%.1f%%，良好%.1f%%，合格%.1f%%，低分%.1f%%\n",
			overall.GetExcellent().GetPercentage(), overall.GetGood().GetPercentage(),
			overall.GetPass().GetPercentage(), overall.GetFail().GetPercentage())
	}
	fmt.Fprintf(&b, "各班明细：\n")
	for _, class := range reply.GetClassDetails() {
		fmt.Fprintf(&b, "  %s：优秀%.1f%%，良好%.1f%%，合格%.1f%%\n",
			class.GetClassName(), class.GetExcellent().GetPercentage(),
			class.GetGood().GetPercentage(), class.GetPass().GetPercentage())
	}
	fmt.Fprintf(&b, "\n【分析要求】\n")
	fmt.Fprintf(&b, "1. 评价整体四率表现\n")
	fmt.Fprintf(&b, "2. 指出需要重点关注的班级\n")
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "[[班级名称|single-class-summary|{\"classId\":\"班级ID\"}]]\n")
	fmt.Fprintf(&b, "\n只输出分析文本。")
	return b.String(), nil
}

func (h *AIAnalysisHandler) buildClassSubjectSummaryPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	classID := params["classId"]
	if classID == "" {
		return "", fmt.Errorf("classId is required for class-subject-summary")
	}
	reply, err := h.analysis.GetClassSubjectSummary(ctx, &v1.GetClassSubjectSummaryRequest{
		ExamId:  strconv.FormatInt(examID, 10),
		ClassId: classID,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下班级学科汇总数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "班级：%s\n", reply.GetClassName())
	for _, s := range reply.GetSubjects() {
		fmt.Fprintf(&b, "  %s：班均%.2f，年级均%.2f，差%.2f，班排%d/%d\n",
			s.GetSubjectName(), s.GetClassAvgScore(), s.GetGradeAvgScore(),
			s.GetScoreDiff(), s.GetClassRank(), s.GetTotalClasses())
	}
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "[[学科名称|single-class-question|{\"subjectId\":\"学科ID\",\"classId\":\"%s\"}]]\n", classID)
	fmt.Fprintf(&b, "\n只输出分析文本。")
	return b.String(), nil
}

func (h *AIAnalysisHandler) buildSingleClassSummaryPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	subjectID := params["subjectId"]
	if subjectID == "" {
		return "", fmt.Errorf("subjectId is required for single-class-summary")
	}
	reply, err := h.analysis.GetSingleClassSummary(ctx, &v1.GetSingleClassSummaryRequest{
		ExamId:    strconv.FormatInt(examID, 10),
		SubjectId: subjectID,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下单科班级汇总数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "学科：%s\n", reply.GetSubjectName())
	for _, class := range reply.GetClasses() {
		fmt.Fprintf(&b, "  %s：%d人，均分%.2f，最高%.2f，最低%.2f\n",
			class.GetClassName(), class.GetTotalStudents(), class.GetAvgScore(),
			class.GetHighestScore(), class.GetLowestScore())
	}
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "[[班级名称|single-class-question|{\"classId\":\"班级ID\",\"subjectId\":\"%s\"}]]\n", subjectID)
	fmt.Fprintf(&b, "\n只输出分析文本。")
	return b.String(), nil
}

func (h *AIAnalysisHandler) buildSingleClassQuestionPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	subjectID := params["subjectId"]
	classID := params["classId"]
	if subjectID == "" || classID == "" {
		return "", fmt.Errorf("subjectId and classId are required for single-class-question")
	}
	reply, err := h.analysis.GetSingleClassQuestions(ctx, &v1.GetSingleClassQuestionsRequest{
		ExamId:    strconv.FormatInt(examID, 10),
		SubjectId: subjectID,
		ClassId:   classID,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下单科班级题目数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "学科：%s，班级：%s\n", reply.GetSubjectName(), reply.GetClassName())
	for _, q := range reply.GetQuestions() {
		fmt.Fprintf(&b, "  第%s题（%s）：满分%.0f，班均%.2f，得分率%.1f%%\n",
			q.GetQuestionNumber(), q.GetQuestionType(), q.GetFullScore(), q.GetClassAvgScore(), q.GetScoreRate()*100)
	}
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "[[题号|single-question-summary|{\"subjectId\":\"%s\"}]]\n", subjectID)
	fmt.Fprintf(&b, "\n只输出分析文本。")
	return b.String(), nil
}

func (h *AIAnalysisHandler) buildSingleQuestionSummaryPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	subjectID := params["subjectId"]
	if subjectID == "" {
		return "", fmt.Errorf("subjectId is required for single-question-summary")
	}
	reply, err := h.analysis.GetSingleQuestionSummary(ctx, &v1.GetSingleQuestionSummaryRequest{
		ExamId:    strconv.FormatInt(examID, 10),
		SubjectId: subjectID,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下单科题目汇总数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "学科：%s\n", reply.GetSubjectName())
	for _, q := range reply.GetQuestions() {
		fmt.Fprintf(&b, "  第%s题（%s）：年级均%.2f，得分率%.1f%%\n",
			q.GetQuestionNumber(), q.GetQuestionType(), q.GetGradeAvgScore(), q.GetScoreRate()*100)
	}
	fmt.Fprintf(&b, "\n【链接规则】\n")
	fmt.Fprintf(&b, "[[题号|single-question-detail|{\"subjectId\":\"%s\",\"questionId\":\"题目ID\"}]]\n", subjectID)
	fmt.Fprintf(&b, "\n只输出分析文本。")
	return b.String(), nil
}

func (h *AIAnalysisHandler) buildSingleQuestionDetailPrompt(ctx context.Context, examID int64, params map[string]string) (string, error) {
	subjectID := params["subjectId"]
	classID := params["classId"]
	questionID := params["questionId"]
	if subjectID == "" || classID == "" || questionID == "" {
		return "", fmt.Errorf("subjectId, classId and questionId are required for single-question-detail")
	}
	reply, err := h.analysis.GetSingleQuestionDetail(ctx, &v1.GetSingleQuestionDetailRequest{
		ExamId:     strconv.FormatInt(examID, 10),
		SubjectId:  subjectID,
		ClassId:    classID,
		QuestionId: questionID,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是一位学业数据分析专家。请基于以下题目详情数据生成一段简要分析（80-120字）。\n\n")
	fmt.Fprintf(&b, "【数据】\n")
	fmt.Fprintf(&b, "学科：%s，班级：%s，第%s题（%s）\n",
		reply.GetSubjectName(), reply.GetClassName(), reply.GetQuestionNumber(), reply.GetQuestionType())
	students := reply.GetStudents()
	if len(students) > 0 {
		fmt.Fprintf(&b, "满分%.0f，得分率%.1f%%\n", reply.GetFullScore(), students[0].GetScoreRate()*100)
	} else {
		fmt.Fprintf(&b, "满分%.0f\n", reply.GetFullScore())
	}
	fmt.Fprintf(&b, "\n【分析要求】\n")
	fmt.Fprintf(&b, "1. 分析该题的得分分布\n")
	fmt.Fprintf(&b, "2. 给出针对性的讲评建议\n")
	fmt.Fprintf(&b, "\n只输出分析文本。")
	return b.String(), nil
}

func parseSegments(content string) []AISegment {
	if content == "" {
		return []AISegment{{Type: "text", Content: "暂无分析数据。"}}
	}

	var segments []AISegment
	lastIndex := 0

	matches := linkPattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		start, end := match[0], match[1]
		if start > lastIndex {
			segments = append(segments, AISegment{
				Type:    "text",
				Content: strings.TrimSpace(content[lastIndex:start]),
			})
		}

		groups := linkPattern.FindStringSubmatch(content[start:end])
		if len(groups) == 4 {
			var params map[string]string
			_ = json.Unmarshal([]byte("{"+groups[3]+"}"), &params)
			segments = append(segments, AISegment{
				Type:    "link",
				Content: strings.TrimSpace(groups[1]),
				Link: &AILink{
					Label:      strings.TrimSpace(groups[1]),
					TargetView: strings.TrimSpace(groups[2]),
					Params:     params,
				},
			})
		}
		lastIndex = end
	}

	if lastIndex < len(content) {
		remaining := strings.TrimSpace(content[lastIndex:])
		if remaining != "" {
			segments = append(segments, AISegment{
				Type:    "text",
				Content: remaining,
			})
		}
	}

	if len(segments) == 0 {
		return []AISegment{{Type: "text", Content: content}}
	}
	return segments
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
