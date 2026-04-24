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
		want      string
	}{
		{name: "easy at threshold", scoreRate: 80, want: "easy"},
		{name: "medium at threshold", scoreRate: 60, want: "medium"},
		{name: "hard below medium", scoreRate: 59.99, want: "hard"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := difficultyFromScoreRate(tc.scoreRate); got != tc.want {
				t.Fatalf("difficultyFromScoreRate(%v) = %q, want %q", tc.scoreRate, got, tc.want)
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
	singleClassQuestions  *SingleClassQuestionStats
	singleQuestionSummary *SingleQuestionSummaryStats
	singleQuestionDetail  *SingleQuestionDetailStats
	err                   error
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
