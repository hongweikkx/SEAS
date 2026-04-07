package server

import (
	"context"
	"fmt"

	v1 "seas/api/seas/v1"
	"seas/internal/service"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

const (
	toolNameListExams             = "list_exams"
	toolNameListSubjectsByExam    = "list_subjects_by_exam"
	toolNameGetSubjectSummary     = "get_subject_summary"
	toolNameGetClassSummary       = "get_class_summary"
	toolNameGetRatingDistribution = "get_rating_distribution"
)

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

func recordToolResult(ctx context.Context, toolName string, args map[string]any, result proto.Message, err error) (*schema.ToolResult, error) {
	if err != nil {
		appendToolResultRecord(ctx, toolResultRecord{
			Name:      toolName,
			Arguments: args,
			Result:    map[string]any{"error": err.Error()},
		})
		return toolErrorResult(toolName, err), nil
	}

	appendToolResultRecord(ctx, toolResultRecord{
		Name:      toolName,
		Arguments: args,
		Result:    result,
	})
	return protoToolResult(result)
}

type listExamsArgs struct {
	PageIndex int32 `json:"page_index,omitempty" jsonschema_description:"Page number, starts from 1"`
	PageSize  int32 `json:"page_size,omitempty" jsonschema_description:"Page size"`
}

type listSubjectsByExamArgs struct {
	ExamID    int64 `json:"exam_id" jsonschema:"required" jsonschema_description:"Exam ID"`
	PageIndex int32 `json:"page_index,omitempty" jsonschema_description:"Page number, starts from 1"`
	PageSize  int32 `json:"page_size,omitempty" jsonschema_description:"Page size"`
}

type subjectSummaryArgs struct {
	ExamID    int64  `json:"exam_id" jsonschema:"required" jsonschema_description:"Exam ID"`
	Scope     string `json:"scope" jsonschema:"required,enum=all_subjects,enum=single_subject" jsonschema_description:"Scope of the summary"`
	SubjectID int64  `json:"subject_id,omitempty" jsonschema_description:"Subject ID when scope is single_subject"`
}

type classSummaryArgs struct {
	ExamID    int64  `json:"exam_id" jsonschema:"required" jsonschema_description:"Exam ID"`
	Scope     string `json:"scope" jsonschema:"required,enum=all_subjects,enum=single_subject" jsonschema_description:"Scope of the summary"`
	SubjectID int64  `json:"subject_id,omitempty" jsonschema_description:"Subject ID when scope is single_subject"`
}

type ratingDistributionArgs struct {
	ExamID             int64   `json:"exam_id" jsonschema:"required" jsonschema_description:"Exam ID"`
	Scope              string  `json:"scope" jsonschema:"required,enum=all_subjects,enum=single_subject" jsonschema_description:"Scope of the distribution"`
	SubjectID          int64   `json:"subject_id,omitempty" jsonschema_description:"Subject ID when scope is single_subject"`
	ExcellentThreshold float64 `json:"excellent_threshold,omitempty" jsonschema_description:"Score threshold for excellent, default: 90"`
	GoodThreshold      float64 `json:"good_threshold,omitempty" jsonschema_description:"Score threshold for good, default: 70"`
	PassThreshold      float64 `json:"pass_threshold,omitempty" jsonschema_description:"Score threshold for pass, default: 60"`
}

func newListExamsTool(b *AnalysisToolBridge) tool.BaseTool {
	t, err := toolutils.InferEnhancedTool(toolNameListExams, "Get a list of exams with pagination support", func(ctx context.Context, input listExamsArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.ListExams(ctx, &v1.ListExamsRequest{
			PageIndex: normalizePageIndex(input.PageIndex),
			PageSize:  normalizePageSize(input.PageSize),
		})
		return recordToolResult(ctx, toolNameListExams, map[string]any{"page_index": input.PageIndex, "page_size": input.PageSize}, reply, err)
	})
	if err != nil {
		panic(err)
	}
	return t
}

func newListSubjectsByExamTool(b *AnalysisToolBridge) tool.BaseTool {
	t, err := toolutils.InferEnhancedTool(toolNameListSubjectsByExam, "Get a list of subjects associated with a specific exam", func(ctx context.Context, input listSubjectsByExamArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.ListSubjectsByExam(ctx, &v1.ListSubjectsByExamRequest{
			ExamId:    input.ExamID,
			PageIndex: normalizePageIndex(input.PageIndex),
			PageSize:  normalizePageSize(input.PageSize),
		})
		return recordToolResult(ctx, toolNameListSubjectsByExam, map[string]any{"exam_id": input.ExamID, "page_index": input.PageIndex, "page_size": input.PageSize}, reply, err)
	})
	if err != nil {
		panic(err)
	}
	return t
}

func newGetSubjectSummaryTool(b *AnalysisToolBridge) tool.BaseTool {
	t, err := toolutils.InferEnhancedTool(toolNameGetSubjectSummary, "Get summary statistics for subjects in an exam", func(ctx context.Context, input subjectSummaryArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.GetSubjectSummary(ctx, &v1.GetSubjectSummaryRequest{
			ExamId:    input.ExamID,
			Scope:     input.Scope,
			SubjectId: input.SubjectID,
		})
		return recordToolResult(ctx, toolNameGetSubjectSummary, map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID}, reply, err)
	})
	if err != nil {
		panic(err)
	}
	return t
}

func newGetClassSummaryTool(b *AnalysisToolBridge) tool.BaseTool {
	t, err := toolutils.InferEnhancedTool(toolNameGetClassSummary, "Get summary statistics for classes in an exam/subject", func(ctx context.Context, input classSummaryArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.GetClassSummary(ctx, &v1.GetClassSummaryRequest{
			ExamId:    input.ExamID,
			Scope:     input.Scope,
			SubjectId: input.SubjectID,
		})
		return recordToolResult(ctx, toolNameGetClassSummary, map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID}, reply, err)
	})
	if err != nil {
		panic(err)
	}
	return t
}

func newGetRatingDistributionTool(b *AnalysisToolBridge) tool.BaseTool {
	t, err := toolutils.InferEnhancedTool(toolNameGetRatingDistribution, "Get four-rate (excellent, good, pass, fail) analysis for an exam/subject", func(ctx context.Context, input ratingDistributionArgs) (*schema.ToolResult, error) {
		reply, err := b.analysis.GetRatingDistribution(ctx, &v1.GetRatingDistributionRequest{
			ExamId:             input.ExamID,
			Scope:              input.Scope,
			SubjectId:          input.SubjectID,
			ExcellentThreshold: normalizeThreshold(input.ExcellentThreshold, 90),
			GoodThreshold:      normalizeThreshold(input.GoodThreshold, 70),
			PassThreshold:      normalizeThreshold(input.PassThreshold, 60),
		})
		return recordToolResult(ctx, toolNameGetRatingDistribution, map[string]any{"exam_id": input.ExamID, "scope": input.Scope, "subject_id": input.SubjectID, "excellent_threshold": input.ExcellentThreshold, "good_threshold": input.GoodThreshold, "pass_threshold": input.PassThreshold}, reply, err)
	})
	if err != nil {
		panic(err)
	}
	return t
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
