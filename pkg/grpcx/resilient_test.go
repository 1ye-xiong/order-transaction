package grpcx

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
)

// 复现并回归"启动竞态 fail-fast"缺陷：下游晚于调用方启动时，
// fail-fast RPC 会返回启动瞬间缓存的 connection refused（Unavailable），
// 而韧性拨号应等待连接恢复后再执行调用。

type stubUserService struct {
	userv1.UnimplementedUserServiceServer
}

func resilientDial(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		ResilientDialOption(3*time.Second),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestResilientDialWaitsForLateDownstream(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()

	conn := resilientDial(t, addr)
	client := userv1.NewUserServiceClient(conn)

	// 下游延迟 300ms 才启动，模拟服务启动顺序竞态。
	go func() {
		time.Sleep(300 * time.Millisecond)
		server := grpc.NewServer()
		userv1.RegisterUserServiceServer(server, &stubUserService{})
		_ = server.Serve(listener)
	}()

	start := time.Now()
	// 无 deadline：应被 ResilientDialOption 补充默认超时，并等待下游就绪。
	_, err = client.Login(context.Background(), &userv1.LoginRequest{Account: "x", Password: "y"})
	elapsed := time.Since(start)

	if elapsed < 300*time.Millisecond {
		t.Fatalf("RPC returned before downstream started (%v); fail-fast regression", elapsed)
	}
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented once connected, got %v (err=%v)", status.Code(err), err)
	}
}

func TestResilientDialBoundsCallWhenDownstreamAbsent(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// 故意不 Serve：下游永远不可用。
	_ = listener.Close()

	conn := resilientDial(t, listener.Addr().String())
	client := userv1.NewUserServiceClient(conn)

	start := time.Now()
	_, err = client.Login(context.Background(), &userv1.LoginRequest{Account: "x", Password: "y"})
	elapsed := time.Since(start)

	if elapsed > 4*time.Second {
		t.Fatalf("call hung past default timeout: %v", elapsed)
	}
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v (err=%v)", status.Code(err), err)
	}
}
