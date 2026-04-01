package service

import (
	"context"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"

	pb "seas/api/seas/v1"
)

type AnalysisService struct {
	pb.UnimplementedAnalysisServer
	analysisUC     *biz.AnalysisUseCase
	examAnalysisUC *biz.ExamAnalysisUseCase
}

func NewAnalysisService(analysisUC *biz.AnalysisUseCase, examAnalysisUC *biz.ExamAnalysisUseCase) *AnalysisService {
	return &AnalysisService{
		analysisUC:     analysisUC,
		examAnalysisUC: examAnalysisUC,
	}
}

// ListExams 获取考试列表
func (s *AnalysisService) ListExams(ctx context.Context, req *pb.ListExamsRequest) (*pb.ListExamsReply, error) {
	log.Context(ctx).Infof("Received ListExamsRequest: %v", req)

	exams, total, err := s.examAnalysisUC.ListExams(ctx, req.GetPageIndex(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	reply := &pb.ListExamsReply{
		TotalCount: total,
		PageIndex:  req.GetPageIndex(),
		PageSize:   req.GetPageSize(),
	}

	reply.Exams = make([]*pb.ExamInfo, len(exams))
	for i, exam := range exams {
		reply.Exams[i] = &pb.ExamInfo{
			Id:        exam.ID,
			Name:      exam.Name,
			ExamDate:  exam.ExamDate.Format("2006-01-02T15:04:05Z"),
			CreatedAt: exam.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return reply, nil
}

// ListSubjectsByExam 获取考试关联的学科列表
func (s *AnalysisService) ListSubjectsByExam(ctx context.Context, req *pb.ListSubjectsByExamRequest) (*pb.ListSubjectsByExamReply, error) {
	log.Context(ctx).Infof("Received ListSubjectsByExamRequest: %v", req)

	subjects, total, err := s.examAnalysisUC.ListSubjectsByExam(ctx, req.GetExamId(), req.GetPageIndex(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	reply := &pb.ListSubjectsByExamReply{
		ExamId:     req.GetExamId(),
		TotalCount: total,
		PageIndex:  req.GetPageIndex(),
		PageSize:   req.GetPageSize(),
	}

	reply.Subjects = make([]*pb.SubjectBasicInfo, len(subjects))
	for i, subject := range subjects {
		// 暂时使用默认满分，后续需要从 exam_subjects 表获取
		reply.Subjects[i] = &pb.SubjectBasicInfo{
			Id:        subject.ID,
			Name:      subject.Name,
			FullScore: 100, // 默认值
		}
	}

	return reply, nil
}

// GetSubjectSummary 获取学科情况汇总
func (s *AnalysisService) GetSubjectSummary(ctx context.Context, req *pb.GetSubjectSummaryRequest) (*pb.GetSubjectSummaryReply, error) {
	log.Context(ctx).Infof("Received GetSubjectSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetSubjectSummary(ctx, req.GetExamId(), req.GetSubjectId())
	if err != nil {
		return nil, err
	}

	// 获取考试名称
	examName, err := s.examAnalysisUC.GetExamName(ctx, req.GetExamId())
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试" // 默认值
	}

	reply := &pb.GetSubjectSummaryReply{
		ExamId:            req.GetExamId(),
		ExamName:          examName,
		Scope:             req.GetScope(),
		TotalParticipants: stats.TotalParticipants,
		SubjectsInvolved:  stats.SubjectsInvolved,
		ClassesInvolved:   stats.ClassesInvolved,
	}

	reply.Subjects = make([]*pb.SubjectSummaryItem, len(stats.Subjects))
	for i, subject := range stats.Subjects {
		reply.Subjects[i] = &pb.SubjectSummaryItem{
			Id:           subject.ID,
			Name:         subject.Name,
			FullScore:    subject.FullScore,
			AvgScore:     subject.AvgScore,
			HighestScore: subject.HighestScore,
			LowestScore:  subject.LowestScore,
			Difficulty:   subject.Difficulty,
			StudentCount: subject.StudentCount,
		}
	}

	return reply, nil
}

// GetClassSummary 获取班级情况汇总
func (s *AnalysisService) GetClassSummary(ctx context.Context, req *pb.GetClassSummaryRequest) (*pb.GetClassSummaryReply, error) {
	log.Context(ctx).Infof("Received GetClassSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetClassSummary(ctx, req.GetExamId(), req.GetSubjectId())
	if err != nil {
		return nil, err
	}

	// 获取考试名称
	examName, err := s.examAnalysisUC.GetExamName(ctx, req.GetExamId())
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试" // 默认值
	}

	reply := &pb.GetClassSummaryReply{
		ExamId:            req.GetExamId(),
		ExamName:          examName,
		Scope:             req.GetScope(),
		TotalParticipants: stats.TotalParticipants,
	}

	if stats.OverallGrade != nil {
		reply.OverallGrade = &pb.ClassSummaryItem{
			ClassId:        stats.OverallGrade.ClassID,
			ClassName:      stats.OverallGrade.ClassName,
			TotalStudents:  stats.OverallGrade.TotalStudents,
			AvgScore:       stats.OverallGrade.AvgScore,
			HighestScore:   stats.OverallGrade.HighestScore,
			LowestScore:    stats.OverallGrade.LowestScore,
			ScoreDeviation: stats.OverallGrade.ScoreDeviation,
			Difficulty:     stats.OverallGrade.Difficulty,
			StdDev:         stats.OverallGrade.StdDev,
		}
	}

	reply.ClassDetails = make([]*pb.ClassSummaryItem, len(stats.ClassDetails))
	for i, class := range stats.ClassDetails {
		reply.ClassDetails[i] = &pb.ClassSummaryItem{
			ClassId:        class.ClassID,
			ClassName:      class.ClassName,
			TotalStudents:  class.TotalStudents,
			AvgScore:       class.AvgScore,
			HighestScore:   class.HighestScore,
			LowestScore:    class.LowestScore,
			ScoreDeviation: class.ScoreDeviation,
			Difficulty:     class.Difficulty,
			StdDev:         class.StdDev,
		}
	}

	return reply, nil
}

// GetRatingDistribution 获取四率分析
func (s *AnalysisService) GetRatingDistribution(ctx context.Context, req *pb.GetRatingDistributionRequest) (*pb.GetRatingDistributionReply, error) {
	log.Context(ctx).Infof("Received GetRatingDistributionRequest: %v", req)

	// 使用默认值或请求中的配置
	excellentThreshold := req.GetExcellentThreshold()
	goodThreshold := req.GetGoodThreshold()
	passThreshold := req.GetPassThreshold()

	if excellentThreshold == 0 {
		excellentThreshold = 90
	}
	if goodThreshold == 0 {
		goodThreshold = 70
	}
	if passThreshold == 0 {
		passThreshold = 60
	}

	stats, err := s.examAnalysisUC.GetRatingDistribution(ctx, req.GetExamId(), req.GetSubjectId(), excellentThreshold, goodThreshold, passThreshold)
	if err != nil {
		return nil, err
	}

	// 获取考试名称
	examName, err := s.examAnalysisUC.GetExamName(ctx, req.GetExamId())
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试" // 默认值
	}

	reply := &pb.GetRatingDistributionReply{
		ExamId:            req.GetExamId(),
		ExamName:          examName,
		Scope:             req.GetScope(),
		TotalParticipants: stats.TotalParticipants,
		Config: &pb.RatingConfig{
			ExcellentThreshold: excellentThreshold,
			GoodThreshold:      goodThreshold,
			PassThreshold:      passThreshold,
		},
	}

	if stats.OverallGrade != nil {
		reply.OverallGrade = &pb.ClassRatingDistribution{
			ClassId:       stats.OverallGrade.ClassID,
			ClassName:     stats.OverallGrade.ClassName,
			TotalStudents: stats.OverallGrade.TotalStudents,
			AvgScore:      stats.OverallGrade.AvgScore,
			Excellent: &pb.RatingItem{
				Count:      stats.OverallGrade.Excellent.Count,
				Percentage: stats.OverallGrade.Excellent.Percentage,
			},
			Good: &pb.RatingItem{
				Count:      stats.OverallGrade.Good.Count,
				Percentage: stats.OverallGrade.Good.Percentage,
			},
			Pass: &pb.RatingItem{
				Count:      stats.OverallGrade.Pass.Count,
				Percentage: stats.OverallGrade.Pass.Percentage,
			},
			Fail: &pb.RatingItem{
				Count:      stats.OverallGrade.Fail.Count,
				Percentage: stats.OverallGrade.Fail.Percentage,
			},
		}
	}

	reply.ClassDetails = make([]*pb.ClassRatingDistribution, len(stats.ClassDetails))
	for i, class := range stats.ClassDetails {
		reply.ClassDetails[i] = &pb.ClassRatingDistribution{
			ClassId:       class.ClassID,
			ClassName:     class.ClassName,
			TotalStudents: class.TotalStudents,
			AvgScore:      class.AvgScore,
			Excellent: &pb.RatingItem{
				Count:      class.Excellent.Count,
				Percentage: class.Excellent.Percentage,
			},
			Good: &pb.RatingItem{
				Count:      class.Good.Count,
				Percentage: class.Good.Percentage,
			},
			Pass: &pb.RatingItem{
				Count:      class.Pass.Count,
				Percentage: class.Pass.Percentage,
			},
			Fail: &pb.RatingItem{
				Count:      class.Fail.Count,
				Percentage: class.Fail.Percentage,
			},
		}
	}

	return reply, nil
}
