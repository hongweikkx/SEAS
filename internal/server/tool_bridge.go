package server

import (
	"context"
	"fmt"
	"strings"

	v1 "seas/api/seas/v1"
	"seas/internal/service"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

type toolResultRecord struct {
	Name      string
	Arguments map[string]any
	Result    any
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

func (b *AnalysisToolBridge) EinoTools() ([]tool.BaseTool, error) {
	defs := []tool.BaseTool{
		newListExamsTool(b),
		newListSubjectsByExamTool(b),
		newGetSubjectSummaryTool(b),
		newGetClassSummaryTool(b),
		newGetRatingDistributionTool(b),
	}
	return defs, nil
}

func (b *AnalysisToolBridge) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	switch name {
	case "list_exams":
		return b.analysis.ListExams(ctx, &v1.ListExamsRequest{
			PageIndex: normalizePageIndex(getInt32Arg(args, "page_index")),
			PageSize:  normalizePageSize(getInt32Arg(args, "page_size")),
		})
	case "list_subjects_by_exam":
		return b.analysis.ListSubjectsByExam(ctx, &v1.ListSubjectsByExamRequest{
			ExamId:    getInt64Arg(args, "exam_id"),
			PageIndex: normalizePageIndex(getInt32Arg(args, "page_index")),
			PageSize:  normalizePageSize(getInt32Arg(args, "page_size")),
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
			ExcellentThreshold: normalizeThreshold(getFloat64Arg(args, "excellent_threshold"), 90),
			GoodThreshold:      normalizeThreshold(getFloat64Arg(args, "good_threshold"), 70),
			PassThreshold:      normalizeThreshold(getFloat64Arg(args, "pass_threshold"), 60),
		})
	default:
		return nil, fmt.Errorf("unsupported tool: %s", name)
	}
}

type listExamsArgs struct {
	PageIndex int32 `json:"page_index,omitempty"`
	PageSize  int32 `json:"page_size,omitempty"`
}

type listSubjectsByExamArgs struct {
	ExamID    int64 `json:"exam_id"`
	PageIndex int32 `json:"page_index,omitempty"`
	PageSize  int32 `json:"page_size,omitempty"`
}

type subjectSummaryArgs struct {
	ExamID    int64  `json:"exam_id"`
	Scope     string `json:"scope"`
	SubjectID int64  `json:"subject_id,omitempty"`
}

type classSummaryArgs struct {
	ExamID    int64  `json:"exam_id"`
	Scope     string `json:"scope"`
	SubjectID int64  `json:"subject_id,omitempty"`
}

type ratingDistributionArgs struct {
	ExamID             int64   `json:"exam_id"`
	Scope              string  `json:"scope"`
	SubjectID          int64   `json:"subject_id,omitempty"`
	ExcellentThreshold float64 `json:"excellent_threshold,omitempty"`
	GoodThreshold      float64 `json:"good_threshold,omitempty"`
	PassThreshold      float64 `json:"pass_threshold,omitempty"`
}

func newListExamsTool(b *AnalysisToolBridge) tool.BaseTool {
	info := &schema.ToolInfo{
		Name: "list_exams",
		Desc: "Get a list of exams with pagination support",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"page_index": {Type: schema.Integer, Desc: "Page number, starts from 1"},
			"page_size":  {Type: schema.Integer, Desc: "Page size"},
		}),
	}

	return toolutils.NewEnhancedTool(info, func(ctx context.Context, input listExamsArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.ListExams(ctx, &v1.ListExamsRequest{
			PageIndex: normalizePageIndex(input.PageIndex),
			PageSize:  normalizePageSize(input.PageSize),
		})
		if err != nil {
			appendToolResultRecord(ctx, toolResultRecord{
				Name:      info.Name,
				Arguments: map[string]any{"page_index": input.PageIndex, "page_size": input.PageSize},
				Result:    map[string]any{"error": err.Error()},
			})
			return toolErrorResult(info.Name, err), nil
		}

		appendToolResultRecord(ctx, toolResultRecord{
			Name:      info.Name,
			Arguments: map[string]any{"page_index": input.PageIndex, "page_size": input.PageSize},
			Result:    reply,
		})
		return protoToolResult(reply)
	})
}

func newListSubjectsByExamTool(b *AnalysisToolBridge) tool.BaseTool {
	info := &schema.ToolInfo{
		Name: "list_subjects_by_exam",
		Desc: "Get a list of subjects associated with a specific exam",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"exam_id":    {Type: schema.Integer, Desc: "Exam ID, required", Required: true},
			"page_index": {Type: schema.Integer, Desc: "Page number, starts from 1"},
			"page_size":  {Type: schema.Integer, Desc: "Page size"},
		}),
	}

	return toolutils.NewEnhancedTool(info, func(ctx context.Context, input listSubjectsByExamArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.ListSubjectsByExam(ctx, &v1.ListSubjectsByExamRequest{
			ExamId:    input.ExamID,
			PageIndex: normalizePageIndex(input.PageIndex),
			PageSize:  normalizePageSize(input.PageSize),
		})
		if err != nil {
			appendToolResultRecord(ctx, toolResultRecord{
				Name:      info.Name,
				Arguments: map[string]any{"exam_id": input.ExamID, "page_index": input.PageIndex, "page_size": input.PageSize},
				Result:    map[string]any{"error": err.Error()},
			})
			return toolErrorResult(info.Name, err), nil
		}

		appendToolResultRecord(ctx, toolResultRecord{
			Name:      info.Name,
			Arguments: map[string]any{"exam_id": input.ExamID, "page_index": input.PageIndex, "page_size": input.PageSize},
			Result:    reply,
		})
		return protoToolResult(reply)
	})
}

func newGetSubjectSummaryTool(b *AnalysisToolBridge) tool.BaseTool {
	info := &schema.ToolInfo{
		Name: "get_subject_summary",
		Desc: "Get summary statistics for subjects in an exam",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"exam_id":    {Type: schema.Integer, Desc: "Exam ID, required", Required: true},
			"scope":      {Type: schema.String, Desc: "all_subjects or single_subject", Required: true, Enum: []string{"all_subjects", "single_subject"}},
			"subject_id": {Type: schema.Integer, Desc: "Subject ID when scope is single_subject"},
		}),
	}

	return toolutils.NewEnhancedTool(info, func(ctx context.Context, input subjectSummaryArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.GetSubjectSummary(ctx, &v1.GetSubjectSummaryRequest{
			ExamId:    input.ExamID,
			Scope:     input.Scope,
			SubjectId: input.SubjectID,
		})
		if err != nil {
			appendToolResultRecord(ctx, toolResultRecord{
				Name:      info.Name,
				Arguments: map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID},
				Result:    map[string]any{"error": err.Error()},
			})
			return toolErrorResult(info.Name, err), nil
		}

		appendToolResultRecord(ctx, toolResultRecord{
			Name:      info.Name,
			Arguments: map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID},
			Result:    reply,
		})
		return protoToolResult(reply)
	})
}

func newGetClassSummaryTool(b *AnalysisToolBridge) tool.BaseTool {
	info := &schema.ToolInfo{
		Name: "get_class_summary",
		Desc: "Get summary statistics for classes in an exam/subject",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"exam_id":    {Type: schema.Integer, Desc: "Exam ID, required", Required: true},
			"scope":      {Type: schema.String, Desc: "all_subjects or single_subject", Required: true, Enum: []string{"all_subjects", "single_subject"}},
			"subject_id": {Type: schema.Integer, Desc: "Subject ID when scope is single_subject"},
		}),
	}

	return toolutils.NewEnhancedTool(info, func(ctx context.Context, input classSummaryArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.GetClassSummary(ctx, &v1.GetClassSummaryRequest{
			ExamId:    input.ExamID,
			Scope:     input.Scope,
			SubjectId: input.SubjectID,
		})
		if err != nil {
			appendToolResultRecord(ctx, toolResultRecord{
				Name:      info.Name,
				Arguments: map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID},
				Result:    map[string]any{"error": err.Error()},
			})
			return toolErrorResult(info.Name, err), nil
		}

		appendToolResultRecord(ctx, toolResultRecord{
			Name:      info.Name,
			Arguments: map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID},
			Result:    reply,
		})
		return protoToolResult(reply)
	})
}

func newGetRatingDistributionTool(b *AnalysisToolBridge) tool.BaseTool {
	info := &schema.ToolInfo{
		Name: "get_rating_distribution",
		Desc: "Get four-rate (excellent, good, pass, fail) analysis for an exam/subject",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"exam_id":             {Type: schema.Integer, Desc: "Exam ID, required", Required: true},
			"scope":               {Type: schema.String, Desc: "all_subjects or single_subject", Required: true, Enum: []string{"all_subjects", "single_subject"}},
			"subject_id":          {Type: schema.Integer, Desc: "Subject ID when scope is single_subject"},
			"excellent_threshold": {Type: schema.Number, Desc: "Excellent score threshold"},
			"good_threshold":      {Type: schema.Number, Desc: "Good score threshold"},
			"pass_threshold":      {Type: schema.Number, Desc: "Pass score threshold"},
		}),
	}

	return toolutils.NewEnhancedTool(info, func(ctx context.Context, input ratingDistributionArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.GetRatingDistribution(ctx, &v1.GetRatingDistributionRequest{
			ExamId:             input.ExamID,
			Scope:              input.Scope,
			SubjectId:          input.SubjectID,
			ExcellentThreshold: normalizeThreshold(input.ExcellentThreshold, 90),
			GoodThreshold:      normalizeThreshold(input.GoodThreshold, 70),
			PassThreshold:      normalizeThreshold(input.PassThreshold, 60),
		})
		if err != nil {
			appendToolResultRecord(ctx, toolResultRecord{
				Name:      info.Name,
				Arguments: map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID, "excellent_threshold": input.ExcellentThreshold, "good_threshold": input.GoodThreshold, "pass_threshold": input.PassThreshold},
				Result:    map[string]any{"error": err.Error()},
			})
			return toolErrorResult(info.Name, err), nil
		}

		appendToolResultRecord(ctx, toolResultRecord{
			Name:      info.Name,
			Arguments: map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID, "excellent_threshold": input.ExcellentThreshold, "good_threshold": input.GoodThreshold, "pass_threshold": input.PassThreshold},
			Result:    reply,
		})
		return protoToolResult(reply)
	})
}

func protoToolResult(msg proto.Message) (*schema.ToolResult, error) {
	payload, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return &schema.ToolResult{
		Parts: []schema.ToolOutputPart{
			{
				Type: schema.ToolPartTypeText,
				Text: string(payload),
			},
		},
	}, nil
}

func toolErrorResult(toolName string, err error) *schema.ToolResult {
	return &schema.ToolResult{
		Parts: []schema.ToolOutputPart{
			{
				Type: schema.ToolPartTypeText,
				Text: fmt.Sprintf("tool %s failed: %s", toolName, err.Error()),
			},
		},
	}
}

func getStringArg(args map[string]any, key string) string {
	value, _ := args[key]
	str, _ := value.(string)
	return strings.TrimSpace(str)
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

func normalizePageIndex(v int32) int32 {
	if v <= 0 {
		return 1
	}
	return v
}

func normalizePageSize(v int32) int32 {
	if v <= 0 {
		return 20
	}
	return v
}

func normalizeThreshold(v float64, defaultValue float64) float64 {
	if v == 0 {
		return defaultValue
	}
	return v
}
