package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"seas/internal/server/middleware"
	"seas/pkg/jwt"

	"github.com/go-kratos/kratos/v2/log"
)

// User 用户模型
type User struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	OpenID    string    `gorm:"uniqueIndex;type:varchar(64);not null;column:openid"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at"`
}

// TableName 显式指定表名,与 init.sql 中的 users 表对齐
func (User) TableName() string { return "users" }

// AuthRepo 认证数据访问接口
type AuthRepo interface {
	GetByOpenID(ctx context.Context, openid string) (*User, error)
	CreateUser(ctx context.Context, openid string) (*User, error)
	SaveLoginCode(ctx context.Context, code string, status string, expiration time.Duration) error
	GetLoginCode(ctx context.Context, code string) (string, error)
	UpdateLoginCode(ctx context.Context, code string, status string, expiration time.Duration) error
}

// AuthUsecase 认证业务用例
type AuthUsecase struct {
	repo AuthRepo
	log  *log.Helper
}

// NewAuthUsecase 创建认证业务用例
func NewAuthUsecase(repo AuthRepo, logger log.Logger) *AuthUsecase {
	return &AuthUsecase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// LoginRequestResponse 登录请求响应
type LoginRequestResponse struct {
	Code          string
	QrURL         string
	ExpireSeconds int32
}

// LoginStatus 登录状态
type LoginStatus struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
}

// GenerateLoginCode 生成 5 位验证码
func (uc *AuthUsecase) GenerateLoginCode(ctx context.Context, qrURL string) (*LoginRequestResponse, error) {
	code := fmt.Sprintf("%05d", rand.Intn(100000))
	expiration := 5 * time.Minute

	err := uc.repo.SaveLoginCode(ctx, code, `{"status":"waiting"}`, expiration)
	if err != nil {
		return nil, err
	}

	return &LoginRequestResponse{
		Code:          code,
		QrURL:         qrURL,
		ExpireSeconds: 300,
	}, nil
}

// GetLoginStatus 查询登录状态
func (uc *AuthUsecase) GetLoginStatus(ctx context.Context, code string) (*LoginStatus, error) {
	raw, err := uc.repo.GetLoginCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return &LoginStatus{Status: "expired"}, nil
	}

	// Redis 中存储的是 JSON 字符串（如 {"status":"waiting"}），需要解析提取 status
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err == nil {
		s, _ := data["status"].(string)
		token, _ := data["token"].(string)
		return &LoginStatus{Status: s, Token: token}, nil
	}

	return &LoginStatus{Status: raw}, nil
}

// VerifyLoginCode 验证登录验证码（被微信回调调用）
func (uc *AuthUsecase) VerifyLoginCode(ctx context.Context, code string, openid string) (*LoginStatus, error) {
	status, err := uc.repo.GetLoginCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if status == "" {
		return &LoginStatus{Status: "expired"}, nil
	}

	user, err := uc.repo.GetByOpenID(ctx, openid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		user, err = uc.repo.CreateUser(ctx, openid)
		if err != nil {
			return nil, err
		}
	}

	token, err := jwt.GenerateToken(user.ID)
	if err != nil {
		return nil, err
	}

	loginStatus := fmt.Sprintf(`{"status":"success","token":"%s","user_id":%d}`, token, user.ID)
	err = uc.repo.UpdateLoginCode(ctx, code, loginStatus, 5*time.Minute)
	if err != nil {
		return nil, err
	}

	return &LoginStatus{Status: "success", Token: token}, nil
}

// GetCurrentUserID 从 context 读取当前用户 ID
func GetCurrentUserID(ctx context.Context) (uint64, error) {
	userID := middleware.GetUserID(ctx)
	if userID == 0 {
		return 0, errors.New("未登录或 token 无效")
	}
	return userID, nil
}

// CheckExamOwnership 校验当前用户是否有权访问该考试
// user_id=0 的公开考试允许任何人访问
func CheckExamOwnership(ctx context.Context, examRepo ExamRepo, examID int64) error {
	ownerID, err := examRepo.GetUserIDByExamID(ctx, examID)
	if err != nil {
		return err
	}
	// 公开考试允许任何人访问
	if ownerID == 0 {
		return nil
	}
	userID, err := GetCurrentUserID(ctx)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return errors.New("无权访问该考试数据")
	}
	return nil
}
