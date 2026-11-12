package main

import (
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"go.uber.org/zap"
	"net/http"
	"os"
	"syscall"
	"testing"
)

func TestGatewayHelpers(t *testing.T) {
	cfg := &config.Config{}
	printStartupSummary(cfg, "0.0.0.0:8080")
	printStartupFailure("x", http.ErrServerClosed)
	_ = zap.NewNop()
}

func TestWaitForShutdown(t *testing.T) {
	old := notifySignals
	defer func() { notifySignals = old }()
	notifySignals = func(ch chan<- os.Signal, _ ...os.Signal) { ch <- syscall.SIGTERM }
	waitForShutdown(&http.Server{}, zap.NewNop())
}
