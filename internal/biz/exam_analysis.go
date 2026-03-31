package biz

import (
	"context"
	"math"
)

// ExamAnalysisUseCase 考试分析业务逻辑
type ExamAnalysisUseCase struct {
	examRepo    ExamRepo
	subjectRepo SubjectRepo
	scoreRepo   ScoreRepo
}

// NewExamAnalysisUseCase 创建考试分析用例
func NewExamAnalysisUseCase(examRepo ExamRepo, subjectRepo SubjectRepo, scoreRepo ScoreRepo) *ExamAnalysisUseCase {
	return &ExamAnalysisUseCase{
		examRepo:    examRepo,
		subjectRepo: subjectRepo,
		scoreRepo:   scoreRepo,
	}
}

// ListExams 获取考试列表
func (uc *ExamAnalysisUseCase) ListExams(ctx context.Context, pageIndex, pageSize int32) ([]*Exam, int64, error) {
	return uc.examRepo.ListAll(ctx, pageIndex, pageSize)
}

// ListSubjectsByExam 获取考试关联的学科列表
func (uc *ExamAnalysisUseCase) ListSubjectsByExam(ctx context.Context, examID int64, pageIndex, pageSize int32) ([]*Subject, int64, error) {
	return uc.subjectRepo.ListByExamID(ctx, examID, pageIndex, pageSize)
}

// GetSubjectSummary 获取学科情况汇总
func (uc *ExamAnalysisUseCase) GetSubjectSummary(ctx context.Context, examID, subjectID int64) (*SubjectSummaryStats, error) {
	return uc.scoreRepo.GetSubjectSummary(ctx, examID, subjectID)
}

// GetClassSummary 获取班级情况汇总
func (uc *ExamAnalysisUseCase) GetClassSummary(ctx context.Context, examID, subjectID int64) (*ClassSummaryStats, error) {
	stats, err := uc.scoreRepo.GetClassSummary(ctx, examID, subjectID)
	if err != nil {
		return nil, err
	}

	// 计算离均差：班级平均分 - 全年级平均分
	if stats.OverallGrade != nil && stats.OverallGrade.AvgScore > 0 {
		overallAvg := stats.OverallGrade.AvgScore
		for _, class := range stats.ClassDetails {
			class.ScoreDeviation = roundTo2Decimal(class.AvgScore - overallAvg)
		}
		// 全年级离均差固定为0
		stats.OverallGrade.ScoreDeviation = 0
	}

	return stats, nil
}

// GetRatingDistribution 获取四率分析
func (uc *ExamAnalysisUseCase) GetRatingDistribution(ctx context.Context, examID, subjectID int64, excellentThreshold, goodThreshold, passThreshold float64) (*RatingDistributionStats, error) {
	// 使用默认值如果参数为0
	if excellentThreshold == 0 {
		excellentThreshold = 90
	}
	if goodThreshold == 0 {
		goodThreshold = 70
	}
	if passThreshold == 0 {
		passThreshold = 60
	}

	stats, err := uc.scoreRepo.GetRatingDistribution(ctx, examID, subjectID, excellentThreshold, goodThreshold, passThreshold)
	if err != nil {
		return nil, err
	}

	// 设置配置信息
	stats.Config = &RatingConfigStats{
		ExcellentThreshold: excellentThreshold,
		GoodThreshold:      goodThreshold,
		PassThreshold:      passThreshold,
	}

	// 计算百分比
	if stats.OverallGrade != nil {
		uc.calculateRatingPercentages(stats.OverallGrade)
	}
	for _, class := range stats.ClassDetails {
		uc.calculateRatingPercentages(class)
	}

	return stats, nil
}

// calculateRatingPercentages 计算四率百分比
func (uc *ExamAnalysisUseCase) calculateRatingPercentages(classRating *ClassRatingStats) {
	if classRating.TotalStudents == 0 {
		return
	}

	total := float64(classRating.TotalStudents)
	classRating.Excellent.Percentage = roundTo2Decimal(float64(classRating.Excellent.Count) / total * 100)
	classRating.Good.Percentage = roundTo2Decimal(float64(classRating.Good.Count) / total * 100)
	classRating.Pass.Percentage = roundTo2Decimal(float64(classRating.Pass.Count) / total * 100)
	classRating.Fail.Percentage = roundTo2Decimal(float64(classRating.Fail.Count) / total * 100)
}

// roundTo2Decimal 四舍五入到2位小数
func roundTo2Decimal(value float64) float64 {
	return math.Round(value*100) / 100
}

// GetExamName 获取考试名称
func (uc *ExamAnalysisUseCase) GetExamName(ctx context.Context, examID int64) (string, error) {
	return uc.examRepo.GetExamName(ctx, examID)
}
