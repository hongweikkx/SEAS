package biz

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/xuri/excelize/v2"
)

// ImportResult 导入结果
type ImportResult struct {
	ImportedStudents int
	ImportedSubjects int
	Mode             string
	Warnings         []string
}

// ExamImportUseCase 考试导入业务逻辑
type ExamImportUseCase struct {
	examRepo      ExamRepo
	classRepo     ClassRepo
	studentRepo   StudentRepo
	subjectRepo   SubjectRepo
	scoreRepo     ScoreRepo
	scoreItemRepo ScoreItemRepo
	log           *log.Helper
}

// NewExamImportUseCase 创建导入用例
func NewExamImportUseCase(
	examRepo ExamRepo,
	classRepo ClassRepo,
	studentRepo StudentRepo,
	subjectRepo SubjectRepo,
	scoreRepo ScoreRepo,
	scoreItemRepo ScoreItemRepo,
	logger log.Logger,
) *ExamImportUseCase {
	return &ExamImportUseCase{
		examRepo:      examRepo,
		classRepo:     classRepo,
		studentRepo:   studentRepo,
		subjectRepo:   subjectRepo,
		scoreRepo:     scoreRepo,
		scoreItemRepo: scoreItemRepo,
		log:           log.NewHelper(logger),
	}
}

// CreateExam 创建考试
func (uc *ExamImportUseCase) CreateExam(ctx context.Context, name string, examDate time.Time) (int64, error) {
	exam := &Exam{
		Name:     name,
		ExamDate: examDate,
	}
	if err := uc.examRepo.Create(ctx, exam); err != nil {
		return 0, err
	}
	return exam.ID, nil
}

// ImportScoresFromExcel 从 Excel 导入成绩
func (uc *ExamImportUseCase) ImportScoresFromExcel(ctx context.Context, examID int64, filePath string) (*ImportResult, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("open excel failed: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found")
	}

	result := &ImportResult{
		Mode:     "simple",
		Warnings: []string{},
	}

	// 确定总成绩 sheet
	summarySheet := sheets[0]
	for _, s := range sheets {
		if strings.TrimSpace(s) == "总成绩" {
			summarySheet = s
			break
		}
	}

	// 解析总成绩表
	rows, err := f.GetRows(summarySheet)
	if err != nil {
		return nil, fmt.Errorf("read summary sheet failed: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("summary sheet has no data")
	}

	// 解析表头
	headers := rows[0]
	nameIdx, classIdx := -1, -1
	subjectCols := make(map[int]string) // column index -> subject name

	for i, h := range headers {
		h = strings.TrimSpace(h)
		switch h {
		case "姓名":
			nameIdx = i
		case "班级":
			classIdx = i
		default:
			if h != "" {
				subjectCols[i] = h
			}
		}
	}

	if nameIdx == -1 {
		return nil, fmt.Errorf("column '姓名' not found")
	}
	if classIdx == -1 {
		return nil, fmt.Errorf("column '班级' not found")
	}
	if len(subjectCols) == 0 {
		return nil, fmt.Errorf("no subject columns found")
	}

	// 获取或创建学科
	subjectMap := make(map[string]int64) // subject name -> subject id
	for _, subjName := range subjectCols {
		subj, err := uc.subjectRepo.FindOrCreateByName(ctx, subjName)
		if err != nil {
			return nil, fmt.Errorf("find or create subject '%s' failed: %w", subjName, err)
		}
		subjectMap[subjName] = subj.ID
	}

	// 解析学生数据
	type studentScore struct {
		studentID int64
		classID   int64
		scores    map[string]float64 // subject name -> total score
	}
	studentScores := make([]*studentScore, 0, len(rows)-1)
	nameToStudentID := make(map[string]int64) // studentName -> studentID
	nameToClassID := make(map[string]int64)   // studentName -> classID

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 || strings.TrimSpace(row[nameIdx]) == "" {
			continue
		}

		studentName := strings.TrimSpace(row[nameIdx])
		className := strings.TrimSpace(row[classIdx])

		// 查找或创建班级
		class, err := uc.classRepo.FindOrCreateByName(ctx, className)
		if err != nil {
			return nil, fmt.Errorf("find or create class '%s' failed: %w", className, err)
		}

		// 查找或创建学生（自动生成学号）
		student, err := uc.studentRepo.FindOrCreateByNameClass(ctx, studentName, class.ID)
		if err != nil {
			return nil, fmt.Errorf("find or create student '%s' failed: %w", studentName, err)
		}

		nameToStudentID[studentName] = student.ID
		nameToClassID[studentName] = class.ID

		ss := &studentScore{
			studentID: student.ID,
			classID:   class.ID,
			scores:    make(map[string]float64),
		}

		for colIdx, subjName := range subjectCols {
			if colIdx < len(row) {
				scoreStr := strings.TrimSpace(row[colIdx])
				if scoreStr != "" {
					score, _ := strconv.ParseFloat(scoreStr, 64)
					ss.scores[subjName] = score
				}
			}
		}

		studentScores = append(studentScores, ss)
	}

	result.ImportedStudents = len(studentScores)
	result.ImportedSubjects = len(subjectCols)

	// 批量写入 scores 表
	scores := make([]*Score, 0)
	for _, ss := range studentScores {
		for subjName, totalScore := range ss.scores {
			scores = append(scores, &Score{
				StudentID:  ss.studentID,
				ExamID:     examID,
				SubjectID:  subjectMap[subjName],
				TotalScore: totalScore,
			})
		}
	}

	if len(scores) > 0 {
		if err := uc.scoreRepo.BatchCreate(ctx, scores); err != nil {
			return nil, fmt.Errorf("batch create scores failed: %w", err)
		}
	}

	// 建立 (studentID, subjectID) -> scoreID 映射
	scoreIDMap := make(map[string]int64)
	for _, s := range scores {
		key := fmt.Sprintf("%d_%d", s.StudentID, s.SubjectID)
		scoreIDMap[key] = s.ID
	}

	// 检查是否有科目明细 sheet
	for _, sheetName := range sheets {
		if sheetName == summarySheet {
			continue
		}
		subjID, ok := subjectMap[sheetName]
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sheet '%s' not matched to any subject, skipped", sheetName))
			continue
		}

		// 解析科目明细 sheet
		itemRows, err := f.GetRows(sheetName)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("read sheet '%s' failed: %v", sheetName, err))
			continue
		}
		if len(itemRows) < 2 {
			continue
		}

		// 解析明细表头
		itemHeaders := itemRows[0]
		itemNameIdx := -1
		questionCols := make(map[int]string)
		totalScoreIdx := -1

		for i, h := range itemHeaders {
			h = strings.TrimSpace(h)
			switch h {
			case "姓名":
				itemNameIdx = i
			case "总分":
				totalScoreIdx = i
			default:
				if strings.HasPrefix(h, "题") {
					questionCols[i] = h
				}
			}
		}

		if itemNameIdx == -1 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sheet '%s': column '姓名' not found", sheetName))
			continue
		}

		// 收集 score_items
		scoreItems := make([]*ScoreItem, 0)

		for i := 1; i < len(itemRows); i++ {
			row := itemRows[i]
			if len(row) == 0 || strings.TrimSpace(row[itemNameIdx]) == "" {
				continue
			}

			studentName := strings.TrimSpace(row[itemNameIdx])
			studentID, ok := nameToStudentID[studentName]
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("sheet '%s': student '%s' not found in summary", sheetName, studentName))
				continue
			}

			scoreKey := fmt.Sprintf("%d_%d", studentID, subjID)
			scoreID, ok := scoreIDMap[scoreKey]
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("sheet '%s': score not found for student '%s' subject '%s'", sheetName, studentName, sheetName))
				continue
			}

			// 解析每道题目的得分
			for colIdx, qNum := range questionCols {
				if colIdx < len(row) {
					scoreStr := strings.TrimSpace(row[colIdx])
					if scoreStr != "" {
						score, _ := strconv.ParseFloat(scoreStr, 64)
						scoreItems = append(scoreItems, &ScoreItem{
							ScoreID:        scoreID,
							QuestionNumber: qNum,
							Score:          score,
						})
					}
				}
			}

			// 如果提供了总分，也记录下来（可选）
			_ = totalScoreIdx
		}

		if len(scoreItems) > 0 {
			result.Mode = "full"
			if err := uc.scoreItemRepo.BatchCreate(ctx, scoreItems); err != nil {
				return nil, fmt.Errorf("batch create score items for subject '%s' failed: %w", sheetName, err)
			}
		}
	}

	return result, nil
}
