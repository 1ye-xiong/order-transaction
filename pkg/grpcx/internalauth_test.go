package grpcx

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
)

func authServe(t *testing.T, secret string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(UnaryServerAuthInterceptor(secret)))
	userv1.RegisterUserServiceServer(server, &stubUserService{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String()
}

func authDial(t *testing.T, addr, secret string, withAuth bool, wrongSecret bool) userv1.UserServiceClient {
	t.Helper()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if withAuth {
		s := secret
		if wrongSecret {
			s = secret + "-wrong"
		}
		opts = append(opts, grpc.WithChainUnaryInterceptor(UnaryClientAuthInterceptor(s)))
	}
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn)
}

// 持有共享密钥的内部调用放行；缺失或伪造凭据一律 Unauthenticated。
func TestInternalAuth(t *testing.T) {
	const secret = "teaching-internal-rpc-secret"
	addr := authServe(t, secret)
	call := func(client userv1.UserServiceClient) codes.Code {
		_, err := client.Login(context.Background(), &userv1.LoginRequest{Account: "x", Password: "y"})
		return status.Code(err)
	}

	if got := call(authDial(t, addr, secret, true, false)); got != codes.Unimplemented {
		t.Fatalf("带正确凭据应到达业务处理, got %v", got)
	}
	if got := call(authDial(t, addr, secret, false, false)); got != codes.Unauthenticated {
		t.Fatalf("缺凭据应拒绝, got %v", got)
	}
	if got := call(authDial(t, addr, secret, true, true)); got != codes.Unauthenticated {
		t.Fatalf("伪造凭据应拒绝, got %v", got)
	}
}
