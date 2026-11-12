package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/client"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/router"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
	"github.com/BwCloudWeGo/bw-cli/pkg/observability"
)

var notifySignals = signal.Notify
var listenTCP = net.Listen
var serveHTTP = func(s *http.Server, l net.Listener) error { return s.Serve(l) }
var awaitShutdown = waitForShutdown

func main() {
	if err := run("configs/config.yaml"); err != nil {
		panic(err)
	}
}

func run(configPath string) error {
	// 构建依赖前先加载运行时配置。
	if err := config.InitGlobal(configPath); err != nil {
		return err
	}
	cfg := config.MustGlobal()
	gatewayServiceName := cfg.ServiceName("gateway")
	cfg.Log.Service = gatewayServiceName
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	defer log.Sync()
	observability.Register(gatewayServiceName, log)
	config.PrintSourceNotice(cfg, os.Stdout)

	clients, err := client.New(cfg, log)
	if err != nil {
		return fmt.Errorf("initialize grpc clients failed"+": %w", err)
	}
	defer clients.Close()

	engine := router.New(clients, log, cfg.Middleware)
	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTP.WriteTimeoutSeconds) * time.Second,
	}

	listener, err := listenTCP("tcp", addr)
	if err != nil {
		printStartupFailure(addr, err)
		return fmt.Errorf("gateway listen failed"+": %w", err)
	}
	printStartupSummary(cfg, addr)

	go func() {
		log.Info("gateway listening", zap.String("addr", addr))
		if err := serveHTTP(server, listener); err != nil && err != http.ErrServerClosed {
			log.Error("gateway stopped unexpectedly", zap.Error(err))
		}
	}()

	awaitShutdown(server, log)
	return nil
}

func printStartupFailure(addr string, err error) {
	fmt.Fprintf(os.Stderr, "\n[Gateway Start Failed]\n")
	fmt.Fprintf(os.Stderr, "  listen: %s\n", addr)
	fmt.Fprintf(os.Stderr, "  error: %v\n\n", err)
}

func printStartupSummary(cfg *config.Config, addr string) {
	host := cfg.HTTP.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	baseURL := fmt.Sprintf("http://%s:%d", host, cfg.HTTP.Port)
	fmt.Fprintf(os.Stdout, "\n[Gateway Started]\n")
	fmt.Fprintf(os.Stdout, "  service: %s\n", cfg.ServiceName("gateway"))
	fmt.Fprintf(os.Stdout, "  env: %s\n", cfg.App.Env)
	fmt.Fprintf(os.Stdout, "  listen: %s\n", addr)
	fmt.Fprintf(os.Stdout, "  http: %s\n", baseURL)
	fmt.Fprintf(os.Stdout, "  health: %s/healthz\n", baseURL)
	fmt.Fprintf(os.Stdout, "  api: %s/api/v1\n\n", baseURL)
}

func waitForShutdown(server *http.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	notifySignals(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("gateway shutting down")
	if err := server.Shutdown(ctx); err != nil {
		log.Error("gateway shutdown failed", zap.Error(err))
	}
}
