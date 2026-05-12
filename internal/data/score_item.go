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
			CASE WHEN MAX(si.full_score) > MAX(si.score) THEN MAX(si.full_score) ELSE MAX(si.score) END AS full_score,
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
		Participants   int     `gorm:"column:participants"`
		HighestScore   float64 `gorm:"column:highest_score"`
		LowestScore    float64 `gorm:"column:lowest_score"`
		StdDev         float64 `gorm:"column:std_dev"`
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			si.question_number,
			CASE WHEN MAX(si.full_score) > MAX(si.score) THEN MAX(si.full_score) ELSE MAX(si.score) END AS full_score,
			ROUND(AVG(si.score), 2) AS grade_avg_score,
			COUNT(*) AS participants,
			MAX(si.score) AS highest_score,
			MIN(si.score) AS lowest_score,
			ROUND(IFNULL(STDDEV_SAMP(si.score), 0), 2) AS std_dev
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
		Participants   int     `gorm:"column:participants"`
		StdDev         float64 `gorm:"column:std_dev"`
	}
	err = r.data.db.WithContext(ctx).Raw(`
		SELECT
			si.question_number,
			st.class_id,
			c.name AS class_name,
			ROUND(AVG(si.score), 2) AS avg_score,
			COUNT(*) AS participants,
			ROUND(IFNULL(STDDEV_SAMP(si.score), 0), 2) AS std_dev
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
			ClassID:      br.ClassID,
			ClassName:    br.ClassName,
			AvgScore:     br.AvgScore,
			Participants: br.Participants,
			StdDev:       br.StdDev,
		})
	}

	// 查询区分度（高低分组法：总分前27%与后27%的平均得分率之差）
	var discriminationRows []struct {
		QuestionNumber  string  `gorm:"column:question_number"`
		FullScore       float64 `gorm:"column:full_score"`
		HighGroupAvg    float64 `gorm:"column:high_group_avg"`
		LowGroupAvg     float64 `gorm:"column:low_group_avg"`
	}
	err = r.data.db.WithContext(ctx).Raw(`
		WITH student_total AS (
			SELECT sc.student_id, SUM(si.score) as total_score
			FROM score_items si
			JOIN scores sc ON sc.id = si.score_id
			WHERE sc.exam_id = ? AND sc.subject_id = ?
			GROUP BY sc.student_id
		),
		cutoff AS (
			-- SQLite 无 CEIL,对非负数等价改写: ceil(x) = cast(x as int) + (1 if 有小数部分 else 0)
			SELECT CAST(COUNT(*) * 0.27 AS INTEGER) +
				CASE WHEN COUNT(*) * 0.27 > CAST(COUNT(*) * 0.27 AS INTEGER) THEN 1 ELSE 0 END as k
			FROM student_total
		),
		student_rank AS (
			SELECT student_id,
				ROW_NUMBER() OVER (ORDER BY total_score DESC) as rn_desc,
				ROW_NUMBER() OVER (ORDER BY total_score ASC) as rn_asc
			FROM student_total
		)
		SELECT
			si.question_number,
			MAX(si.full_score) as full_score,
			IFNULL(AVG(CASE WHEN sr.rn_desc <= c.k THEN si.score END), 0) as high_group_avg,
			IFNULL(AVG(CASE WHEN sr.rn_asc <= c.k THEN si.score END), 0) as low_group_avg
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		JOIN student_rank sr ON sr.student_id = sc.student_id
		CROSS JOIN cutoff c
		WHERE sc.exam_id = ? AND sc.subject_id = ?
		GROUP BY si.question_number
	`, examID, subjectID, examID, subjectID).Scan(&discriminationRows).Error
	if err != nil {
		return nil, err
	}

	discriminationMap := make(map[string]float64)
	for _, dr := range discriminationRows {
		if dr.FullScore > 0 {
			discriminationMap[dr.QuestionNumber] = roundTo2Decimal((dr.HighGroupAvg - dr.LowGroupAvg) / dr.FullScore * 100)
		}
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
			Participants:   row.Participants,
			HighestScore:   row.HighestScore,
			LowestScore:    row.LowestScore,
			StdDev:         row.StdDev,
			Discrimination: discriminationMap[row.QuestionNumber],
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

	var err error
	if classID == 0 {
		// 全年级,不限制班级
		err = r.data.db.WithContext(ctx).Raw(`
			SELECT
				st.id AS student_id,
				st.name AS student_name,
				si.score,
				si.full_score
			FROM score_items si
			JOIN scores sc ON sc.id = si.score_id
			JOIN students st ON st.id = sc.student_id
			WHERE sc.exam_id = ? AND sc.subject_id = ? AND si.question_number = ?
			ORDER BY si.score DESC, st.id ASC
		`, examID, subjectID, questionID).Scan(&rows).Error
	} else {
		err = r.data.db.WithContext(ctx).Raw(`
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
	}
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

func (r *scoreItemRepo) GetSingleQuestionClassCompare(ctx context.Context, examID, subjectID int64, questionID string) (*biz.SingleQuestionClassCompareStats, error) {
	// 全年级聚合
	var overallRow struct {
		Participants int     `gorm:"column:participants"`
		AvgScore     float64 `gorm:"column:avg_score"`
		HighestScore float64 `gorm:"column:highest_score"`
		LowestScore  float64 `gorm:"column:lowest_score"`
		StdDev       float64 `gorm:"column:std_dev"`
		FullScore    float64 `gorm:"column:full_score"`
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS participants,
			ROUND(AVG(si.score), 2) AS avg_score,
			MAX(si.score) AS highest_score,
			MIN(si.score) AS lowest_score,
			ROUND(IFNULL(STDDEV_SAMP(si.score), 0), 2) AS std_dev,
			MAX(si.full_score) AS full_score
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		WHERE sc.exam_id = ? AND sc.subject_id = ? AND si.question_number = ?
	`, examID, subjectID, questionID).Scan(&overallRow).Error
	if err != nil {
		return nil, err
	}

	// 各班聚合(按均分降序,同分按 class_id 升序)
	var classRows []struct {
		ClassID      int64   `gorm:"column:class_id"`
		ClassName    string  `gorm:"column:class_name"`
		Participants int     `gorm:"column:participants"`
		AvgScore     float64 `gorm:"column:avg_score"`
		HighestScore float64 `gorm:"column:highest_score"`
		LowestScore  float64 `gorm:"column:lowest_score"`
		StdDev       float64 `gorm:"column:std_dev"`
	}
	err = r.data.db.WithContext(ctx).Raw(`
		SELECT
			st.class_id,
			c.name AS class_name,
			COUNT(*) AS participants,
			ROUND(AVG(si.score), 2) AS avg_score,
			MAX(si.score) AS highest_score,
			MIN(si.score) AS lowest_score,
			ROUND(IFNULL(STDDEV_SAMP(si.score), 0), 2) AS std_dev
		FROM score_items si
		JOIN scores sc ON sc.id = si.score_id
		JOIN students st ON st.id = sc.student_id
		JOIN classes c ON c.id = st.class_id
		WHERE sc.exam_id = ? AND sc.subject_id = ? AND si.question_number = ?
		GROUP BY st.class_id, c.name
		ORDER BY avg_score DESC, st.class_id ASC
	`, examID, subjectID, questionID).Scan(&classRows).Error
	if err != nil {
		return nil, err
	}

	stats := &biz.SingleQuestionClassCompareStats{
		ExamID:         examID,
		SubjectID:      subjectID,
		QuestionID:     questionID,
		QuestionNumber: questionID,
		FullScore:      overallRow.FullScore,
	}

	scoreRate := 0.0
	if overallRow.FullScore > 0 {
		scoreRate = roundTo2Decimal(overallRow.AvgScore / overallRow.FullScore * 100)
	}

	totalClasses := int32(len(classRows))
	stats.Overall = &biz.SingleQuestionClassCompareItemStats{
		ClassID:      0,
		ClassName:    "全年级",
		Participants: overallRow.Participants,
		AvgScore:     overallRow.AvgScore,
		ScoreRate:    scoreRate,
		ScoreDiff:    0,
		ClassRank:    0,
		TotalClasses: totalClasses,
		HighestScore: overallRow.HighestScore,
		LowestScore:  overallRow.LowestScore,
		StdDev:       overallRow.StdDev,
	}

	for i, row := range classRows {
		sr := 0.0
		if overallRow.FullScore > 0 {
			sr = roundTo2Decimal(row.AvgScore / overallRow.FullScore * 100)
		}
		stats.Classes = append(stats.Classes, &biz.SingleQuestionClassCompareItemStats{
			ClassID:      row.ClassID,
			ClassName:    row.ClassName,
			Participants: row.Participants,
			AvgScore:     row.AvgScore,
			ScoreRate:    sr,
			ScoreDiff:    roundTo2Decimal(row.AvgScore - overallRow.AvgScore),
			ClassRank:    int32(i + 1),
			TotalClasses: totalClasses,
			HighestScore: row.HighestScore,
			LowestScore:  row.LowestScore,
			StdDev:       row.StdDev,
		})
	}

	return stats, nil
}
