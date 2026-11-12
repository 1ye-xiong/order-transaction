package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	userdto "github.com/BwCloudWeGo/bw-cli/internal/user/dto"
	userhandler "github.com/BwCloudWeGo/bw-cli/internal/user/handler"
	userrepo "github.com/BwCloudWeGo/bw-cli/internal/user/repo"
	userservice "github.com/BwCloudWeGo/bw-cli/internal/user/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

var notifySignals = signal.Notify
var listenTCP = net.Listen

var serveGRPC = func(server *grpc.Server, listener net.Listener) error { return server.Serve(listener) }
var beginShutdown = func(server *grpc.Server, log *zap.Logger) { go shutdownOnSignal(server, log) }

func main() {
	if err := run("configs/config.yaml"); err != nil {
		panic(err)
	}
}

func run(configPath string) error {
	// 加载服务身份、数据库和日志配置。
	if err := config.InitGlobal(configPath); err != nil {
		return err
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("user")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	// Database.Open 会根据配置的驱动选择 SQLite、MySQL 或 PostgreSQL。
	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		return fmt.Errorf("open database failed"+": %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := userrepo.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate user database failed"+": %w", err)
	}

	repo := userrepo.NewGormRepository(db, log)
	svc := userservice.NewService(repo, userrepo.NewSHA256Hasher())
	if err := seedAccounts(context.Background(), svc, log); err != nil {
		return fmt.Errorf("seed accounts failed"+": %w", err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	userv1.RegisterUserServiceServer(server, userhandler.NewServer(svc, log))

	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.ServicePort("user", 9001))
	listener, err := listenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen failed"+": %w", err)
	}

	beginShutdown(server, log)
	log.Info("user service listening", zap.String("addr", addr))
	if err := serveGRPC(server, listener); err != nil {
		return fmt.Errorf("user service stopped unexpectedly"+": %w", err)
	}
	return nil
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	notifySignals(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
	log.Info("user service shutting down")
	server.GracefulStop()
}

// seedAccounts 预置题目 01 的教学账号；已存在时跳过，幂等可重复启动。
func seedAccounts(ctx context.Context, svc *userservice.Service, log *zap.Logger) error {
	accounts := []struct {
		account     string
		displayName string
		role        string
	}{
		{"buyer1", "买家一号", "buyer"},
		{"buyer2", "买家二号", "buyer"},
		{"buyer3", "买家三号", "buyer"},
		{"ops01", "运营", "ops"},
		{"warehouse01", "仓库", "warehouse"},
		{"cs01", "客服", "cs"},
	}
	for _, item := range accounts {
		_, err := svc.Register(ctx, userdto.RegisterCommand{
			Account:     item.account,
			DisplayName: item.displayName,
			Password:    "123456",
			Role:        item.role,
		})
		if err != nil {
			continue
		}
		log.Info("seeded account", zap.String("account", item.account), zap.String("role", item.role))
	}
	return nil
}
