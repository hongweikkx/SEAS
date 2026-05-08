package data

import (
	"context"
	"time"

	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// authUser GORM 用户模型
type authUser struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	OpenID    string    `gorm:"column:openid;type:varchar(64);uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName 指定表名
func (authUser) TableName() string {
	return "users"
}

// authRepo 认证数据访问实现
type authRepo struct {
	data *Data
	log  *log.Helper
}

// NewAuthRepo 创建认证数据访问实例
func NewAuthRepo(data *Data, logger log.Logger) biz.AuthRepo {
	return &authRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// GetByOpenID 根据 OpenID 查询用户
func (r *authRepo) GetByOpenID(ctx context.Context, openid string) (*biz.User, error) {
	var user authUser
	err := r.data.db.WithContext(ctx).Where("openid = ?", openid).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &biz.User{
		ID:        user.ID,
		OpenID:    user.OpenID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// CreateUser 创建新用户
func (r *authRepo) CreateUser(ctx context.Context, openid string) (*biz.User, error) {
	user := &authUser{OpenID: openid}
	err := r.data.db.WithContext(ctx).Create(user).Error
	if err != nil {
		return nil, err
	}
	return &biz.User{
		ID:        user.ID,
		OpenID:    user.OpenID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
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
