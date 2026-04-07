package data

import (
	"context"
	"errors"
	"math"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type scoreRepo struct {
	data        *Data
	subjectRepo biz.SubjectRepo
}

func NewScoreRepo(data *Data, subjectRepo biz.SubjectRepo) biz.ScoreRepo {
	return &scoreRepo{
		data:        data,
		subjectRepo: subjectRepo,
	}
}

func (r *scoreRepo) GetByExamSubjectStudent(ctx context.Context, examID, subjectID, studentID int64) (*biz.Score, error) {
	var s biz.Score
	err := r.data.db.WithContext(ctx).Where("exam_id = ? AND subject_id = ? AND student_id = ?", examID, subjectID, studentID).First(&s).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("scoreRepo.GetByExamSubjectStudent err: %+v", err)
	}
	return &s, err
}

func (r *scoreRepo) GetByStudentID(ctx context.Context, studentID int64) ([]*biz.Score, error) {
	var scores []*biz.Score
	err := r.data.db.WithContext(ctx).Where("student_id = ?", studentID).Find(&scores).Error
	if err != nil {
		log.Context(ctx).Errorf("scoreRepo.GetByStudentID err: %+v", err)
		return nil, err
	}
	return scores, nil
}

// GetSubjectSummary 获取学科统计信息（全科或单科）
func (r *scoreRepo) GetSubjectSummary(ctx context.Context, examID, subjectID int64) (*biz.SubjectSummaryStats, error) {
	var stats biz.SubjectSummaryStats

	// 获取总参考人数
	var totalParticipants int64
	query := r.data.db.WithContext(ctx).Model(&biz.Score{}).Where("exam_id = ?", examID)
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	if err := query.Count(&totalParticipants).Error; err != nil {
		log.Context(ctx).Errorf("GetSubjectSummary count participants err: %+v", err)
		return nil, err
	}
	stats.TotalParticipants = totalParticipants

	if subjectID == 0 {
		// 全科模式：统计所有学科
		var subjectStats []struct {
			SubjectID    int64   `gorm:"column:subject_id"`
			SubjectName  string  `gorm:"column:name"`
			FullScore    float64 `gorm:"column:full_score"`
			AvgScore     float64 `gorm:"column:avg_score"`
			HighestScore float64 `gorm:"column:highest_score"`
			LowestScore  float64 `gorm:"column:lowest_score"`
			StudentCount int64   `gorm:"column:student_count"`
		}

		err := r.data.db.WithContext(ctx).Raw(`
			SELECT
				s.id as subject_id,
				s.name,
				es.full_score,
				ROUND(AVG(sc.total_score), 2) as avg_score,
				MAX(sc.total_score) as highest_score,
				MIN(sc.total_score) as lowest_score,
				COUNT(sc.student_id) as student_count
			FROM subjects s
			JOIN exam_subjects es ON es.subject_id = s.id AND es.exam_id = ?
			LEFT JOIN scores sc ON sc.subject_id = s.id AND sc.exam_id = ?
			GROUP BY s.id, s.name, es.full_score
			ORDER BY s.id
		`, examID, examID).Scan(&subjectStats).Error

		if err != nil {
			log.Context(ctx).Errorf("GetSubjectSummary all subjects err: %+v", err)
			return nil, err
		}

		stats.SubjectsInvolved = int32(len(subjectStats))
		stats.Subjects = make([]*biz.SubjectStats, len(subjectStats))

		for i, stat := range subjectStats {
			stats.Subjects[i] = &biz.SubjectStats{
				ID:           stat.SubjectID,
				Name:         stat.SubjectName,
				FullScore:    stat.FullScore,
				AvgScore:     stat.AvgScore,
				HighestScore: stat.HighestScore,
				LowestScore:  stat.LowestScore,
				Difficulty:   r.calculateDifficulty(stat.AvgScore, stat.FullScore),
				StudentCount: stat.StudentCount,
			}
		}

		// 计算涉及班级数
		var classesInvolved int32
		r.data.db.WithContext(ctx).Raw(`
			SELECT COUNT(DISTINCT st.class_id) as classes_count
			FROM scores sc
			JOIN students st ON st.id = sc.student_id
			WHERE sc.exam_id = ?
		`, examID).Scan(&classesInvolved)
		stats.ClassesInvolved = classesInvolved

	} else {
		// 单科模式
		var subjectStat struct {
			SubjectID    int64   `gorm:"column:subject_id"`
			SubjectName  string  `gorm:"column:name"`
			FullScore    float64 `gorm:"column:full_score"`
			AvgScore     float64 `gorm:"column:avg_score"`
			HighestScore float64 `gorm:"column:highest_score"`
			LowestScore  float64 `gorm:"column:lowest_score"`
			StudentCount int64   `gorm:"column:student_count"`
		}

		err := r.data.db.WithContext(ctx).Raw(`
			SELECT
				s.id as subject_id,
				s.name,
				es.full_score,
				ROUND(AVG(sc.total_score), 2) as avg_score,
				MAX(sc.total_score) as highest_score,
				MIN(sc.total_score) as lowest_score,
				COUNT(sc.student_id) as student_count
			FROM subjects s
			JOIN exam_subjects es ON es.subject_id = s.id AND es.exam_id = ? AND es.subject_id = ?
			LEFT JOIN scores sc ON sc.subject_id = s.id AND sc.exam_id = ? AND sc.subject_id = ?
			WHERE s.id = ?
			GROUP BY s.id, s.name, es.full_score
		`, examID, subjectID, examID, subjectID, subjectID).Scan(&subjectStat).Error

		if err != nil {
			log.Context(ctx).Errorf("GetSubjectSummary single subject err: %+v", err)
			return nil, err
		}

		stats.Subjects = []*biz.SubjectStats{
			{
				ID:           subjectStat.SubjectID,
				Name:         subjectStat.SubjectName,
				FullScore:    subjectStat.FullScore,
				AvgScore:     subjectStat.AvgScore,
				HighestScore: subjectStat.HighestScore,
				LowestScore:  subjectStat.LowestScore,
				Difficulty:   r.calculateDifficulty(subjectStat.AvgScore, subjectStat.FullScore),
				StudentCount: subjectStat.StudentCount,
			},
		}
	}

	return &stats, nil
}

// GetClassSummary 获取班级统计信息（全科或单科）
func (r *scoreRepo) GetClassSummary(ctx context.Context, examID, subjectID int64) (*biz.ClassSummaryStats, error) {
	var stats biz.ClassSummaryStats

	// 获取总参考人数
	var totalParticipants int64
	query := r.data.db.WithContext(ctx).Model(&biz.Score{}).Where("exam_id = ?", examID)
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	if err := query.Count(&totalParticipants).Error; err != nil {
		log.Context(ctx).Errorf("GetClassSummary count participants err: %+v", err)
		return nil, err
	}
	stats.TotalParticipants = totalParticipants

	var fullScore float64
	if subjectID > 0 {
		fullScore, _ = r.subjectRepo.GetFullScoreByExamSubject(ctx, examID, subjectID)
	} else {
		// 全科模式：计算加权平均满分
		fullScore = r.calculateOverallFullScore(ctx, examID)
	}

	// 查询各班级统计
	var classStats []struct {
		ClassID       int64   `gorm:"column:class_id"`
		ClassName     string  `gorm:"column:class_name"`
		TotalStudents int64   `gorm:"column:total_students"`
		AvgScore      float64 `gorm:"column:avg_score"`
		HighestScore  float64 `gorm:"column:highest_score"`
		LowestScore   float64 `gorm:"column:lowest_score"`
		StdDev        float64 `gorm:"column:std_dev"`
	}

	whereClause := "sc.exam_id = ?"
	args := []interface{}{examID}
	if subjectID > 0 {
		whereClause += " AND sc.subject_id = ?"
		args = append(args, subjectID)
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			c.id as class_id,
			c.name as class_name,
			COUNT(sc.student_id) as total_students,
			ROUND(AVG(sc.total_score), 2) as avg_score,
			MAX(sc.total_score) as highest_score,
			MIN(sc.total_score) as lowest_score,
			ROUND(STDDEV_POP(sc.total_score), 2) as std_dev
		FROM classes c
		LEFT JOIN students st ON st.class_id = c.id
		LEFT JOIN scores sc ON sc.student_id = st.id AND `+whereClause+`
		GROUP BY c.id, c.name
		HAVING COUNT(sc.student_id) > 0
		ORDER BY c.id
	`, args...).Scan(&classStats).Error

	if err != nil {
		log.Context(ctx).Errorf("GetClassSummary class stats err: %+v", err)
		return nil, err
	}

	stats.ClassDetails = make([]*biz.ClassStats, len(classStats))
	var overallAvg float64
	var overallHighest float64
	var overallLowest float64 = 999
	var overallStdDev float64

	for i, stat := range classStats {
		stats.ClassDetails[i] = &biz.ClassStats{
			ClassID:       stat.ClassID,
			ClassName:     stat.ClassName,
			TotalStudents: stat.TotalStudents,
			AvgScore:      stat.AvgScore,
			HighestScore:  stat.HighestScore,
			LowestScore:   stat.LowestScore,
			Difficulty:    r.calculateDifficulty(stat.AvgScore, fullScore),
			StdDev:        stat.StdDev,
		}

		// 累加计算全年级统计
		overallAvg += stat.AvgScore * float64(stat.TotalStudents)
		if stat.HighestScore > overallHighest {
			overallHighest = stat.HighestScore
		}
		if stat.LowestScore < overallLowest {
			overallLowest = stat.LowestScore
		}
		overallStdDev += stat.StdDev * stat.StdDev * float64(stat.TotalStudents)
	}

	// 计算全年级平均值
	if totalParticipants > 0 {
		overallAvg = overallAvg / float64(totalParticipants)
		overallStdDev = math.Sqrt(overallStdDev / float64(totalParticipants))
	}

	stats.OverallGrade = &biz.ClassStats{
		ClassID:       0,
		ClassName:     "全年级",
		TotalStudents: totalParticipants,
		AvgScore:      math.Round(overallAvg*100) / 100,
		HighestScore:  overallHighest,
		LowestScore:   overallLowest,
		Difficulty:    r.calculateDifficulty(overallAvg, r.calculateOverallFullScore(ctx, examID)),
		StdDev:        math.Round(overallStdDev*100) / 100,
	}

	return &stats, nil
}

// GetRatingDistribution 获取四率分布统计
func (r *scoreRepo) GetRatingDistribution(ctx context.Context, examID, subjectID int64, excellentThreshold, goodThreshold, passThreshold float64) (*biz.RatingDistributionStats, error) {
	var stats biz.RatingDistributionStats

	// 获取总参考人数
	var totalParticipants int64
	query := r.data.db.WithContext(ctx).Model(&biz.Score{}).Where("exam_id = ?", examID)
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	if err := query.Count(&totalParticipants).Error; err != nil {
		log.Context(ctx).Errorf("GetRatingDistribution count participants err: %+v", err)
		return nil, err
	}
	stats.TotalParticipants = totalParticipants

	// 查询各班级四率统计
	var ratingStats []struct {
		ClassID       int64   `gorm:"column:class_id"`
		ClassName     string  `gorm:"column:class_name"`
		TotalStudents int64   `gorm:"column:total_students"`
		AvgScore      float64 `gorm:"column:avg_score"`
		Excellent     int64   `gorm:"column:excellent"`
		Good          int64   `gorm:"column:good"`
		Pass          int64   `gorm:"column:pass"`
		Fail          int64   `gorm:"column:fail"`
	}

	whereClause := "sc.exam_id = ?"
	args := []interface{}{examID}
	if subjectID > 0 {
		whereClause += " AND sc.subject_id = ?"
		args = append(args, subjectID)
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			c.id as class_id,
			c.name as class_name,
			COUNT(sc.student_id) as total_students,
			ROUND(AVG(sc.total_score), 2) as avg_score,
			SUM(CASE WHEN sc.total_score >= ? THEN 1 ELSE 0 END) as excellent,
			SUM(CASE WHEN sc.total_score >= ? AND sc.total_score < ? THEN 1 ELSE 0 END) as good,
			SUM(CASE WHEN sc.total_score >= ? AND sc.total_score < ? THEN 1 ELSE 0 END) as pass,
			SUM(CASE WHEN sc.total_score < ? THEN 1 ELSE 0 END) as fail
		FROM classes c
		LEFT JOIN students st ON st.class_id = c.id
		LEFT JOIN scores sc ON sc.student_id = st.id AND `+whereClause+`
		GROUP BY c.id, c.name
		HAVING COUNT(sc.student_id) > 0
		ORDER BY c.id
	`, append([]interface{}{excellentThreshold, goodThreshold, excellentThreshold, passThreshold, goodThreshold, passThreshold}, args...)...).Scan(&ratingStats).Error

	if err != nil {
		log.Context(ctx).Errorf("GetRatingDistribution rating stats err: %+v", err)
		return nil, err
	}

	stats.ClassDetails = make([]*biz.ClassRatingStats, len(ratingStats))
	var overallExcellent, overallGood, overallPass, overallFail int64
	var overallAvg float64

	for i, stat := range ratingStats {
		stats.ClassDetails[i] = &biz.ClassRatingStats{
			ClassID:       stat.ClassID,
			ClassName:     stat.ClassName,
			TotalStudents: stat.TotalStudents,
			AvgScore:      stat.AvgScore,
			Excellent: &biz.RatingItemStats{
				Count: stat.Excellent,
			},
			Good: &biz.RatingItemStats{
				Count: stat.Good,
			},
			Pass: &biz.RatingItemStats{
				Count: stat.Pass,
			},
			Fail: &biz.RatingItemStats{
				Count: stat.Fail,
			},
		}

		// 累加全年级统计
		overallExcellent += stat.Excellent
		overallGood += stat.Good
		overallPass += stat.Pass
		overallFail += stat.Fail
		overallAvg += stat.AvgScore * float64(stat.TotalStudents)
	}

	// 计算全年级平均分
	if totalParticipants > 0 {
		overallAvg = overallAvg / float64(totalParticipants)
	}

	stats.OverallGrade = &biz.ClassRatingStats{
		ClassID:       0,
		ClassName:     "全年级",
		TotalStudents: totalParticipants,
		AvgScore:      math.Round(overallAvg*100) / 100,
		Excellent: &biz.RatingItemStats{
			Count: overallExcellent,
		},
		Good: &biz.RatingItemStats{
			Count: overallGood,
		},
		Pass: &biz.RatingItemStats{
			Count: overallPass,
		},
		Fail: &biz.RatingItemStats{
			Count: overallFail,
		},
	}

	return &stats, nil
}

// calculateDifficulty 计算难度 = 平均分/满分 * 100
func (r *scoreRepo) calculateDifficulty(avgScore, fullScore float64) float64 {
	if fullScore == 0 {
		return 0
	}
	return math.Round(avgScore/fullScore*100*100) / 100
}

// calculateOverallFullScore 计算全科平均满分
func (r *scoreRepo) calculateOverallFullScore(ctx context.Context, examID int64) float64 {
	var avgFullScore float64
	r.data.db.WithContext(ctx).Raw(`
		SELECT AVG(es.full_score) as avg_full_score
		FROM exam_subjects es
		WHERE es.exam_id = ?
	`, examID).Scan(&avgFullScore)
	return avgFullScore
}
