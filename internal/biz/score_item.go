package biz

import "context"

type ScoreItem struct {
	ID             int64   `gorm:"primaryKey;column:id"`
	ScoreID        int64   `gorm:"index;column:score_id"`                    // 外键关联 score 表
	QuestionNumber string  `gorm:"type:varchar(20);column:question_number"`  // 小题编号
	KnowledgePoint string  `gorm:"type:varchar(100);column:knowledge_point"` // 知识点
	Score          float64 `gorm:"column:score"`                             // 得分
	FullScore      float64 `gorm:"column:full_score"`                        // 总分
	IsCorrect      bool    `gorm:"column:is_correct"`                        // 是否正确
}

func (ScoreItem) TableName() string {
	return "score_items"
}

type ScoreItemRepo interface {
	ListByScoreID(ctx context.Context, scoreID int64) ([]*ScoreItem, error)
	GetSingleClassQuestions(ctx context.Context, examID, subjectID, classID int64) (*SingleClassQuestionStats, error)
	GetSingleQuestionSummary(ctx context.Context, examID, subjectID int64) (*SingleQuestionSummaryStats, error)
	GetSingleQuestionDetail(ctx context.Context, examID, subjectID, classID int64, questionID string) (*SingleQuestionDetailStats, error)
}

// SingleClassQuestionStats 单科班级题目汇总
type SingleClassQuestionStats struct {
	ExamID      int64
	ExamName    string
	SubjectID   int64
	SubjectName string
	ClassID     int64
	ClassName   string
	Questions   []*ClassQuestionItemStats
}

// ClassQuestionItemStats 单科班级题目汇总项
type ClassQuestionItemStats struct {
	QuestionID     string
	QuestionNumber string
	QuestionType   string
	FullScore      float64
	ClassAvgScore  float64
	ScoreRate      float64
	GradeAvgScore  float64
	Difficulty     string
}

// SingleQuestionDetailStats 单科班级题目详情
type SingleQuestionDetailStats struct {
	ExamID          int64
	ExamName        string
	SubjectID       int64
	SubjectName     string
	ClassID         int64
	ClassName       string
	QuestionID      string
	QuestionNumber  string
	QuestionType    string
	FullScore       float64
	QuestionContent string
	Students        []*StudentQuestionDetailStats
}

// StudentQuestionDetailStats 学生题目详情
type StudentQuestionDetailStats struct {
	StudentID     int64
	StudentName   string
	Score         float64
	FullScore     float64
	ScoreRate     float64
	ClassRank     int32
	GradeRank     int32
	AnswerContent string
}
