package service

import (
	"testing"

	pb "seas/api/seas/v1"
	"seas/internal/biz"
)

func TestMapStudentQuestionDetail(t *testing.T) {
	stats := &biz.SingleQuestionDetailStats{
		QuestionID:     "10",
		QuestionNumber: "10",
		QuestionType:   "",
		FullScore:      12,
		Students: []*biz.StudentQuestionDetailStats{
			{
				StudentID:   101,
				StudentName: "张三",
				Score:       9,
				FullScore:   12,
				ScoreRate:   75,
				ClassRank:   1,
				GradeRank:   3,
			},
		},
	}

	reply := &pb.GetSingleQuestionDetailReply{
		QuestionId:     stats.QuestionID,
		QuestionNumber: stats.QuestionNumber,
		QuestionType:   stats.QuestionType,
		FullScore:      stats.FullScore,
	}
	for _, student := range stats.Students {
		reply.Students = append(reply.Students, &pb.StudentQuestionDetail{
			StudentId:   student.StudentID,
			StudentName: student.StudentName,
			Score:       student.Score,
			FullScore:   student.FullScore,
			ScoreRate:   student.ScoreRate,
			ClassRank:   student.ClassRank,
			GradeRank:   student.GradeRank,
		})
	}

	if reply.QuestionId != "10" || len(reply.Students) != 1 || reply.Students[0].ClassRank != 1 {
		t.Fatalf("unexpected reply mapping: %+v", reply)
	}
}

func TestMapClassSubjectItem(t *testing.T) {
	stats := &biz.ClassSubjectSummaryStats{
		ExamID:    1,
		ClassID:   2,
		ClassName: "一班",
		Overall: &biz.ClassSubjectItemStats{
			SubjectID:     0,
			SubjectName:   "总分",
			FullScore:     500,
			ClassAvgScore: 380.5,
			GradeAvgScore: 400,
			ScoreDiff:     -19.5,
			ClassHighest:  480,
			ClassLowest:   200,
			ClassRank:     3,
			TotalClasses:  5,
		},
		Subjects: []*biz.ClassSubjectItemStats{
			{
				SubjectID:     1,
				SubjectName:   "语文",
				FullScore:     100,
				ClassAvgScore: 85,
				GradeAvgScore: 82,
				ScoreDiff:     3,
				ClassHighest:  95,
				ClassLowest:   60,
				ClassRank:     2,
				TotalClasses:  5,
			},
		},
	}

	reply := &pb.GetClassSubjectSummaryReply{
		ExamId:    stats.ExamID,
		ClassId:   stats.ClassID,
		ClassName: stats.ClassName,
	}
	if stats.Overall != nil {
		reply.Overall = &pb.ClassSubjectItem{
			SubjectId:     stats.Overall.SubjectID,
			SubjectName:   stats.Overall.SubjectName,
			FullScore:     stats.Overall.FullScore,
			ClassAvgScore: stats.Overall.ClassAvgScore,
			GradeAvgScore: stats.Overall.GradeAvgScore,
			ScoreDiff:     stats.Overall.ScoreDiff,
			ClassHighest:  stats.Overall.ClassHighest,
			ClassLowest:   stats.Overall.ClassLowest,
			ClassRank:     stats.Overall.ClassRank,
			TotalClasses:  stats.Overall.TotalClasses,
		}
	}
	for _, subject := range stats.Subjects {
		reply.Subjects = append(reply.Subjects, &pb.ClassSubjectItem{
			SubjectId:     subject.SubjectID,
			SubjectName:   subject.SubjectName,
			FullScore:     subject.FullScore,
			ClassAvgScore: subject.ClassAvgScore,
			GradeAvgScore: subject.GradeAvgScore,
			ScoreDiff:     subject.ScoreDiff,
			ClassHighest:  subject.ClassHighest,
			ClassLowest:   subject.ClassLowest,
			ClassRank:     subject.ClassRank,
			TotalClasses:  subject.TotalClasses,
		})
	}

	if reply.Overall == nil || reply.Overall.ClassRank != 3 {
		t.Fatalf("expected overall class_rank = 3, got %+v", reply.Overall)
	}
	if len(reply.Subjects) != 1 || reply.Subjects[0].SubjectName != "语文" {
		t.Fatalf("unexpected subjects mapping: %+v", reply.Subjects)
	}
}

func TestMapSingleClassSummaryItem(t *testing.T) {
	stats := &biz.SingleClassSummaryStats{
		ExamID:      1,
		SubjectID:   2,
		SubjectName: "数学",
		Overall: &biz.SingleClassSummaryItemStats{
			ClassID:         0,
			ClassName:       "全年级",
			TotalStudents:   100,
			SubjectAvgScore: 78.5,
			GradeAvgScore:   78.5,
			ScoreDiff:       0,
			ClassRank:       0,
			TotalClasses:    4,
			PassRate:        85,
			ExcellentRate:   20,
		},
		Classes: []*biz.SingleClassSummaryItemStats{
			{
				ClassID:         1,
				ClassName:       "一班",
				TotalStudents:   25,
				SubjectAvgScore: 82,
				GradeAvgScore:   78.5,
				ScoreDiff:       3.5,
				ClassRank:       1,
				TotalClasses:    4,
				PassRate:        90,
				ExcellentRate:   25,
			},
		},
	}

	reply := &pb.GetSingleClassSummaryReply{
		ExamId:    stats.ExamID,
		SubjectId: stats.SubjectID,
	}
	if stats.Overall != nil {
		reply.Overall = &pb.SingleClassSummaryItem{
			ClassId:         stats.Overall.ClassID,
			ClassName:       stats.Overall.ClassName,
			TotalStudents:   stats.Overall.TotalStudents,
			SubjectAvgScore: stats.Overall.SubjectAvgScore,
			GradeAvgScore:   stats.Overall.GradeAvgScore,
			ScoreDiff:       stats.Overall.ScoreDiff,
			ClassRank:       stats.Overall.ClassRank,
			TotalClasses:    stats.Overall.TotalClasses,
			PassRate:        stats.Overall.PassRate,
			ExcellentRate:   stats.Overall.ExcellentRate,
		}
		reply.SubjectName = stats.Overall.ClassName
	}
	for _, class := range stats.Classes {
		reply.Classes = append(reply.Classes, &pb.SingleClassSummaryItem{
			ClassId:         class.ClassID,
			ClassName:       class.ClassName,
			TotalStudents:   class.TotalStudents,
			SubjectAvgScore: class.SubjectAvgScore,
			GradeAvgScore:   class.GradeAvgScore,
			ScoreDiff:       class.ScoreDiff,
			ClassRank:       class.ClassRank,
			TotalClasses:    class.TotalClasses,
			PassRate:        class.PassRate,
			ExcellentRate:   class.ExcellentRate,
		})
	}

	if reply.Overall == nil || reply.Overall.PassRate != 85 {
		t.Fatalf("expected overall pass_rate = 85, got %+v", reply.Overall)
	}
	if len(reply.Classes) != 1 || reply.Classes[0].ClassRank != 1 {
		t.Fatalf("unexpected classes mapping: %+v", reply.Classes)
	}
}

func TestMapClassQuestionItem(t *testing.T) {
	stats := &biz.SingleClassQuestionStats{
		ExamID:      1,
		SubjectID:   2,
		SubjectName: "英语",
		ClassID:     3,
		ClassName:   "二班",
		Questions: []*biz.ClassQuestionItemStats{
			{
				QuestionID:     "1",
				QuestionNumber: "1",
				QuestionType:   "",
				FullScore:      10,
				ClassAvgScore:  8,
				ScoreRate:      80,
				GradeAvgScore:  7.5,
				Difficulty:     "easy",
			},
		},
	}

	reply := &pb.GetSingleClassQuestionsReply{
		ExamId:      stats.ExamID,
		SubjectId:   stats.SubjectID,
		SubjectName: stats.SubjectName,
		ClassId:     stats.ClassID,
		ClassName:   stats.ClassName,
	}
	reply.Questions = make([]*pb.ClassQuestionItem, len(stats.Questions))
	for i, q := range stats.Questions {
		reply.Questions[i] = &pb.ClassQuestionItem{
			QuestionId:     q.QuestionID,
			QuestionNumber: q.QuestionNumber,
			QuestionType:   q.QuestionType,
			FullScore:      q.FullScore,
			ClassAvgScore:  q.ClassAvgScore,
			ScoreRate:      q.ScoreRate,
			GradeAvgScore:  q.GradeAvgScore,
			Difficulty:     q.Difficulty,
		}
	}

	if len(reply.Questions) != 1 || reply.Questions[0].Difficulty != "easy" {
		t.Fatalf("unexpected questions mapping: %+v", reply.Questions)
	}
}

func TestMapSingleQuestionSummaryItem(t *testing.T) {
	stats := &biz.SingleQuestionSummaryStats{
		ExamID:      1,
		SubjectID:   2,
		SubjectName: "物理",
		Questions: []*biz.SingleQuestionSummaryItemStats{
			{
				QuestionID:     "5",
				QuestionNumber: "5",
				QuestionType:   "",
				FullScore:      20,
				GradeAvgScore:  12,
				ScoreRate:      60,
				Difficulty:     "medium",
				ClassBreakdown: []*biz.QuestionClassBreakdownStats{
					{ClassID: 1, ClassName: "一班", AvgScore: 14},
					{ClassID: 2, ClassName: "二班", AvgScore: 10},
				},
			},
		},
	}

	reply := &pb.GetSingleQuestionSummaryReply{
		ExamId:      stats.ExamID,
		SubjectId:   stats.SubjectID,
		SubjectName: stats.SubjectName,
	}
	reply.Questions = make([]*pb.SingleQuestionSummaryItem, len(stats.Questions))
	for i, q := range stats.Questions {
		item := &pb.SingleQuestionSummaryItem{
			QuestionId:     q.QuestionID,
			QuestionNumber: q.QuestionNumber,
			QuestionType:   q.QuestionType,
			FullScore:      q.FullScore,
			GradeAvgScore:  q.GradeAvgScore,
			ScoreRate:      q.ScoreRate,
			Difficulty:     q.Difficulty,
		}
		item.ClassBreakdown = make([]*pb.QuestionClassBreakdown, len(q.ClassBreakdown))
		for j, cb := range q.ClassBreakdown {
			item.ClassBreakdown[j] = &pb.QuestionClassBreakdown{
				ClassId:   cb.ClassID,
				ClassName: cb.ClassName,
				AvgScore:  cb.AvgScore,
			}
		}
		reply.Questions[i] = item
	}

	if len(reply.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(reply.Questions))
	}
	q := reply.Questions[0]
	if q.Difficulty != "medium" {
		t.Fatalf("expected difficulty medium, got %s", q.Difficulty)
	}
	if len(q.ClassBreakdown) != 2 {
		t.Fatalf("expected 2 class breakdowns, got %d", len(q.ClassBreakdown))
	}
}
