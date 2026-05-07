package biz

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestQuestionNumberLess(t *testing.T) {
	questions := []string{"10", "2", "1"}

	sort.Slice(questions, func(i, j int) bool {
		return questionNumberLess(questions[i], questions[j])
	})

	want := []string{"1", "2", "10"}
	if !reflect.DeepEqual(questions, want) {
		t.Fatalf("sorted questions = %v, want %v", questions, want)
	}
}

func TestDifficultyFromScoreRate(t *testing.T) {
	cases := []struct {
		name      string
		scoreRate float64
		want      float64
	}{
		{name: "easy at threshold", scoreRate: 80, want: 80},
		{name: "medium at threshold", scoreRate: 60, want: 60},
		{name: "hard below medium", scoreRate: 59.99, want: 59.99},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := difficultyFromScoreRate(tc.scoreRate); got != tc.want {
				t.Fatalf("difficultyFromScoreRate(%v) = %v, want %v", tc.scoreRate, got, tc.want)
			}
		})
	}
}

func TestAssignSequentialRanks(t *testing.T) {
	students := []*StudentQuestionDetailStats{
		{StudentID: 1001, Score: 5},
		{StudentID: 1002, Score: 3},
		{StudentID: 1003, Score: 1},
	}

	assignSequentialRanks(students, func(item *StudentQuestionDetailStats, rank int32) {
		item.ClassRank = rank
	})

	got := []int32{students[0].ClassRank, students[1].ClassRank, students[2].ClassRank}
	want := []int32{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("assigned ranks = %v, want %v", got, want)
	}
}

func TestNormalizeQuestionID(t *testing.T) {
	if got := normalizeQuestionID("repo-id", "2"); got != "2" {
		t.Fatalf("normalizeQuestionID() = %q, want %q", got, "2")
	}
}

func TestQuestionLevelMethodsRequireScoreItemRepo(t *testing.T) {
	uc := NewExamAnalysisUseCase(nil, nil, nil)

	t.Run("single class questions", func(t *testing.T) {
		_, err := uc.GetSingleClassQuestions(context.Background(), 1, 2, 3)
		if err == nil {
			t.Fatal("expected error when score item repo is not configured")
		}
	})

	t.Run("single question summary", func(t *testing.T) {
		_, err := uc.GetSingleQuestionSummary(context.Background(), 1, 2)
		if err == nil {
			t.Fatal("expected error when score item repo is not configured")
		}
	})

	t.Run("single question detail", func(t *testing.T) {
		_, err := uc.GetSingleQuestionDetail(context.Background(), 1, 2, 3, "1")
		if err == nil {
			t.Fatal("expected error when score item repo is not configured")
		}
	})
}

func TestGetSingleQuestionDetailAssignsClassRankAfterSorting(t *testing.T) {
	repo := &stubScoreItemRepo{
		singleQuestionDetail: &SingleQuestionDetailStats{
			QuestionID:     "repo-id",
			QuestionNumber: "2",
			Students: []*StudentQuestionDetailStats{
				{StudentID: 1002, Score: 3},
				{StudentID: 1001, Score: 5},
				{StudentID: 1003, Score: 1},
			},
		},
	}
	uc := NewExamAnalysisUseCase(nil, nil, nil).WithScoreItemRepo(repo)

	stats, err := uc.GetSingleQuestionDetail(context.Background(), 1, 2, 3, "2")
	if err != nil {
		t.Fatalf("GetSingleQuestionDetail() error = %v", err)
	}

	if stats.QuestionID != "2" {
		t.Fatalf("QuestionID = %q, want %q", stats.QuestionID, "2")
	}

	gotStudents := []int64{stats.Students[0].StudentID, stats.Students[1].StudentID, stats.Students[2].StudentID}
	wantStudents := []int64{1001, 1002, 1003}
	if !reflect.DeepEqual(gotStudents, wantStudents) {
		t.Fatalf("students order = %v, want %v", gotStudents, wantStudents)
	}

	gotRanks := []int32{stats.Students[0].ClassRank, stats.Students[1].ClassRank, stats.Students[2].ClassRank}
	wantRanks := []int32{1, 2, 3}
	if !reflect.DeepEqual(gotRanks, wantRanks) {
		t.Fatalf("class ranks = %v, want %v", gotRanks, wantRanks)
	}
}

type stubScoreItemRepo struct {
	singleClassQuestions       *SingleClassQuestionStats
	singleQuestionSummary      *SingleQuestionSummaryStats
	singleQuestionDetail       *SingleQuestionDetailStats
	singleQuestionClassCompare *SingleQuestionClassCompareStats
	err                        error
}

func (s *stubScoreItemRepo) ListByScoreID(context.Context, int64) ([]*ScoreItem, error) {
	return nil, s.err
}

func (s *stubScoreItemRepo) GetSingleClassQuestions(context.Context, int64, int64, int64) (*SingleClassQuestionStats, error) {
	return s.singleClassQuestions, s.err
}

func (s *stubScoreItemRepo) GetSingleQuestionSummary(context.Context, int64, int64) (*SingleQuestionSummaryStats, error) {
	return s.singleQuestionSummary, s.err
}

func (s *stubScoreItemRepo) GetSingleQuestionDetail(context.Context, int64, int64, int64, string) (*SingleQuestionDetailStats, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.singleQuestionDetail == nil {
		return nil, errors.New("missing detail stats")
	}
	return s.singleQuestionDetail, nil
}

func TestGetSingleQuestionClassCompare(t *testing.T) {
	uc := NewExamAnalysisUseCase(nil, nil, nil)
	uc.WithScoreItemRepo(&stubScoreItemRepo{
		singleQuestionClassCompare: &SingleQuestionClassCompareStats{
			ExamID:     1,
			SubjectID:  1,
			QuestionID: "Q1",
			FullScore:  10,
			Overall: &SingleQuestionClassCompareItemStats{
				ClassID:      0,
				ClassName:    "全年级",
				Participants: 150,
				AvgScore:     7.2,
				ScoreRate:    72,
				ScoreDiff:    0,
				ClassRank:    0,
				TotalClasses: 3,
				HighestScore: 10,
				LowestScore:  0,
				StdDev:       1.45,
			},
			Classes: []*SingleQuestionClassCompareItemStats{
				{ClassID: 1, ClassName: "一班", Participants: 50, AvgScore: 7.5, ScoreRate: 75, ScoreDiff: 0.3, ClassRank: 1, TotalClasses: 3, HighestScore: 10, LowestScore: 2, StdDev: 1.32},
				{ClassID: 2, ClassName: "二班", Participants: 49, AvgScore: 7.0, ScoreRate: 70, ScoreDiff: -0.2, ClassRank: 2, TotalClasses: 3, HighestScore: 10, LowestScore: 1, StdDev: 1.51},
				{ClassID: 3, ClassName: "三班", Participants: 51, AvgScore: 5.1, ScoreRate: 51, ScoreDiff: -2.1, ClassRank: 3, TotalClasses: 3, HighestScore: 9, LowestScore: 0, StdDev: 1.78},
			},
		},
	})

	stats, err := uc.GetSingleQuestionClassCompare(context.Background(), 1, 1, "Q1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Overall.ClassName != "全年级" {
		t.Errorf("overall class name = %s, want 全年级", stats.Overall.ClassName)
	}
	if stats.Overall.ScoreDiff != 0 {
		t.Errorf("overall score diff = %f, want 0", stats.Overall.ScoreDiff)
	}
	if len(stats.Classes) != 3 {
		t.Fatalf("classes count = %d, want 3", len(stats.Classes))
	}
	if stats.Classes[0].ClassRank != 1 || stats.Classes[0].ClassName != "一班" {
		t.Errorf("first class rank = %d name = %s, want 1/一班", stats.Classes[0].ClassRank, stats.Classes[0].ClassName)
	}
	if stats.Classes[2].ClassRank != 3 || stats.Classes[2].ScoreDiff != -2.1 {
		t.Errorf("last class rank = %d diff = %f, want 3/-2.1", stats.Classes[2].ClassRank, stats.Classes[2].ScoreDiff)
	}
}

func TestGetSingleQuestionDetailWithClassIDZero(t *testing.T) {
	uc := NewExamAnalysisUseCase(nil, nil, nil)
	uc.WithScoreItemRepo(&stubScoreItemRepo{
		singleQuestionDetail: &SingleQuestionDetailStats{
			ExamID:     1,
			SubjectID:  1,
			ClassID:    0,
			QuestionID: "Q1",
			FullScore:  10,
			Students: []*StudentQuestionDetailStats{
				{StudentID: 1, StudentName: "A", Score: 9, FullScore: 10},
				{StudentID: 2, StudentName: "B", Score: 8, FullScore: 10},
				{StudentID: 3, StudentName: "C", Score: 7, FullScore: 10},
			},
		},
	})

	stats, err := uc.GetSingleQuestionDetail(context.Background(), 1, 1, 0, "Q1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.ClassName != "全年级" {
		t.Errorf("class name = %s, want 全年级", stats.ClassName)
	}
	if len(stats.Students) != 3 {
		t.Fatalf("students count = %d, want 3", len(stats.Students))
	}
	if stats.Students[0].ClassRank != 1 {
		t.Errorf("first student class rank = %d, want 1", stats.Students[0].ClassRank)
	}
	if stats.Students[0].GradeRank != 1 {
		t.Errorf("first student grade rank = %d, want 1", stats.Students[0].GradeRank)
	}
}

func (s *stubScoreItemRepo) GetSingleQuestionClassCompare(context.Context, int64, int64, string) (*SingleQuestionClassCompareStats, error) {
	return s.singleQuestionClassCompare, s.err
}

func (s *stubScoreItemRepo) BatchCreate(context.Context, []*ScoreItem) error {
	return s.err
}
