package grpcx

import (
	"context"
	ae "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"net"
	"testing"
)

func TestServerAndIdentity(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataUserID, "u", MetadataUserRole, "ops", MetadataRequestID, "r"))
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{Port: 123}})
	u, r := IdentityFromContext(ctx)
	if u != "u" || r != "ops" || RequestIDFromContext(ctx) != "r" {
		t.Fatal("metadata")
	}
	f := UnaryServerInterceptor(zap.NewNop())
	for _, err := range []error{nil, ae.NotFound("missing", "x")} {
		v, e := f(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/x"}, func(context.Context, any) (any, error) { return "ok", err })
		if v != "ok" || (err != nil && status.Code(e) != codes.NotFound) {
			t.Fatal(v, e)
		}
	}
	if firstMetadata(context.Background(), "x") != "" || firstMetadata(metadata.NewIncomingContext(context.Background(), metadata.MD{}), "x") != "" || peerAddress(context.Background()) != "" {
		t.Fatal("empty")
	}
}
func TestClientMetadata(t *testing.T) {
	for _, id := range []string{"", "r"} {
		called := false
		e := UnaryClientInterceptor(id)(context.Background(), "/x", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			called = true
			md, _ := metadata.FromOutgoingContext(ctx)
			if id != "" && md.Get(MetadataRequestID)[0] != id {
				t.Fatal(md)
			}
			return nil
		})
		if e != nil || !called {
			t.Fatal(e)
		}
	}
}
