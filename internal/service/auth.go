package service

import (
	"context"

	v1 "seas/api/seas/v1"
	"seas/internal/biz"
	"seas/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
)

// AuthService 认证服务实现
type AuthService struct {
	v1.UnimplementedAuthServer

	uc      *biz.AuthUsecase
	authConf *conf.Auth
	log     *log.Helper
}

// NewAuthService 创建认证服务
func NewAuthService(uc *biz.AuthUsecase, authConf *conf.Auth, logger log.Logger) *AuthService {
	return &AuthService{
		uc:      uc,
		authConf: authConf,
		log:     log.NewHelper(logger),
	}
}

// qrURL 从配置读取公众号二维码 URL
func (s *AuthService) qrURL() string {
	if s.authConf != nil && s.authConf.WechatQrUrl != "" {
		return s.authConf.WechatQrUrl
	}
	return "https://mp.weixin.qq.com/"
}

// LoginRequest 生成登录验证码
func (s *AuthService) LoginRequest(ctx context.Context, _ *v1.LoginRequestRequest) (*v1.LoginRequestResponse, error) {
	resp, err := s.uc.GenerateLoginCode(ctx, s.qrURL())
	if err != nil {
		return nil, err
	}
	return &v1.LoginRequestResponse{
		Code:          resp.Code,
		QrUrl:         resp.QrURL,
		ExpireSeconds: resp.ExpireSeconds,
	}, nil
}

// Logout 登出
func (s *AuthService) Logout(ctx context.Context, _ *v1.LogoutRequest) (*v1.LogoutResponse, error) {
	// JWT 无状态，服务端无需额外操作
	// 前端删除 token 即可
	return &v1.LogoutResponse{}, nil
}
