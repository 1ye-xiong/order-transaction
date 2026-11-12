package main

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDisconnect struct{ err error }

func (f fakeDisconnect) Disconnect(context.Context) error { return f.err }
func TestDisconnectMongo(t *testing.T) {
	disconnectMongo(fakeDisconnect{}, zap.NewNop())
	disconnectMongo(fakeDisconnect{err: context.Canceled}, zap.NewNop())
}

func TestRunMissingConfig(t *testing.T) {
	if err := run(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("missing config accepted")
	}
}

func TestRunAssembly(t *testing.T) {
	raw, e := os.ReadFile("../../configs/config.yaml")
	if e != nil {
		t.Fatal(e)
	}
	text := strings.ReplaceAll(string(raw), "enabled: true", "enabled: false")
	p := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(p, []byte(text), 0600)
	oldL, oldS, oldA, oldP := listenTCP, serveGRPC, beginShutdown, pingMongo
	defer func() { listenTCP = oldL; serveGRPC = oldS; beginShutdown = oldA; pingMongo = oldP }()
	listenTCP = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	pingMongo = func(context.Context, *mongo.Client) error { return nil }
	beginShutdown = func(*grpc.Server, *zap.Logger) {}
	serveGRPC = func(s *grpc.Server, l net.Listener) error {
		defer l.Close()
		if len(s.GetServiceInfo()) == 0 {
			t.Error("no service")
		}
		return nil
	}
	if e = run(p); e != nil {
		t.Fatal(e)
	}
	fault := errors.New("probe")
	pingMongo = func(context.Context, *mongo.Client) error { return fault }
	if e = run(p); !errors.Is(e, fault) {
		t.Fatalf("ping error lost: %v", e)
	}
	pingMongo = func(context.Context, *mongo.Client) error { return nil }
	listenTCP = func(string, string) (net.Listener, error) { return nil, fault }
	if e = run(p); !errors.Is(e, fault) {
		t.Fatal(e)
	}
	listenTCP = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	serveGRPC = func(_ *grpc.Server, l net.Listener) error { l.Close(); return fault }
	if e = run(p); !errors.Is(e, fault) {
		t.Fatal(e)
	}
	bad := strings.ReplaceAll(text, "level: info", "level: bogus")
	os.WriteFile(p, []byte(bad), 0600)
	if run(p) == nil {
		t.Fatal("invalid logger accepted")
	}
	bad = strings.ReplaceAll(text, "uri: mongodb://127.0.0.1:27017", "uri: invalid://bad")
	os.WriteFile(p, []byte(bad), 0600)
	if run(p) == nil {
		t.Fatal("invalid Mongo uri accepted")
	}

}
