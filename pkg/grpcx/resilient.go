package grpcx

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// DefaultInternalCallTimeout 是内部服务间调用的默认超时。
// 仅当调用方没有显式设置 deadline 时才生效。
const DefaultInternalCallTimeout = 5 * time.Second

// ResilientDialOption 返回内部服务间拨号的韧性选项，解决两处已确认的可用性缺口：
//
//  1. 启动竞态 fail-fast：grpc.Dial 是非阻塞拨号，若下游晚于本服务启动，
//     首次后台连接会失败并进入 TRANSIENT_FAILURE 退避；在退避窗口内到达的
//     fail-fast RPC 会直接返回缓存的 "connection refused"，即使下游此刻已经
//     就绪也不会重试。WaitForReady(true) 让这类调用等待连接恢复，
//     而不是把启动瞬间的瞬时失败抛给调用方。
//
//  2. 无超时调用：若下游彻底不可用，等待必须有限。本选项在调用方未设置
//     deadline 时补充默认超时，避免请求无限挂起。
func ResilientDialOption(defaultCallTimeout time.Duration) grpc.DialOption {
	timeoutInterceptor := func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := ctx.Deadline(); !ok && defaultCallTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultCallTimeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	return grpc.WithChainUnaryInterceptor(timeoutInterceptor)
}
