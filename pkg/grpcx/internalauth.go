package grpcx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetadataInternalToken 是内部服务间调用的互信凭据 metadata 键。
const MetadataInternalToken = "x-internal-token"

// internalToken 由共享密钥派生固定长度的调用凭据。
func internalToken(secret string) string {
	mac := hmac.New(sha256.New, []byte("internal-rpc-v1"))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

// UnaryClientAuthInterceptor 为每个出站内部调用附加互信凭据 metadata。
func UnaryClientAuthInterceptor(secret string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataInternalToken, internalToken(secret))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryServerAuthInterceptor 校验入站调用的互信凭据；缺失或不匹配一律拒绝。
// 使用常数时间比较，避免逐字节泄露匹配进度。
func UnaryServerAuthInterceptor(secret string) grpc.UnaryServerInterceptor {
	expected := internalToken(secret)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "internal rpc auth required")
		}
		values := md.Get(MetadataInternalToken)
		if len(values) == 0 || !hmac.Equal([]byte(values[0]), []byte(expected)) {
			return nil, status.Error(codes.Unauthenticated, "internal rpc auth required")
		}
		return handler(ctx, req)
	}
}
