package data

import (
	"context"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type scoreItemRepo struct {
	data *Data
}

func NewScoreItemRepo(data *Data) biz.ScoreItemRepo {
	return &scoreItemRepo{
		data: data,
	}
}

func (r *scoreItemRepo) ListByScoreID(ctx context.Context, scoreID int64) ([]*biz.ScoreItem, error) {
	var items []*biz.ScoreItem
	err := r.data.db.WithContext(ctx).Where("score_id = ?", scoreID).Find(&items).Error
	if err != nil {
		log.Context(ctx).Errorf("query scoreItemRepo.ListByScoreID err: %+v", err)
		return nil, err
	}
	return items, nil
}

func (r *scoreItemRepo) GetSingleClassQuestions(ctx context.Context, examID, subjectID, classID int64) (*biz.SingleClassQuestionStats, error) {
	var rows []struct {
		QuestionNumber string  `gorm:"column:question_number"`
		FullScore      float64 `gorm:"column:full_score"`
		ClassAvgScore  float64 `gorm:"column:class_avg_score"`
		GradeAvgScore  float64 `gorm:"column:grade_avg_score"`
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			si.question_number,
			GREATEST(MAX(si.full_score), MAX(si.score)) AS full_score,
			ROUND(AVG(CASE WHEN st.class_id = ? THEN si.score END), 2) AS class_avg_score,
			ROUND(AVG(si.score), 2) AS grade_avg_score
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		JOIN students st ON st.id = sc.student_id
		WHERE sc.exam_id = ? AND sc.subject_id = ?
		GROUP BY si.question_number
		HAVING class_avg_score IS NOT NULL
	`, classID, examID, subjectID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// 查询科目名称和班级名称
	var subjectName, className string
	r.data.db.WithContext(ctx).Raw(`SELECT name FROM subjects WHERE id = ?`, subjectID).Scan(&subjectName)
	r.data.db.WithContext(ctx).Raw(`SELECT name FROM classes WHERE id = ?`, classID).Scan(&className)

	stats := &biz.SingleClassQuestionStats{
		ExamID:      examID,
		SubjectID:   subjectID,
		SubjectName: subjectName,
		ClassID:     classID,
		ClassName:   className,
	}
	stats.Questions = make([]*biz.ClassQuestionItemStats, 0, len(rows))
	for _, row := range rows {
		scoreRate := 0.0
		if row.FullScore > 0 {
			scoreRate = roundTo2Decimal(row.ClassAvgScore / row.FullScore * 100)
		}
		stats.Questions = append(stats.Questions, &biz.ClassQuestionItemStats{
			QuestionID:     row.QuestionNumber,
			QuestionNumber: row.QuestionNumber,
			QuestionType:   "",
			FullScore:      row.FullScore,
			ClassAvgScore:  row.ClassAvgScore,
			ScoreRate:      scoreRate,
			GradeAvgScore:  row.GradeAvgScore,
		})
	}
	return stats, nil
}

// BatchCreate 批量创建小题成绩记录
func (r *scoreItemRepo) BatchCreate(ctx context.Context, items []*biz.ScoreItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.data.db.WithContext(ctx).CreateInBatches(items, 100).Error
}

func (r *scoreItemRepo) GetSingleQuestionSummary(ctx context.Context, examID, subjectID int64) (*biz.SingleQuestionSummaryStats, error) {
	var rows []struct {
		QuestionNumber string  `gorm:"column:question_number"`
		FullScore      float64 `gorm:"column:full_score"`
		GradeAvgScore  float64 `gorm:"column:grade_avg_score"`
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			si.question_number,
			GREATEST(MAX(si.full_score), MAX(si.score)) AS full_score,
			ROUND(AVG(si.score), 2) AS grade_avg_score
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		WHERE sc.exam_id = ? AND sc.subject_id = ?
		GROUP BY si.question_number
	`, examID, subjectID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// 查询各班得分明细
	var breakdownRows []struct {
		QuestionNumber string  `gorm:"column:question_number"`
		ClassID        int64   `gorm:"column:class_id"`
		ClassName      string  `gorm:"column:class_name"`
		AvgScore       float64 `gorm:"column:avg_score"`
	}
	err = r.data.db.WithContext(ctx).Raw(`
		SELECT
			si.question_number,
			st.class_id,
			c.name AS class_name,
			ROUND(AVG(si.score), 2) AS avg_score
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		JOIN students st ON st.id = sc.student_id
		JOIN classes c ON c.id = st.class_id
		WHERE sc.exam_id = ? AND sc.subject_id = ?
		GROUP BY si.question_number, st.class_id, c.name
		ORDER BY si.question_number, st.class_id
	`, examID, subjectID).Scan(&breakdownRows).Error
	if err != nil {
		return nil, err
	}

	breakdownMap := make(map[string][]*biz.QuestionClassBreakdownStats)
	for _, br := range breakdownRows {
		breakdownMap[br.QuestionNumber] = append(breakdownMap[br.QuestionNumber], &biz.QuestionClassBreakdownStats{
			ClassID:   br.ClassID,
			ClassName: br.ClassName,
			AvgScore:  br.AvgScore,
		})
	}

	// 查询科目名称
	var subjectName string
	r.data.db.WithContext(ctx).Raw(`SELECT name FROM subjects WHERE id = ?`, subjectID).Scan(&subjectName)

	stats := &biz.SingleQuestionSummaryStats{ExamID: examID, SubjectID: subjectID, SubjectName: subjectName}
	stats.Questions = make([]*biz.SingleQuestionSummaryItemStats, 0, len(rows))
	for _, row := range rows {
		item := &biz.SingleQuestionSummaryItemStats{
			QuestionID:     row.QuestionNumber,
			QuestionNumber: row.QuestionNumber,
			QuestionType:   "",
			FullScore:      row.FullScore,
			GradeAvgScore:  row.GradeAvgScore,
			ClassBreakdown: breakdownMap[row.QuestionNumber],
		}
		if row.FullScore > 0 {
			item.ScoreRate = roundTo2Decimal(row.GradeAvgScore / row.FullScore * 100)
		}
		stats.Questions = append(stats.Questions, item)
	}
	return stats, nil
}

func (r *scoreItemRepo) GetSingleQuestionDetail(ctx context.Context, examID, subjectID, classID int64, questionID string) (*biz.SingleQuestionDetailStats, error) {
	var rows []struct {
		StudentID   int64   `gorm:"column:student_id"`
		StudentName string  `gorm:"column:student_name"`
		Score       float64 `gorm:"column:score"`
		FullScore   float64 `gorm:"column:full_score"`
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			st.id AS student_id,
			st.name AS student_name,
			si.score,
			si.full_score
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		JOIN students st ON st.id = sc.student_id
		WHERE sc.exam_id = ? AND sc.subject_id = ? AND st.class_id = ? AND si.question_number = ?
		ORDER BY si.score DESC, st.id ASC
	`, examID, subjectID, classID, questionID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := &biz.SingleQuestionDetailStats{
		ExamID:          examID,
		SubjectID:       subjectID,
		ClassID:         classID,
		QuestionID:      questionID,
		QuestionNumber:  questionID,
		QuestionType:    "",
		QuestionContent: "",
	}

	stats.Students = make([]*biz.StudentQuestionDetailStats, 0, len(rows))
	for _, row := range rows {
		scoreRate := 0.0
		if row.FullScore > 0 {
			scoreRate = roundTo2Decimal(row.Score / row.FullScore * 100)
		}
		stats.Students = append(stats.Students, &biz.StudentQuestionDetailStats{
			StudentID:     row.StudentID,
			StudentName:   row.StudentName,
			Score:         row.Score,
			FullScore:     row.FullScore,
			ScoreRate:     scoreRate,
			AnswerContent: "",
		})
	}
	return stats, nil
}
