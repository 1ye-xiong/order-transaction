package client

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

// New 使用惰性拨号，目标不可达不影响创建；连接资源由 Close 统一释放。
func TestNewClientsAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
services:
  user:
    port: 19001
  note:
    port: 19002
  order:
    port: 19003
  product:
    port: 19004
  payment:
    port: 19005
  warehouse:
    port: 19006
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	clients, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("new clients: %v", err)
	}
	if clients.User == nil || clients.Note == nil || clients.Order == nil ||
		clients.Product == nil || clients.Payment == nil || clients.Warehouse == nil {
		t.Fatal("gRPC client 未全部创建")
	}
	if cfg.ServiceTarget("order") != "127.0.0.1:19003" {
		t.Fatalf("order target = %s", cfg.ServiceTarget("order"))
	}
	clients.Close()
}
