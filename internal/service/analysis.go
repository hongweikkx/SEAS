package service

import (
	"context"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"

	pb "seas/api/seas/v1"
)

type AnalysisService struct {
	pb.UnimplementedAnalysisServer
	uc *biz.AnalysisUseCase
}

func NewAnalysisService(uc *biz.AnalysisUseCase) *AnalysisService {
	return &AnalysisService{uc: uc}
}

func (s *AnalysisService) Analyze(ctx context.Context, req *pb.AnalyzeRequest) (*pb.AnalyzeReply, error) {
	log.Context(ctx).Infof("Received AnalyzeRequest: %v", req)
	summary, suggestions, err := s.uc.Analyze(ctx, req.GetStudentNlInput())
	if err != nil {
		return nil, err
	}
	return &pb.AnalyzeReply{
		ResultSummary: summary,
		Suggestions:   suggestions,
	}, nil
}
