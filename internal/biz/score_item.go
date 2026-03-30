package biz

import "context"

type ScoreItem struct {
	ID             int64   `gorm:"primaryKey;column:id"`
	ScoreID        int64   `gorm:"index;column:score_id"`                    // 外键关联 score 表
	QuestionNumber string  `gorm:"type:varchar(20);column:question_number"`  // 小题编号
	KnowledgePoint string  `gorm:"type:varchar(100);column:knowledge_point"` // 知识点
	Score          float64 `gorm:"column:score"`                             // 得分
	FullScore      float64 `gorm:"column:full_score"`                        // 总分
	IsCorrect      bool    `gorm:"column:is_correct"`                         // 是否正确
}

func (ScoreItem) TableName() string {
	return "score_items"
}

type ScoreItemRepo interface {
	ListByScoreID(ctx context.Context, scoreID int64) ([]*ScoreItem, error)
}
