package server

import (
	"context"
	"fmt"

	v1 "seas/api/seas/v1"
	"seas/internal/service"
)

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type AnalysisToolBridge struct {
	analysis *service.AnalysisService
}

func NewAnalysisToolBridge(analysis *service.AnalysisService) *AnalysisToolBridge {
	return &AnalysisToolBridge{analysis: analysis}
}

func (b *AnalysisToolBridge) Definitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "list_exams",
			Description: "Get a list of exams with pagination support",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_index": map[string]any{"type": "integer", "description": "Page number, starts from 1"},
					"page_size":  map[string]any{"type": "integer", "description": "Page size"},
				},
			},
		},
		{
			Name:        "list_subjects_by_exam",
			Description: "Get a list of subjects associated with a specific exam",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"exam_id":    map[string]any{"type": "integer", "description": "Exam ID"},
					"page_index": map[string]any{"type": "integer", "description": "Page number, starts from 1"},
					"page_size":  map[string]any{"type": "integer", "description": "Page size"},
				},
				"required": []string{"exam_id"},
			},
		},
		{
			Name:        "get_subject_summary",
			Description: "Get summary statistics for subjects in an exam",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"exam_id":    map[string]any{"type": "integer", "description": "Exam ID"},
					"scope":      map[string]any{"type": "string", "description": "all_subjects or single_subject"},
					"subject_id": map[string]any{"type": "integer", "description": "Subject ID when scope is single_subject"},
				},
				"required": []string{"exam_id", "scope"},
			},
		},
		{
			Name:        "get_class_summary",
			Description: "Get summary statistics for classes in an exam/subject",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"exam_id":    map[string]any{"type": "integer", "description": "Exam ID"},
					"scope":      map[string]any{"type": "string", "description": "all_subjects or single_subject"},
					"subject_id": map[string]any{"type": "integer", "description": "Subject ID when scope is single_subject"},
				},
				"required": []string{"exam_id", "scope"},
			},
		},
		{
			Name:        "get_rating_distribution",
			Description: "Get four-rate (excellent, good, pass, fail) analysis for an exam/subject",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"exam_id":             map[string]any{"type": "integer", "description": "Exam ID"},
					"scope":               map[string]any{"type": "string", "description": "all_subjects or single_subject"},
					"subject_id":          map[string]any{"type": "integer", "description": "Subject ID when scope is single_subject"},
					"excellent_threshold": map[string]any{"type": "number", "description": "Excellent score threshold"},
					"good_threshold":      map[string]any{"type": "number", "description": "Good score threshold"},
					"pass_threshold":      map[string]any{"type": "number", "description": "Pass score threshold"},
				},
				"required": []string{"exam_id", "scope"},
			},
		},
	}
}

func (b *AnalysisToolBridge) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	switch name {
	case "list_exams":
		return b.analysis.ListExams(ctx, &v1.ListExamsRequest{
			PageIndex: getInt32Arg(args, "page_index"),
			PageSize:  getInt32Arg(args, "page_size"),
		})
	case "list_subjects_by_exam":
		return b.analysis.ListSubjectsByExam(ctx, &v1.ListSubjectsByExamRequest{
			ExamId:    getInt64Arg(args, "exam_id"),
			PageIndex: getInt32Arg(args, "page_index"),
			PageSize:  getInt32Arg(args, "page_size"),
		})
	case "get_subject_summary":
		return b.analysis.GetSubjectSummary(ctx, &v1.GetSubjectSummaryRequest{
			ExamId:    getInt64Arg(args, "exam_id"),
			Scope:     getStringArg(args, "scope"),
			SubjectId: getInt64Arg(args, "subject_id"),
		})
	case "get_class_summary":
		return b.analysis.GetClassSummary(ctx, &v1.GetClassSummaryRequest{
			ExamId:    getInt64Arg(args, "exam_id"),
			Scope:     getStringArg(args, "scope"),
			SubjectId: getInt64Arg(args, "subject_id"),
		})
	case "get_rating_distribution":
		return b.analysis.GetRatingDistribution(ctx, &v1.GetRatingDistributionRequest{
			ExamId:             getInt64Arg(args, "exam_id"),
			Scope:              getStringArg(args, "scope"),
			SubjectId:          getInt64Arg(args, "subject_id"),
			ExcellentThreshold: getFloat64Arg(args, "excellent_threshold"),
			GoodThreshold:      getFloat64Arg(args, "good_threshold"),
			PassThreshold:      getFloat64Arg(args, "pass_threshold"),
		})
	default:
		return nil, fmt.Errorf("unsupported tool: %s", name)
	}
}

func getStringArg(args map[string]any, key string) string {
	value, _ := args[key]
	str, _ := value.(string)
	return str
}

func getInt32Arg(args map[string]any, key string) int32 {
	return int32(getInt64Arg(args, key))
}

func getInt64Arg(args map[string]any, key string) int64 {
	value, ok := args[key]
	if !ok {
		return 0
	}

	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}

func getFloat64Arg(args map[string]any, key string) float64 {
	value, ok := args[key]
	if !ok {
		return 0
	}

	switch v := value.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	default:
		return 0
	}
}
