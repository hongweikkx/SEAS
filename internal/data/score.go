package data

import (
	"context"
	"errors"
	"math"
	"sort"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type classRankRow struct {
	ClassID  int64
	AvgScore float64
}

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

		// 计算全年级总体数据（overall）
		var overallAvg, overallHighest, overallLowest float64
		overallLowest = 99999
		var totalStudentCount int64
		for _, s := range stats.Subjects {
			totalStudentCount += s.StudentCount
			overallAvg += s.AvgScore * float64(s.StudentCount)
			if s.HighestScore > overallHighest {
				overallHighest = s.HighestScore
			}
			if s.LowestScore < overallLowest {
				overallLowest = s.LowestScore
			}
		}
		if totalStudentCount > 0 {
			overallAvg = overallAvg / float64(totalStudentCount)
		}

		// 查询所有学生全科总分用于计算标准差和区分度
		allScores, _ := r.getAllStudentScores(ctx, examID, 0)
		overallFullScore := r.calculateOverallFullScore(ctx, examID)
		overallStdDev := calculateStdDev(allScores)

		stats.Overall = &biz.SubjectStats{
			ID:             0,
			Name:           "全年级",
			FullScore:      overallFullScore,
			AvgScore:       roundTo2Decimal(overallAvg),
			HighestScore:   overallHighest,
			LowestScore:    overallLowest,
			Difficulty:     r.calculateDifficulty(overallAvg, overallFullScore),
			StudentCount:   totalStudentCount,
			ScoreDeviation: 0,
			StdDev:         roundTo2Decimal(overallStdDev),
			Discrimination: calculateDiscrimination(allScores, overallFullScore),
		}

		// 为每个学科计算标准差、离均差和区分度
		for _, subject := range stats.Subjects {
			subjectScores, _ := r.getAllStudentScores(ctx, examID, subject.ID)
			subject.StdDev = roundTo2Decimal(calculateStdDev(subjectScores))
			subject.ScoreDeviation = roundTo2Decimal(subject.AvgScore - overallAvg)
			subject.Discrimination = calculateDiscrimination(subjectScores, subject.FullScore)
		}

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

		// 单科模式也计算标准差和区分度
		allScores, _ := r.getAllStudentScores(ctx, examID, subjectID)
		stats.Subjects[0].StdDev = roundTo2Decimal(calculateStdDev(allScores))
		stats.Subjects[0].Discrimination = calculateDiscrimination(allScores, stats.Subjects[0].FullScore)
		stats.Subjects[0].ScoreDeviation = 0 // 单科模式下无离均差
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

	overallFullScore := r.calculateOverallFullScore(ctx, examID)
	stats.OverallGrade = &biz.ClassStats{
		ClassID:       0,
		ClassName:     "全年级",
		TotalStudents: totalParticipants,
		FullScore:      overallFullScore,
		AvgScore:      math.Round(overallAvg*100) / 100,
		HighestScore:  overallHighest,
		LowestScore:   overallLowest,
		Difficulty:    r.calculateDifficulty(overallAvg, overallFullScore),
		StdDev:        math.Round(overallStdDev*100) / 100,
	}

	// 查询所有学生分数用于计算区分度
	studentScores, err := r.getStudentScores(ctx, examID, subjectID)
	if err != nil {
		log.Context(ctx).Errorf("GetClassSummary getStudentScores err: %+v", err)
	}

	// 计算各班级区分度
	for _, class := range stats.ClassDetails {
		scores := studentScores[class.ClassID]
		class.Discrimination = calculateDiscrimination(scores, fullScore)
		class.FullScore = fullScore
	}
	// 全年级区分度
	allScores := make([]float64, 0)
	for _, scores := range studentScores {
		allScores = append(allScores, scores...)
	}
	stats.OverallGrade.Discrimination = calculateDiscrimination(allScores, overallFullScore)

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

// GetClassSubjectSummary 获取班级学科下钻汇总
func (r *scoreRepo) GetClassSubjectSummary(ctx context.Context, examID, classID int64) (*biz.ClassSubjectSummaryStats, error) {
	var summary biz.ClassSubjectSummaryStats
	summary.ExamID = examID
	summary.ClassID = classID

	var meta struct {
		ExamName  string `gorm:"column:exam_name"`
		ClassName string `gorm:"column:class_name"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			e.name AS exam_name,
			c.name AS class_name
		FROM exams e
		JOIN classes c ON c.id = ?
		WHERE e.id = ?
	`, classID, examID).Scan(&meta).Error; err != nil {
		log.Context(ctx).Errorf("GetClassSubjectSummary meta err: %+v", err)
		return nil, err
	}
	summary.ExamName = meta.ExamName
	summary.ClassName = meta.ClassName

	var subjectRows []struct {
		SubjectID      int64   `gorm:"column:subject_id"`
		SubjectName    string  `gorm:"column:subject_name"`
		FullScore      float64 `gorm:"column:full_score"`
		ClassAvgScore  float64 `gorm:"column:class_avg_score"`
		GradeAvgScore  float64 `gorm:"column:grade_avg_score"`
		ClassHighest   float64 `gorm:"column:class_highest"`
		ClassLowest    float64 `gorm:"column:class_lowest"`
		Participations int64   `gorm:"column:participations"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			s.id AS subject_id,
			s.name AS subject_name,
			es.full_score AS full_score,
			COALESCE(cs.class_avg_score, 0) AS class_avg_score,
			COALESCE(gs.grade_avg_score, 0) AS grade_avg_score,
			COALESCE(cs.class_highest, 0) AS class_highest,
			COALESCE(cs.class_lowest, 0) AS class_lowest,
			COALESCE(cs.participations, 0) AS participations
		FROM exam_subjects es
		JOIN subjects s ON s.id = es.subject_id
		LEFT JOIN (
			SELECT
				sc.subject_id,
				AVG(sc.total_score) AS class_avg_score,
				MAX(sc.total_score) AS class_highest,
				MIN(sc.total_score) AS class_lowest,
				COUNT(sc.student_id) AS participations
			FROM scores sc
			JOIN students st ON st.id = sc.student_id
			WHERE sc.exam_id = ? AND st.class_id = ?
			GROUP BY sc.subject_id
		) cs ON cs.subject_id = es.subject_id
		LEFT JOIN (
			SELECT
				sc.subject_id,
				AVG(sc.total_score) AS grade_avg_score
			FROM scores sc
			WHERE sc.exam_id = ?
			GROUP BY sc.subject_id
		) gs ON gs.subject_id = es.subject_id
		WHERE es.exam_id = ?
		ORDER BY s.id
	`, examID, classID, examID, examID).Scan(&subjectRows).Error; err != nil {
		log.Context(ctx).Errorf("GetClassSubjectSummary subject rows err: %+v", err)
		return nil, err
	}

	var subjectRanks []struct {
		SubjectID int64   `gorm:"column:subject_id"`
		ClassID   int64   `gorm:"column:class_id"`
		AvgScore  float64 `gorm:"column:avg_score"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			sc.subject_id AS subject_id,
			st.class_id AS class_id,
			AVG(sc.total_score) AS avg_score
		FROM scores sc
		JOIN students st ON st.id = sc.student_id
		WHERE sc.exam_id = ?
		GROUP BY sc.subject_id, st.class_id
	`, examID).Scan(&subjectRanks).Error; err != nil {
		log.Context(ctx).Errorf("GetClassSubjectSummary subject ranks err: %+v", err)
		return nil, err
	}

	ranksBySubject := make(map[int64]map[int64]int32)
	totalClassesBySubject := make(map[int64]int32)
	for _, rankRow := range subjectRanks {
		if _, ok := ranksBySubject[rankRow.SubjectID]; !ok {
			ranksBySubject[rankRow.SubjectID] = make(map[int64]int32)
		}
		totalClassesBySubject[rankRow.SubjectID]++
	}
	for subjectID := range ranksBySubject {
		ordered := make([]struct {
			classID  int64
			avgScore float64
		}, 0)
		for _, rankRow := range subjectRanks {
			if rankRow.SubjectID == subjectID {
				ordered = append(ordered, struct {
					classID  int64
					avgScore float64
				}{
					classID:  rankRow.ClassID,
					avgScore: rankRow.AvgScore,
				})
			}
		}
		for i := 0; i < len(ordered); i++ {
			best := i
			for j := i + 1; j < len(ordered); j++ {
				if ordered[j].avgScore > ordered[best].avgScore || (ordered[j].avgScore == ordered[best].avgScore && ordered[j].classID < ordered[best].classID) {
					best = j
				}
			}
			ordered[i], ordered[best] = ordered[best], ordered[i]
		}
		for idx, item := range ordered {
			ranksBySubject[subjectID][item.classID] = int32(idx + 1)
		}
	}

	summary.Subjects = make([]*biz.ClassSubjectItemStats, 0, len(subjectRows))
	for _, row := range subjectRows {
		item := &biz.ClassSubjectItemStats{
			SubjectID:     row.SubjectID,
			SubjectName:   row.SubjectName,
			FullScore:     roundTo2Decimal(row.FullScore),
			ClassAvgScore: roundTo2Decimal(row.ClassAvgScore),
			GradeAvgScore: roundTo2Decimal(row.GradeAvgScore),
			ScoreDiff:     roundTo2Decimal(row.ClassAvgScore - row.GradeAvgScore),
			ClassHighest:  roundTo2Decimal(row.ClassHighest),
			ClassLowest:   roundTo2Decimal(row.ClassLowest),
			ClassRank:     ranksBySubject[row.SubjectID][classID],
			TotalClasses:  totalClassesBySubject[row.SubjectID],
		}
		summary.Subjects = append(summary.Subjects, item)
	}

	var overall struct {
		FullScore     float64 `gorm:"column:full_score"`
		ClassAvgScore float64 `gorm:"column:class_avg_score"`
		GradeAvgScore float64 `gorm:"column:grade_avg_score"`
		ClassHighest  float64 `gorm:"column:class_highest"`
		ClassLowest   float64 `gorm:"column:class_lowest"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(fs.full_score, 0) AS full_score,
			COALESCE(ca.class_avg_score, 0) AS class_avg_score,
			COALESCE(ga.grade_avg_score, 0) AS grade_avg_score,
			COALESCE(ca.class_highest, 0) AS class_highest,
			COALESCE(ca.class_lowest, 0) AS class_lowest
		FROM (
			SELECT SUM(es.full_score) AS full_score
			FROM exam_subjects es
			WHERE es.exam_id = ?
		) fs
		LEFT JOIN (
			SELECT
				AVG(student_total) AS class_avg_score,
				MAX(student_total) AS class_highest,
				MIN(student_total) AS class_lowest
			FROM (
				SELECT
					sc.student_id,
					SUM(sc.total_score) AS student_total
				FROM scores sc
				JOIN students st ON st.id = sc.student_id
				WHERE sc.exam_id = ? AND st.class_id = ?
				GROUP BY sc.student_id
			) class_totals
		) ca ON 1 = 1
		LEFT JOIN (
			SELECT AVG(student_total) AS grade_avg_score
			FROM (
				SELECT
					sc.student_id,
					SUM(sc.total_score) AS student_total
				FROM scores sc
				WHERE sc.exam_id = ?
				GROUP BY sc.student_id
			) grade_totals
		) ga ON 1 = 1
	`, examID, examID, classID, examID).Scan(&overall).Error; err != nil {
		log.Context(ctx).Errorf("GetClassSubjectSummary overall err: %+v", err)
		return nil, err
	}

	var overallRanks []struct {
		ClassID  int64   `gorm:"column:class_id"`
		AvgScore float64 `gorm:"column:avg_score"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			class_totals.class_id AS class_id,
			AVG(class_totals.student_total) AS avg_score
		FROM (
			SELECT
				st.class_id AS class_id,
				sc.student_id AS student_id,
				SUM(sc.total_score) AS student_total
			FROM scores sc
			JOIN students st ON st.id = sc.student_id
			WHERE sc.exam_id = ?
			GROUP BY st.class_id, sc.student_id
		) class_totals
		GROUP BY class_totals.class_id
	`, examID).Scan(&overallRanks).Error; err != nil {
		log.Context(ctx).Errorf("GetClassSubjectSummary overall ranks err: %+v", err)
		return nil, err
	}

	overallRank := rankByClassID(overallRanks, classID)
	summary.Overall = &biz.ClassSubjectItemStats{
		SubjectID:     0,
		SubjectName:   "总分",
		FullScore:     roundTo2Decimal(overall.FullScore),
		ClassAvgScore: roundTo2Decimal(overall.ClassAvgScore),
		GradeAvgScore: roundTo2Decimal(overall.GradeAvgScore),
		ScoreDiff:     roundTo2Decimal(overall.ClassAvgScore - overall.GradeAvgScore),
		ClassHighest:  roundTo2Decimal(overall.ClassHighest),
		ClassLowest:   roundTo2Decimal(overall.ClassLowest),
		ClassRank:     overallRank,
		TotalClasses:  int32(len(overallRanks)),
	}

	return &summary, nil
}

// BatchCreate 批量创建成绩记录
func (r *scoreRepo) BatchCreate(ctx context.Context, scores []*biz.Score) error {
	if len(scores) == 0 {
		return nil
	}
	return r.data.db.WithContext(ctx).CreateInBatches(scores, 100).Error
}

// GetSingleClassSummary 获取单科学科下班级汇总
func (r *scoreRepo) GetSingleClassSummary(ctx context.Context, examID, subjectID int64) (*biz.SingleClassSummaryStats, error) {
	var summary biz.SingleClassSummaryStats
	summary.ExamID = examID
	summary.SubjectID = subjectID

	var meta struct {
		ExamName    string `gorm:"column:exam_name"`
		SubjectName string `gorm:"column:subject_name"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT e.name AS exam_name, s.name AS subject_name
		FROM exams e JOIN subjects s ON s.id = ?
		WHERE e.id = ?
	`, subjectID, examID).Scan(&meta).Error; err != nil {
		return nil, err
	}
	summary.ExamName = meta.ExamName
	summary.SubjectName = meta.SubjectName

	var fullScore float64
	r.data.db.WithContext(ctx).Raw(`
		SELECT full_score FROM exam_subjects WHERE exam_id = ? AND subject_id = ?
	`, examID, subjectID).Scan(&fullScore)

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
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			c.id AS class_id,
			c.name AS class_name,
			COUNT(sc.student_id) AS total_students,
			ROUND(AVG(sc.total_score), 2) AS avg_score,
			MAX(sc.total_score) AS highest_score,
			MIN(sc.total_score) AS lowest_score,
			ROUND(STDDEV_POP(sc.total_score), 2) AS std_dev
		FROM classes c
		JOIN students st ON st.class_id = c.id
		JOIN scores sc ON sc.student_id = st.id AND sc.exam_id = ? AND sc.subject_id = ?
		GROUP BY c.id, c.name
		ORDER BY c.id
	`, examID, subjectID).Scan(&classStats).Error; err != nil {
		return nil, err
	}

	// 查询年级总体统计
	var gradeOverall struct {
		TotalStudents int64   `gorm:"column:total_students"`
		AvgScore      float64 `gorm:"column:avg_score"`
		HighestScore  float64 `gorm:"column:highest_score"`
		LowestScore   float64 `gorm:"column:lowest_score"`
		StdDev        float64 `gorm:"column:std_dev"`
	}
	if err := r.data.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(sc.student_id) AS total_students,
			ROUND(AVG(sc.total_score), 2) AS avg_score,
			MAX(sc.total_score) AS highest_score,
			MIN(sc.total_score) AS lowest_score,
			ROUND(STDDEV_POP(sc.total_score), 2) AS std_dev
		FROM scores sc
		WHERE sc.exam_id = ? AND sc.subject_id = ?
	`, examID, subjectID).Scan(&gradeOverall).Error; err != nil {
		return nil, err
	}

	// 查询学生分数用于计算区分度
	studentScores, _ := r.getStudentScores(ctx, examID, subjectID)
	allScores := make([]float64, 0)
	for _, scores := range studentScores {
		allScores = append(allScores, scores...)
	}

	// 构建各班明细
	summary.Classes = make([]*biz.ClassStats, 0, len(classStats))
	for _, row := range classStats {
		summary.Classes = append(summary.Classes, &biz.ClassStats{
			ClassID:        row.ClassID,
			ClassName:      row.ClassName,
			TotalStudents:  row.TotalStudents,
			FullScore:      fullScore,
			AvgScore:       row.AvgScore,
			HighestScore:   row.HighestScore,
			LowestScore:    row.LowestScore,
			ScoreDeviation: roundTo2Decimal(row.AvgScore - gradeOverall.AvgScore),
			Difficulty:     r.calculateDifficulty(row.AvgScore, fullScore),
			StdDev:         row.StdDev,
			Discrimination: calculateDiscrimination(studentScores[row.ClassID], fullScore),
		})
	}

	// 全年级总体行
	summary.Overall = &biz.ClassStats{
		ClassID:        0,
		ClassName:      "全年级",
		TotalStudents:  gradeOverall.TotalStudents,
		FullScore:      fullScore,
		AvgScore:       gradeOverall.AvgScore,
		HighestScore:   gradeOverall.HighestScore,
		LowestScore:    gradeOverall.LowestScore,
		ScoreDeviation: 0,
		Difficulty:     r.calculateDifficulty(gradeOverall.AvgScore, fullScore),
		StdDev:         gradeOverall.StdDev,
		Discrimination: calculateDiscrimination(allScores, fullScore),
	}

	return &summary, nil
}

// calculateDifficulty 计算难度 = 平均分/满分 * 100
func (r *scoreRepo) calculateDifficulty(avgScore, fullScore float64) float64 {
	if fullScore == 0 {
		return 0
	}
	return math.Round(avgScore/fullScore*100*100) / 100
}

func roundTo2Decimal(value float64) float64 {
	return math.Round(value*100) / 100
}

func rankByClassID(rows []struct {
	ClassID  int64   `gorm:"column:class_id"`
	AvgScore float64 `gorm:"column:avg_score"`
}, classID int64) int32 {
	ordered := make([]classRankRow, 0, len(rows))
	for _, row := range rows {
		ordered = append(ordered, classRankRow{
			ClassID:  row.ClassID,
			AvgScore: row.AvgScore,
		})
	}
	orderClassRanks(ordered)
	for idx, row := range ordered {
		if row.ClassID == classID {
			return int32(idx + 1)
		}
	}
	return 0
}

func orderClassRanks(rows []classRankRow) {
	for i := 0; i < len(rows); i++ {
		best := i
		for j := i + 1; j < len(rows); j++ {
			if rows[j].AvgScore > rows[best].AvgScore || (rows[j].AvgScore == rows[best].AvgScore && rows[j].ClassID < rows[best].ClassID) {
				best = j
			}
		}
		rows[i], rows[best] = rows[best], rows[i]
	}
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

// calculateDiscrimination 计算区分度：高低分组得分率之差
func calculateDiscrimination(scores []float64, fullScore float64) float64 {
	if len(scores) == 0 || fullScore == 0 {
		return 0
	}
	sort.Float64s(scores)
	n := len(scores)
	groupSize := int(math.Max(1, math.Round(float64(n)*0.27)))

	var lowSum, highSum float64
	for i := 0; i < groupSize; i++ {
		lowSum += scores[i]
	}
	for i := n - groupSize; i < n; i++ {
		highSum += scores[i]
	}

	lowRate := (lowSum / float64(groupSize)) / fullScore
	highRate := (highSum / float64(groupSize)) / fullScore
	return roundTo2Decimal((highRate - lowRate) * 100)
}

// calculateStdDev 计算标准差
func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	var variance float64
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	return math.Sqrt(variance / float64(len(values)))
}

// getStudentScores 获取指定考试和学科下所有学生的分数（用于计算区分度）
// 全科模式下返回每个学生的总分（按班级分组）；单科模式下返回该科分数
func (r *scoreRepo) getStudentScores(ctx context.Context, examID, subjectID int64) (map[int64][]float64, error) {
	var rows []struct {
		ClassID int64   `gorm:"column:class_id"`
		Score   float64 `gorm:"column:total_score"`
	}

	var err error
	if subjectID > 0 {
		err = r.data.db.WithContext(ctx).Raw(`
			SELECT st.class_id, sc.total_score
			FROM scores sc
			JOIN students st ON st.id = sc.student_id
			WHERE sc.exam_id = ? AND sc.subject_id = ?
			ORDER BY st.class_id, sc.total_score
		`, examID, subjectID).Scan(&rows).Error
	} else {
		err = r.data.db.WithContext(ctx).Raw(`
			SELECT st.class_id, SUM(sc.total_score) as total_score
			FROM scores sc
			JOIN students st ON st.id = sc.student_id
			WHERE sc.exam_id = ?
			GROUP BY st.class_id, sc.student_id
			ORDER BY st.class_id, total_score
		`, examID).Scan(&rows).Error
	}
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]float64)
	for _, row := range rows {
		result[row.ClassID] = append(result[row.ClassID], row.Score)
	}
	return result, nil
}

// getAllStudentScores 获取所有学生分数（不按班级分组，用于学科/全年级区分度）
// 全科模式下返回每个学生的总分；单科模式下返回该科分数
func (r *scoreRepo) getAllStudentScores(ctx context.Context, examID, subjectID int64) ([]float64, error) {
	var scores []float64
	var err error

	if subjectID > 0 {
		err = r.data.db.WithContext(ctx).Raw(`
			SELECT total_score FROM scores WHERE exam_id = ? AND subject_id = ? ORDER BY total_score
		`, examID, subjectID).Pluck("total_score", &scores).Error
	} else {
		err = r.data.db.WithContext(ctx).Raw(`
			SELECT SUM(total_score) as total_score
			FROM scores
			WHERE exam_id = ?
			GROUP BY student_id
			ORDER BY total_score
		`, examID).Pluck("total_score", &scores).Error
	}
	if err != nil {
		return nil, err
	}
	return scores, nil
}
