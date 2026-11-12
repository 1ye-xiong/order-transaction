package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"

	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

// outgoingContext 将请求 ID 等 gateway 元数据转发到下游 gRPC 调用。
func outgoingContext(c *gin.Context) context.Context {
	ctx := metadata.AppendToOutgoingContext(c.Request.Context(), grpcx.MetadataRequestID, httpx.RequestID(c))
	// 登录身份由 JWT 决定并透传到下游，禁止信任请求体中的用户编号。
	if claims := middleware.ClaimsFromContext(c); claims.UserID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx,
			grpcx.MetadataUserID, claims.UserID,
			grpcx.MetadataUserRole, claims.Role,
		)
	}
	return ctx
}
