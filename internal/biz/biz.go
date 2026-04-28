package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewAnalysisUseCase,
	NewExamAnalysisUseCaseWithScoreItem,
	NewExamImportUseCase,
)

// NewExamAnalysisUseCaseWithScoreItem 创建完整的考试分析用例（包含 scoreItemRepo）
func NewExamAnalysisUseCaseWithScoreItem(examRepo ExamRepo, subjectRepo SubjectRepo, scoreRepo ScoreRepo, scoreItemRepo ScoreItemRepo) *ExamAnalysisUseCase {
	uc := NewExamAnalysisUseCase(examRepo, subjectRepo, scoreRepo)
	return uc.WithScoreItemRepo(scoreItemRepo)
}
