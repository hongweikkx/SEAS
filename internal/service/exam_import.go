package service

import (
	"context"
	"io"
	"os"
	"strconv"
	"time"

	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	pb "seas/api/seas/v1"
)

type ExamImportService struct {
	pb.UnimplementedExamImportServer
	importUC *biz.ExamImportUseCase
	log      *log.Helper
}

func NewExamImportService(importUC *biz.ExamImportUseCase, logger log.Logger) *ExamImportService {
	return &ExamImportService{
		importUC: importUC,
		log:      log.NewHelper(logger),
	}
}

// CreateExam 创建考试
func (s *ExamImportService) CreateExam(ctx context.Context, req *pb.CreateExamRequest) (*pb.CreateExamReply, error) {
	examDate, err := time.Parse("2006-01-02", req.GetExamDate())
	if err != nil {
		return nil, err
	}

	examID, err := s.importUC.CreateExam(ctx, req.GetName(), examDate)
	if err != nil {
		return nil, err
	}

	return &pb.CreateExamReply{
		ExamId:   strconv.FormatInt(examID, 10),
		Name:     req.GetName(),
		ExamDate: req.GetExamDate(),
	}, nil
}

// ImportScores 导入成绩（gRPC 调用入口，实际文件上传由 HTTP handler 处理）
func (s *ExamImportService) ImportScores(ctx context.Context, req *pb.ImportScoresRequest) (*pb.ImportScoresReply, error) {
	return &pb.ImportScoresReply{}, nil
}

// ImportScoresFromMultipart 处理 multipart 文件上传（供 HTTP handler 调用）
func (s *ExamImportService) ImportScoresFromMultipart(ctx context.Context, examID int64, file io.Reader) (*pb.ImportScoresReply, error) {
	// 保存到临时文件
	tmpFile, err := os.CreateTemp("", "exam-import-*.xlsx")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		return nil, err
	}

	result, err := s.importUC.ImportScoresFromExcel(ctx, examID, tmpFile.Name())
	if err != nil {
		return nil, err
	}

	return &pb.ImportScoresReply{
		ExamId:           strconv.FormatInt(examID, 10),
		ImportedStudents: int32(result.ImportedStudents),
		ImportedSubjects: int32(result.ImportedSubjects),
		Mode:             result.Mode,
		Warnings:         result.Warnings,
	}, nil
}

// UpdateSubjectFullScores 更新考试各学科满分
func (s *ExamImportService) UpdateSubjectFullScores(ctx context.Context, req *pb.UpdateSubjectFullScoresRequest) (*pb.UpdateSubjectFullScoresReply, error) {
	examID, err := strconv.ParseInt(req.GetExamId(), 10, 64)
	if err != nil {
		return nil, err
	}

	fullScores := make(map[string]float64, len(req.GetFullScores()))
	for k, v := range req.GetFullScores() {
		fullScores[k] = v
	}

	if err := s.importUC.UpdateSubjectFullScores(ctx, examID, fullScores); err != nil {
		return nil, err
	}

	return &pb.UpdateSubjectFullScoresReply{}, nil
}
