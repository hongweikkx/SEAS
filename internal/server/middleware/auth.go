package middleware

import (
	"context"
	"strings"

	"seas/pkg/jwt"

	"github.com/go-kratos/kratos/v2/middleware"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
)

type userIDKey struct{}

// WithUserID 将 user_id 注入 context
func WithUserID(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// GetUserID 从 context 读取 user_id
func GetUserID(ctx context.Context) uint64 {
	v, _ := ctx.Value(userIDKey{}).(uint64)
	return v
}

// Auth 从 Authorization header 解析 JWT，将 user_id 注入 context
// 无有效 token 时 user_id 保持为 0（未登录状态）
func Auth() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := httptransport.RequestFromServerContext(ctx); ok {
				auth := tr.Header.Get("Authorization")
				if auth != "" {
					parts := strings.SplitN(auth, " ", 2)
					if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
						claims, err := jwt.ParseToken(parts[1])
						if err == nil && claims.UserID > 0 {
							ctx = WithUserID(ctx, claims.UserID)
						}
					}
				}
			}
			return handler(ctx, req)
		}
	}
}
