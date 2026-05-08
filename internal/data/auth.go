package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	OpenID    string    `gorm:"column:openid;type:varchar(64);uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// AuthRepo 认证数据访问接口
type AuthRepo interface {
	GetByOpenID(ctx context.Context, openid string) (*User, error)
	CreateUser(ctx context.Context, openid string) (*User, error)
	SaveLoginCode(ctx context.Context, code string, status string, expiration time.Duration) error
	GetLoginCode(ctx context.Context, code string) (string, error)
	UpdateLoginCode(ctx context.Context, code string, status string, expiration time.Duration) error
}

// authRepo 认证数据访问实现
type authRepo struct {
	data *Data
	log  *log.Helper
}

// NewAuthRepo 创建认证数据访问实例
func NewAuthRepo(data *Data, logger log.Logger) AuthRepo {
	return &authRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// GetByOpenID 根据 OpenID 查询用户
func (r *authRepo) GetByOpenID(ctx context.Context, openid string) (*User, error) {
	var user User
	err := r.data.db.WithContext(ctx).Where("openid = ?", openid).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// CreateUser 创建新用户
func (r *authRepo) CreateUser(ctx context.Context, openid string) (*User, error) {
	user := &User{OpenID: openid}
	err := r.data.db.WithContext(ctx).Create(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

// loginCodeKey Redis 键前缀
const loginCodeKeyPrefix = "wechat:login:"

func loginCodeKey(code string) string {
	return loginCodeKeyPrefix + code
}

// SaveLoginCode 保存登录验证码
func (r *authRepo) SaveLoginCode(ctx context.Context, code string, status string, expiration time.Duration) error {
	key := loginCodeKey(code)
	_, err := r.data.Redis().Set(ctx, key, status, expiration)
	return err
}

// GetLoginCode 获取登录验证码状态
func (r *authRepo) GetLoginCode(ctx context.Context, code string) (string, error) {
	key := loginCodeKey(code)
	return r.data.Redis().GetExists(ctx, key)
}

// UpdateLoginCode 更新登录验证码状态
func (r *authRepo) UpdateLoginCode(ctx context.Context, code string, status string, expiration time.Duration) error {
	key := loginCodeKey(code)
	_, err := r.data.Redis().Set(ctx, key, status, expiration)
	return err
}
