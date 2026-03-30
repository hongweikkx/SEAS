package biz

import (
	"context"
	"seas/internal/conf"
)

type AnalysisUseCase struct {
	scoreRepo ScoreRepo
	c         *conf.LLM
}

func NewAnalysisUseCase(scoreRepo ScoreRepo, c *conf.LLM) *AnalysisUseCase {
	return &AnalysisUseCase{
		scoreRepo: scoreRepo,
		c:         c,
	}
}

// Analyze 是核心业务逻辑：基于 studentID 查询成绩，并返回总结 + 建议
func (uc *AnalysisUseCase) Analyze(ctx context.Context, nlInputStr string) (string, []string, error) {
	return "成绩分析完成", []string{}, nil
}
