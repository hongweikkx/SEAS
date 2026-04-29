package service

import (
	"context"
	"strconv"

	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"

	pb "seas/api/seas/v1"
)

type AnalysisService struct {
	pb.UnimplementedAnalysisServer
	analysisUC     *biz.AnalysisUseCase
	examAnalysisUC *biz.ExamAnalysisUseCase
}

func NewAnalysisService(analysisUC *biz.AnalysisUseCase, examAnalysisUC *biz.ExamAnalysisUseCase) *AnalysisService {
	return &AnalysisService{
		analysisUC:     analysisUC,
		examAnalysisUC: examAnalysisUC,
	}
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// ListExams 获取考试列表
func (s *AnalysisService) ListExams(ctx context.Context, req *pb.ListExamsRequest) (*pb.ListExamsReply, error) {
	log.Context(ctx).Infof("Received ListExamsRequest: %v", req)

	exams, total, err := s.examAnalysisUC.ListExams(ctx, req.GetPageIndex(), req.GetPageSize(), req.GetKeyword())
	if err != nil {
		return nil, err
	}

	reply := &pb.ListExamsReply{
		TotalCount: total,
		PageIndex:  req.GetPageIndex(),
		PageSize:   req.GetPageSize(),
	}

	// 批量获取各考试的学生人数
	examIDs := make([]int64, len(exams))
	for i, exam := range exams {
		examIDs[i] = exam.ID
	}
	studentCounts, err := s.examAnalysisUC.GetExamStudentCounts(ctx, examIDs)
	if err != nil {
		log.Context(ctx).Errorf("GetExamStudentCounts failed: %v", err)
		studentCounts = make(map[int64]int64)
	}

	reply.Exams = make([]*pb.ExamInfo, len(exams))
	for i, exam := range exams {
		reply.Exams[i] = &pb.ExamInfo{
			Id:           strconv.FormatInt(exam.ID, 10),
			Name:         exam.Name,
			ExamDate:     exam.ExamDate.Format("2006-01-02T15:04:05Z"),
			CreatedAt:    exam.CreatedAt.Format("2006-01-02T15:04:05Z"),
			StudentCount: int32(studentCounts[exam.ID]),
		}
	}

	return reply, nil
}

// ListSubjectsByExam 获取考试关联的学科列表
func (s *AnalysisService) ListSubjectsByExam(ctx context.Context, req *pb.ListSubjectsByExamRequest) (*pb.ListSubjectsByExamReply, error) {
	log.Context(ctx).Infof("Received ListSubjectsByExamRequest: %v", req)

	subjects, total, err := s.examAnalysisUC.ListSubjectsByExam(ctx, parseInt64(req.GetExamId()), req.GetPageIndex(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	reply := &pb.ListSubjectsByExamReply{
		ExamId:     req.GetExamId(),
		TotalCount: total,
		PageIndex:  req.GetPageIndex(),
		PageSize:   req.GetPageSize(),
	}

	reply.Subjects = make([]*pb.SubjectBasicInfo, len(subjects))
	for i, subject := range subjects {
		fullScore, err := s.examAnalysisUC.GetSubjectFullScore(ctx, parseInt64(req.GetExamId()), subject.ID)
		if err != nil {
			log.Context(ctx).Errorf("GetSubjectFullScore failed for subject %d: %v", subject.ID, err)
			fullScore = 100 // fallback
		}
		reply.Subjects[i] = &pb.SubjectBasicInfo{
			Id:        strconv.FormatInt(subject.ID, 10),
			Name:      subject.Name,
			FullScore: fullScore,
		}
	}

	return reply, nil
}

// GetSubjectSummary 获取学科情况汇总
func (s *AnalysisService) GetSubjectSummary(ctx context.Context, req *pb.GetSubjectSummaryRequest) (*pb.GetSubjectSummaryReply, error) {
	log.Context(ctx).Infof("Received GetSubjectSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetSubjectSummary(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()))
	if err != nil {
		return nil, err
	}

	// 获取考试名称
	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试" // 默认值
	}

	reply := &pb.GetSubjectSummaryReply{
		ExamId:            req.GetExamId(),
		ExamName:          examName,
		Scope:             req.GetScope(),
		TotalParticipants: int32(stats.TotalParticipants),
		SubjectsInvolved:  stats.SubjectsInvolved,
		ClassesInvolved:   stats.ClassesInvolved,
	}

	if stats.Overall != nil {
		reply.Overall = &pb.SubjectSummaryItem{
			Id:             strconv.FormatInt(stats.Overall.ID, 10),
			Name:           stats.Overall.Name,
			FullScore:      stats.Overall.FullScore,
			AvgScore:       stats.Overall.AvgScore,
			HighestScore:   stats.Overall.HighestScore,
			LowestScore:    stats.Overall.LowestScore,
			Difficulty:     stats.Overall.Difficulty,
			StudentCount:   int32(stats.Overall.StudentCount),
			ScoreDeviation: stats.Overall.ScoreDeviation,
			StdDev:         stats.Overall.StdDev,
			Discrimination: stats.Overall.Discrimination,
		}
	}

	reply.Subjects = make([]*pb.SubjectSummaryItem, len(stats.Subjects))
	for i, subject := range stats.Subjects {
		reply.Subjects[i] = &pb.SubjectSummaryItem{
			Id:             strconv.FormatInt(subject.ID, 10),
			Name:           subject.Name,
			FullScore:      subject.FullScore,
			AvgScore:       subject.AvgScore,
			HighestScore:   subject.HighestScore,
			LowestScore:    subject.LowestScore,
			Difficulty:     subject.Difficulty,
			StudentCount:   int32(subject.StudentCount),
			ScoreDeviation: subject.ScoreDeviation,
			StdDev:         subject.StdDev,
			Discrimination: subject.Discrimination,
		}
	}

	return reply, nil
}

// GetClassSummary 获取班级情况汇总
func (s *AnalysisService) GetClassSummary(ctx context.Context, req *pb.GetClassSummaryRequest) (*pb.GetClassSummaryReply, error) {
	log.Context(ctx).Infof("Received GetClassSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetClassSummary(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()))
	if err != nil {
		return nil, err
	}

	// 获取考试名称
	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试" // 默认值
	}

	reply := &pb.GetClassSummaryReply{
		ExamId:            req.GetExamId(),
		ExamName:          examName,
		Scope:             req.GetScope(),
		TotalParticipants: int32(stats.TotalParticipants),
	}

	if stats.OverallGrade != nil {
		reply.OverallGrade = &pb.ClassSummaryItem{
			ClassId:        int32(stats.OverallGrade.ClassID),
			ClassName:      stats.OverallGrade.ClassName,
			TotalStudents:  int32(stats.OverallGrade.TotalStudents),
			AvgScore:       stats.OverallGrade.AvgScore,
			HighestScore:   stats.OverallGrade.HighestScore,
			LowestScore:    stats.OverallGrade.LowestScore,
			ScoreDeviation: stats.OverallGrade.ScoreDeviation,
			Difficulty:     stats.OverallGrade.Difficulty,
			StdDev:         stats.OverallGrade.StdDev,
			FullScore:      stats.OverallGrade.FullScore,
			Discrimination: stats.OverallGrade.Discrimination,
		}
	}

	reply.ClassDetails = make([]*pb.ClassSummaryItem, len(stats.ClassDetails))
	for i, class := range stats.ClassDetails {
		reply.ClassDetails[i] = &pb.ClassSummaryItem{
			ClassId:        int32(class.ClassID),
			ClassName:      class.ClassName,
			TotalStudents:  int32(class.TotalStudents),
			AvgScore:       class.AvgScore,
			HighestScore:   class.HighestScore,
			LowestScore:    class.LowestScore,
			ScoreDeviation: class.ScoreDeviation,
			Difficulty:     class.Difficulty,
			StdDev:         class.StdDev,
			FullScore:      class.FullScore,
			Discrimination: class.Discrimination,
		}
	}

	return reply, nil
}

// GetRatingDistribution 获取四率分析
func (s *AnalysisService) GetRatingDistribution(ctx context.Context, req *pb.GetRatingDistributionRequest) (*pb.GetRatingDistributionReply, error) {
	log.Context(ctx).Infof("Received GetRatingDistributionRequest: %v", req)

	// 使用默认值或请求中的配置
	excellentThreshold := req.GetExcellentThreshold()
	goodThreshold := req.GetGoodThreshold()
	passThreshold := req.GetPassThreshold()

	if excellentThreshold == 0 {
		excellentThreshold = 90
	}
	if goodThreshold == 0 {
		goodThreshold = 70
	}
	if passThreshold == 0 {
		passThreshold = 60
	}

	stats, err := s.examAnalysisUC.GetRatingDistribution(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()), excellentThreshold, goodThreshold, passThreshold)
	if err != nil {
		return nil, err
	}

	// 获取考试名称
	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试" // 默认值
	}

	reply := &pb.GetRatingDistributionReply{
		ExamId:            req.GetExamId(),
		ExamName:          examName,
		Scope:             req.GetScope(),
		TotalParticipants: int32(stats.TotalParticipants),
		Config: &pb.RatingConfig{
			ExcellentThreshold: excellentThreshold,
			GoodThreshold:      goodThreshold,
			PassThreshold:      passThreshold,
		},
	}

	if stats.OverallGrade != nil {
		reply.OverallGrade = &pb.ClassRatingDistribution{
			ClassId:       int32(stats.OverallGrade.ClassID),
			ClassName:     stats.OverallGrade.ClassName,
			TotalStudents: int32(stats.OverallGrade.TotalStudents),
			AvgScore:      stats.OverallGrade.AvgScore,
			Excellent: &pb.RatingItem{
				Count:      stats.OverallGrade.Excellent.Count,
				Percentage: stats.OverallGrade.Excellent.Percentage,
			},
			Good: &pb.RatingItem{
				Count:      stats.OverallGrade.Good.Count,
				Percentage: stats.OverallGrade.Good.Percentage,
			},
			Pass: &pb.RatingItem{
				Count:      stats.OverallGrade.Pass.Count,
				Percentage: stats.OverallGrade.Pass.Percentage,
			},
			Fail: &pb.RatingItem{
				Count:      stats.OverallGrade.Fail.Count,
				Percentage: stats.OverallGrade.Fail.Percentage,
			},
		}
	}

	reply.ClassDetails = make([]*pb.ClassRatingDistribution, len(stats.ClassDetails))
	for i, class := range stats.ClassDetails {
		reply.ClassDetails[i] = &pb.ClassRatingDistribution{
			ClassId:       int32(class.ClassID),
			ClassName:     class.ClassName,
			TotalStudents: int32(class.TotalStudents),
			AvgScore:      class.AvgScore,
			Excellent: &pb.RatingItem{
				Count:      class.Excellent.Count,
				Percentage: class.Excellent.Percentage,
			},
			Good: &pb.RatingItem{
				Count:      class.Good.Count,
				Percentage: class.Good.Percentage,
			},
			Pass: &pb.RatingItem{
				Count:      class.Pass.Count,
				Percentage: class.Pass.Percentage,
			},
			Fail: &pb.RatingItem{
				Count:      class.Fail.Count,
				Percentage: class.Fail.Percentage,
			},
		}
	}

	return reply, nil
}

// GetClassSubjectSummary 获取班级学科下钻汇总
func (s *AnalysisService) GetClassSubjectSummary(ctx context.Context, req *pb.GetClassSubjectSummaryRequest) (*pb.GetClassSubjectSummaryReply, error) {
	log.Context(ctx).Infof("Received GetClassSubjectSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetClassSubjectSummary(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetClassId()))
	if err != nil {
		return nil, err
	}

	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试"
	}

	reply := &pb.GetClassSubjectSummaryReply{
		ExamId:    req.GetExamId(),
		ExamName:  examName,
		ClassId:   req.GetClassId(),
		ClassName: stats.ClassName,
	}

	if stats.Overall != nil {
		reply.Overall = &pb.ClassSubjectItem{
			SubjectId:     strconv.FormatInt(stats.Overall.SubjectID, 10),
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

	reply.Subjects = make([]*pb.ClassSubjectItem, len(stats.Subjects))
	for i, subject := range stats.Subjects {
		reply.Subjects[i] = &pb.ClassSubjectItem{
			SubjectId:     strconv.FormatInt(subject.SubjectID, 10),
			SubjectName:   subject.SubjectName,
			FullScore:     subject.FullScore,
			ClassAvgScore: subject.ClassAvgScore,
			GradeAvgScore: subject.GradeAvgScore,
			ScoreDiff:     subject.ScoreDiff,
			ClassHighest:  subject.ClassHighest,
			ClassLowest:   subject.ClassLowest,
			ClassRank:     subject.ClassRank,
			TotalClasses:  subject.TotalClasses,
		}
	}

	return reply, nil
}

// GetSingleClassSummary 获取单科班级汇总
func (s *AnalysisService) GetSingleClassSummary(ctx context.Context, req *pb.GetSingleClassSummaryRequest) (*pb.GetSingleClassSummaryReply, error) {
	log.Context(ctx).Infof("Received GetSingleClassSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetSingleClassSummary(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()))
	if err != nil {
		return nil, err
	}

	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试"
	}

	reply := &pb.GetSingleClassSummaryReply{
		ExamId:      req.GetExamId(),
		ExamName:    examName,
		SubjectId:   req.GetSubjectId(),
		SubjectName: stats.SubjectName,
	}

	if stats.Overall != nil {
		reply.Overall = &pb.ClassSummaryItem{
			ClassId:        int32(stats.Overall.ClassID),
			ClassName:      stats.Overall.ClassName,
			TotalStudents:  int32(stats.Overall.TotalStudents),
			FullScore:      stats.Overall.FullScore,
			AvgScore:       stats.Overall.AvgScore,
			HighestScore:   stats.Overall.HighestScore,
			LowestScore:    stats.Overall.LowestScore,
			ScoreDeviation: stats.Overall.ScoreDeviation,
			Difficulty:     stats.Overall.Difficulty,
			StdDev:         stats.Overall.StdDev,
			Discrimination: stats.Overall.Discrimination,
		}
	}

	reply.Classes = make([]*pb.ClassSummaryItem, len(stats.Classes))
	for i, class := range stats.Classes {
		reply.Classes[i] = &pb.ClassSummaryItem{
			ClassId:        int32(class.ClassID),
			ClassName:      class.ClassName,
			TotalStudents:  int32(class.TotalStudents),
			FullScore:      class.FullScore,
			AvgScore:       class.AvgScore,
			HighestScore:   class.HighestScore,
			LowestScore:    class.LowestScore,
			ScoreDeviation: class.ScoreDeviation,
			Difficulty:     class.Difficulty,
			StdDev:         class.StdDev,
			Discrimination: class.Discrimination,
		}
	}

	return reply, nil
}

// GetSingleClassQuestions 获取单科班级题目汇总
func (s *AnalysisService) GetSingleClassQuestions(ctx context.Context, req *pb.GetSingleClassQuestionsRequest) (*pb.GetSingleClassQuestionsReply, error) {
	log.Context(ctx).Infof("Received GetSingleClassQuestionsRequest: %v", req)

	stats, err := s.examAnalysisUC.GetSingleClassQuestions(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()), parseInt64(req.GetClassId()))
	if err != nil {
		return nil, err
	}

	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试"
	}

	reply := &pb.GetSingleClassQuestionsReply{
		ExamId:      req.GetExamId(),
		ExamName:    examName,
		SubjectId:   req.GetSubjectId(),
		SubjectName: stats.SubjectName,
		ClassId:     req.GetClassId(),
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

	return reply, nil
}

// GetSingleQuestionSummary 获取单科题目汇总
func (s *AnalysisService) GetSingleQuestionSummary(ctx context.Context, req *pb.GetSingleQuestionSummaryRequest) (*pb.GetSingleQuestionSummaryReply, error) {
	log.Context(ctx).Infof("Received GetSingleQuestionSummaryRequest: %v", req)

	stats, err := s.examAnalysisUC.GetSingleQuestionSummary(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()))
	if err != nil {
		return nil, err
	}

	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试"
	}

	reply := &pb.GetSingleQuestionSummaryReply{
		ExamId:      req.GetExamId(),
		ExamName:    examName,
		SubjectId:   req.GetSubjectId(),
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
				ClassId:   int32(cb.ClassID),
				ClassName: cb.ClassName,
				AvgScore:  cb.AvgScore,
			}
		}
		reply.Questions[i] = item
	}

	return reply, nil
}

// GetSingleQuestionDetail 获取单科班级题目详情
func (s *AnalysisService) GetSingleQuestionDetail(ctx context.Context, req *pb.GetSingleQuestionDetailRequest) (*pb.GetSingleQuestionDetailReply, error) {
	log.Context(ctx).Infof("Received GetSingleQuestionDetailRequest: %v", req)

	stats, err := s.examAnalysisUC.GetSingleQuestionDetail(ctx, parseInt64(req.GetExamId()), parseInt64(req.GetSubjectId()), parseInt64(req.GetClassId()), req.GetQuestionId())
	if err != nil {
		return nil, err
	}

	examName, err := s.examAnalysisUC.GetExamName(ctx, parseInt64(req.GetExamId()))
	if err != nil {
		log.Context(ctx).Errorf("GetExamName failed: %v", err)
		examName = "未知考试"
	}

	reply := &pb.GetSingleQuestionDetailReply{
		ExamId:          req.GetExamId(),
		ExamName:        examName,
		SubjectId:       req.GetSubjectId(),
		SubjectName:     stats.SubjectName,
		ClassId:         req.GetClassId(),
		ClassName:       stats.ClassName,
		QuestionId:      stats.QuestionID,
		QuestionNumber:  stats.QuestionNumber,
		QuestionType:    stats.QuestionType,
		FullScore:       stats.FullScore,
		QuestionContent: stats.QuestionContent,
	}

	reply.Students = make([]*pb.StudentQuestionDetail, len(stats.Students))
	for i, st := range stats.Students {
		reply.Students[i] = &pb.StudentQuestionDetail{
			StudentId:     strconv.FormatInt(st.StudentID, 10),
			StudentName:   st.StudentName,
			Score:         st.Score,
			FullScore:     st.FullScore,
			ScoreRate:     st.ScoreRate,
			ClassRank:     st.ClassRank,
			GradeRank:     st.GradeRank,
			AnswerContent: st.AnswerContent,
		}
	}

	return reply, nil
}
