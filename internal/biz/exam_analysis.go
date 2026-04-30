package biz

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
)

// ExamAnalysisUseCase 考试分析业务逻辑
type ExamAnalysisUseCase struct {
	examRepo      ExamRepo
	subjectRepo   SubjectRepo
	scoreRepo     ScoreRepo
	scoreItemRepo ScoreItemRepo
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
func (uc *ExamAnalysisUseCase) ListExams(ctx context.Context, pageIndex, pageSize int32, keyword string) ([]*Exam, int64, error) {
	return uc.examRepo.ListAll(ctx, pageIndex, pageSize, keyword)
}

// ListSubjectsByExam 获取考试关联的学科列表
func (uc *ExamAnalysisUseCase) ListSubjectsByExam(ctx context.Context, examID int64, pageIndex, pageSize int32) ([]*Subject, int64, error) {
	return uc.subjectRepo.ListByExamID(ctx, examID, pageIndex, pageSize)
}

// GetSubjectFullScore 获取考试中某学科的满分
func (uc *ExamAnalysisUseCase) GetSubjectFullScore(ctx context.Context, examID, subjectID int64) (float64, error) {
	return uc.subjectRepo.GetFullScoreByExamSubject(ctx, examID, subjectID)
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
func (uc *ExamAnalysisUseCase) GetRatingDistribution(ctx context.Context, examID, subjectID int64, excellentThreshold, goodThreshold, mediumThreshold, passThreshold, lowScoreThreshold float64) (*RatingDistributionStats, error) {
	// 使用默认值如果参数为0
	if excellentThreshold == 0 {
		excellentThreshold = 85
	}
	if goodThreshold == 0 {
		goodThreshold = 76
	}
	if mediumThreshold == 0 {
		mediumThreshold = 68
	}
	if passThreshold == 0 {
		passThreshold = 60
	}
	if lowScoreThreshold == 0 {
		lowScoreThreshold = 40
	}

	stats, err := uc.scoreRepo.GetRatingDistribution(ctx, examID, subjectID, excellentThreshold, goodThreshold, mediumThreshold, passThreshold, lowScoreThreshold)
	if err != nil {
		return nil, err
	}

	// 设置配置信息
	stats.Config = &RatingConfigStats{
		ExcellentThreshold: excellentThreshold,
		GoodThreshold:      goodThreshold,
		MediumThreshold:    mediumThreshold,
		PassThreshold:      passThreshold,
		LowScoreThreshold:  lowScoreThreshold,
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

// WithScoreItemRepo 为后续题目维度接口注入 score item repo。
func (uc *ExamAnalysisUseCase) WithScoreItemRepo(scoreItemRepo ScoreItemRepo) *ExamAnalysisUseCase {
	uc.scoreItemRepo = scoreItemRepo
	return uc
}

// GetClassSubjectSummary 获取班级学科下钻汇总
func (uc *ExamAnalysisUseCase) GetClassSubjectSummary(ctx context.Context, examID, classID int64) (*ClassSubjectSummaryStats, error) {
	return uc.scoreRepo.GetClassSubjectSummary(ctx, examID, classID)
}

// GetSingleClassSummary 获取单科班级汇总
func (uc *ExamAnalysisUseCase) GetSingleClassSummary(ctx context.Context, examID, subjectID int64) (*SingleClassSummaryStats, error) {
	return uc.scoreRepo.GetSingleClassSummary(ctx, examID, subjectID)
}

// GetSingleClassQuestions 获取单科班级题目汇总
func (uc *ExamAnalysisUseCase) GetSingleClassQuestions(ctx context.Context, examID, subjectID, classID int64) (*SingleClassQuestionStats, error) {
	if err := uc.requireScoreItemRepo(); err != nil {
		return nil, err
	}

	stats, err := uc.scoreItemRepo.GetSingleClassQuestions(ctx, examID, subjectID, classID)
	if err != nil {
		return nil, err
	}

	sort.Slice(stats.Questions, func(i, j int) bool {
		return questionNumberLess(stats.Questions[i].QuestionNumber, stats.Questions[j].QuestionNumber)
	})
	for _, question := range stats.Questions {
		question.QuestionID = normalizeQuestionID(question.QuestionID, question.QuestionNumber)
		question.Difficulty = difficultyFromScoreRate(question.ScoreRate)
	}

	return stats, nil
}

// GetSingleQuestionSummary 获取单科题目汇总
func (uc *ExamAnalysisUseCase) GetSingleQuestionSummary(ctx context.Context, examID, subjectID int64) (*SingleQuestionSummaryStats, error) {
	if err := uc.requireScoreItemRepo(); err != nil {
		return nil, err
	}

	stats, err := uc.scoreItemRepo.GetSingleQuestionSummary(ctx, examID, subjectID)
	if err != nil {
		return nil, err
	}

	sort.Slice(stats.Questions, func(i, j int) bool {
		return questionNumberLess(stats.Questions[i].QuestionNumber, stats.Questions[j].QuestionNumber)
	})
	for _, question := range stats.Questions {
		question.QuestionID = normalizeQuestionID(question.QuestionID, question.QuestionNumber)
		question.Difficulty = difficultyFromScoreRate(question.ScoreRate)
	}

	return stats, nil
}

// GetSingleQuestionDetail 获取单科班级题目详情
func (uc *ExamAnalysisUseCase) GetSingleQuestionDetail(ctx context.Context, examID, subjectID, classID int64, questionID string) (*SingleQuestionDetailStats, error) {
	if err := uc.requireScoreItemRepo(); err != nil {
		return nil, err
	}

	stats, err := uc.scoreItemRepo.GetSingleQuestionDetail(ctx, examID, subjectID, classID, questionID)
	if err != nil {
		return nil, err
	}

	stats.QuestionID = normalizeQuestionID(stats.QuestionID, stats.QuestionNumber)
	sort.SliceStable(stats.Students, func(i, j int) bool {
		if stats.Students[i].Score == stats.Students[j].Score {
			return stats.Students[i].StudentID < stats.Students[j].StudentID
		}
		return stats.Students[i].Score > stats.Students[j].Score
	})
	assignSequentialRanks(stats.Students, func(item *StudentQuestionDetailStats, rank int32) {
		item.ClassRank = rank
	})

	gradeOrdered := make([]*StudentQuestionDetailStats, len(stats.Students))
	copy(gradeOrdered, stats.Students)
	sort.SliceStable(gradeOrdered, func(i, j int) bool {
		if gradeOrdered[i].Score == gradeOrdered[j].Score {
			return gradeOrdered[i].StudentID < gradeOrdered[j].StudentID
		}
		return gradeOrdered[i].Score > gradeOrdered[j].Score
	})
	assignSequentialRanks(gradeOrdered, func(item *StudentQuestionDetailStats, rank int32) {
		item.GradeRank = rank
	})

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
	classRating.Medium.Percentage = roundTo2Decimal(float64(classRating.Medium.Count) / total * 100)
	classRating.Pass.Percentage = roundTo2Decimal(float64(classRating.Pass.Count) / total * 100)
	classRating.LowScore.Percentage = roundTo2Decimal(float64(classRating.LowScore.Count) / total * 100)
}

// roundTo2Decimal 四舍五入到2位小数
func roundTo2Decimal(value float64) float64 {
	return math.Round(value*100) / 100
}

func (uc *ExamAnalysisUseCase) requireScoreItemRepo() error {
	if uc.scoreItemRepo == nil {
		return errors.New("score item repo is not configured")
	}
	return nil
}

func normalizeQuestionID(questionID, questionNumber string) string {
	return questionNumber
}

func questionNumberLess(left, right string) bool {
	leftNumber, leftErr := strconv.Atoi(left)
	rightNumber, rightErr := strconv.Atoi(right)
	if leftErr == nil && rightErr == nil {
		return leftNumber < rightNumber
	}
	return left < right
}

func difficultyFromScoreRate(scoreRate float64) float64 {
	return scoreRate
}

func assignSequentialRanks(items []*StudentQuestionDetailStats, setRank func(item *StudentQuestionDetailStats, rank int32)) {
	for index, item := range items {
		setRank(item, int32(index+1))
	}
}

// GetExamName 获取考试名称
func (uc *ExamAnalysisUseCase) GetExamName(ctx context.Context, examID int64) (string, error) {
	return uc.examRepo.GetExamName(ctx, examID)
}

// GetExamStudentCounts 批量获取考试的独立学生人数
func (uc *ExamAnalysisUseCase) GetExamStudentCounts(ctx context.Context, examIDs []int64) (map[int64]int64, error) {
	return uc.examRepo.GetExamStudentCounts(ctx, examIDs)
}

// DeleteExam 删除考试及其关联数据
func (uc *ExamAnalysisUseCase) DeleteExam(ctx context.Context, examID int64) error {
	return uc.examRepo.Delete(ctx, examID)
}
