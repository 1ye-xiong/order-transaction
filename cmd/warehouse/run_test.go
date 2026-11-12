package main

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForwardIdentityPreservesContext(t *testing.T) {
	ctx := context.Background()
	if forwardIdentity(ctx) == nil {
		t.Fatal("nil context")
	}
}

func TestRunMissingConfig(t *testing.T) {
	if err := run(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("missing config accepted")
	}
}

func TestRunAssembly(t *testing.T) {
	raw, err := os.ReadFile("../../configs/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	text = strings.ReplaceAll(text, "host: 0.0.0.0", "host: 127.0.0.1")
	text = strings.ReplaceAll(text, "enabled: true", "enabled: false")
	text = strings.ReplaceAll(text, "data/order-transaction.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", filepath.ToSlash(filepath.Join(t.TempDir(), "service.db")))
	for _, p := range []string{"9001", "9002", "9100", "9101", "9103", "9104"} {
		text = strings.ReplaceAll(text, "port: "+p, "port: 0")
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err = os.WriteFile(cfgPath, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	oldListen := listenTCP
	defer func() { listenTCP = oldListen }()
	listenTCP = func(network, addr string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	oldServe, oldShutdown := serveGRPC, beginShutdown
	defer func() { serveGRPC = oldServe; beginShutdown = oldShutdown }()
	beginShutdown = func(*grpc.Server, *zap.Logger) {}
	called := false
	serveGRPC = func(s *grpc.Server, l net.Listener) error {
		defer l.Close()
		called = true
		if len(s.GetServiceInfo()) == 0 {
			t.Error("no registered services")
		}
		return nil
	}
	if err = run(cfgPath); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("server not assembled")
	}

	fault := errors.New("listen fault")
	listenTCP = func(string, string) (net.Listener, error) { return nil, fault }
	if e := run(cfgPath); !errors.Is(e, fault) {
		t.Fatalf("listen error lost: %v", e)
	}
	listenTCP = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	serveGRPC = func(_ *grpc.Server, l net.Listener) error { l.Close(); return fault }
	if e := run(cfgPath); !errors.Is(e, fault) {
		t.Fatalf("serve error lost: %v", e)
	}
	bad := strings.ReplaceAll(text, "level: info", "level: bogus")
	os.WriteFile(cfgPath, []byte(bad), 0600)
	if run(cfgPath) == nil {
		t.Fatal("invalid log level accepted")
	}
}
