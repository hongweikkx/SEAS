package server

import (
	"context"
	"net/http"
	"time"

	"seas/internal/service"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServer struct {
	*mcp.Server
	tools *AnalysisToolBridge
}

func NewMCPServer(analysis *service.AnalysisService) *MCPServer {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "seas-analysis",
		Version: "1.0.0",
	}, nil)

	srv := &MCPServer{
		Server: s,
		tools:  NewAnalysisToolBridge(analysis),
	}

	srv.registerTools()
	return srv
}

func (s *MCPServer) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.Server
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 10 * time.Minute,
	})
}

func (s *MCPServer) registerTools() {
	// List Exams
	mcp.AddTool(s.Server, &mcp.Tool{
		Name:        "list_exams",
		Description: "Get a list of exams with pagination support",
	}, s.listExams)

	// List Subjects By Exam
	mcp.AddTool(s.Server, &mcp.Tool{
		Name:        "list_subjects_by_exam",
		Description: "Get a list of subjects associated with a specific exam",
	}, s.listSubjectsByExam)

	// Get Subject Summary
	mcp.AddTool(s.Server, &mcp.Tool{
		Name:        "get_subject_summary",
		Description: "Get summary statistics for subjects in an exam",
	}, s.getSubjectSummary)

	// Get Class Summary
	mcp.AddTool(s.Server, &mcp.Tool{
		Name:        "get_class_summary",
		Description: "Get summary statistics for classes in an exam/subject",
	}, s.getClassSummary)

	// Get Rating Distribution
	mcp.AddTool(s.Server, &mcp.Tool{
		Name:        "get_rating_distribution",
		Description: "Get four-rate (excellent, good, pass, fail) analysis for an exam/subject",
	}, s.getRatingDistribution)
}

func (s *MCPServer) listExams(ctx context.Context, req *mcp.CallToolRequest, args struct {
	PageIndex int32 `json:"page_index,omitzero" jsonschema:"Page number (starts from 1, default: 1)"`
	PageSize  int32 `json:"page_size,omitzero" jsonschema:"Number of items per page (default: 20)"`
}) (*mcp.CallToolResult, any, error) {
	resp, err := s.tools.Call(ctx, "list_exams", map[string]any{
		"page_index": args.PageIndex,
		"page_size":  args.PageSize,
	})
	return nil, resp, err
}

func (s *MCPServer) listSubjectsByExam(ctx context.Context, req *mcp.CallToolRequest, args struct {
	ExamID    int64 `json:"exam_id" jsonschema:"ID of the exam"`
	PageIndex int32 `json:"page_index,omitzero" jsonschema:"Page number (starts from 1, default: 1)"`
	PageSize  int32 `json:"page_size,omitzero" jsonschema:"Number of items per page (default: 20)"`
}) (*mcp.CallToolResult, any, error) {
	resp, err := s.tools.Call(ctx, "list_subjects_by_exam", map[string]any{
		"exam_id":    args.ExamID,
		"page_index": args.PageIndex,
		"page_size":  args.PageSize,
	})
	return nil, resp, err
}

func (s *MCPServer) getSubjectSummary(ctx context.Context, req *mcp.CallToolRequest, args struct {
	ExamID    int64  `json:"exam_id" jsonschema:"ID of the exam"`
	Scope     string `json:"scope" jsonschema:"Scope of analysis: 'all_subjects' or 'single_subject'"`
	SubjectID int64  `json:"subject_id,omitzero" jsonschema:"ID of the subject (required if scope is single_subject)"`
}) (*mcp.CallToolResult, any, error) {
	resp, err := s.tools.Call(ctx, "get_subject_summary", map[string]any{
		"exam_id":    args.ExamID,
		"scope":      args.Scope,
		"subject_id": args.SubjectID,
	})
	return nil, resp, err
}

func (s *MCPServer) getClassSummary(ctx context.Context, req *mcp.CallToolRequest, args struct {
	ExamID    int64  `json:"exam_id" jsonschema:"ID of the exam"`
	Scope     string `json:"scope" jsonschema:"Scope of analysis: 'all_subjects' or 'single_subject'"`
	SubjectID int64  `json:"subject_id,omitzero" jsonschema:"ID of the subject (required if scope is single_subject)"`
}) (*mcp.CallToolResult, any, error) {
	resp, err := s.tools.Call(ctx, "get_class_summary", map[string]any{
		"exam_id":    args.ExamID,
		"scope":      args.Scope,
		"subject_id": args.SubjectID,
	})
	return nil, resp, err
}

func (s *MCPServer) getRatingDistribution(ctx context.Context, req *mcp.CallToolRequest, args struct {
	ExamID             int64   `json:"exam_id" jsonschema:"ID of the exam"`
	Scope              string  `json:"scope" jsonschema:"Scope of analysis: 'all_subjects' or 'single_subject'"`
	SubjectID          int64   `json:"subject_id,omitzero" jsonschema:"ID of the subject (required if scope is single_subject)"`
	ExcellentThreshold float64 `json:"excellent_threshold,omitzero" jsonschema:"Threshold for excellent rating (default 90)"`
	GoodThreshold      float64 `json:"good_threshold,omitzero" jsonschema:"Threshold for good rating (default 70)"`
	PassThreshold      float64 `json:"pass_threshold,omitzero" jsonschema:"Threshold for pass rating (default 60)"`
}) (*mcp.CallToolResult, any, error) {
	resp, err := s.tools.Call(ctx, "get_rating_distribution", map[string]any{
		"exam_id":             args.ExamID,
		"scope":               args.Scope,
		"subject_id":          args.SubjectID,
		"excellent_threshold": args.ExcellentThreshold,
		"good_threshold":      args.GoodThreshold,
		"pass_threshold":      args.PassThreshold,
	})
	return nil, resp, err
}
