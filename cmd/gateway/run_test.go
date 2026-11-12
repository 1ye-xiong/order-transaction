package main

import (
	"errors"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	oldL, oldS, oldA := listenTCP, serveHTTP, awaitShutdown
	defer func() { listenTCP = oldL; serveHTTP = oldS; awaitShutdown = oldA }()
	listenTCP = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	done := make(chan struct{})
	serveHTTP = func(s *http.Server, l net.Listener) error {
		defer l.Close()
		defer close(done)
		if s.Handler == nil {
			t.Error("missing router")
		}
		return http.ErrServerClosed
	}
	awaitShutdown = func(*http.Server, *zap.Logger) { <-done }
	if e = run(p); e != nil {
		t.Fatal(e)
	}
	fault := errors.New("listen fault")
	listenTCP = func(string, string) (net.Listener, error) { return nil, fault }
	if e = run(p); !errors.Is(e, fault) {
		t.Fatal(e)
	}
	bad := strings.ReplaceAll(text, "level: info", "level: bogus")
	os.WriteFile(p, []byte(bad), 0600)
	if run(p) == nil {
		t.Fatal("invalid logger accepted")
	}

}
