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

	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	productdto "github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	producthandler "github.com/BwCloudWeGo/bw-cli/internal/product/handler"
	productrepo "github.com/BwCloudWeGo/bw-cli/internal/product/repo"
	productservice "github.com/BwCloudWeGo/bw-cli/internal/product/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

const defaultGRPCPort = 9101

// seedProducts 预置教学商品；按 ID 幂等。
func seedProducts(ctx context.Context, svc *productservice.Service, log *zap.Logger) {
	existing, err := svc.List(ctx, true)
	if err != nil {
		log.Warn("list products for seeding failed", zap.Error(err))
		return
	}
	if len(existing) > 0 {
		return
	}
	seeds := []struct {
		name       string
		priceCents int64
		stock      int64
	}{
		{"限量款蓝牙耳机", 10000, 5},
		{"运动水杯", 5000, 8},
	}
	for _, item := range seeds {
		created, err := svc.Create(ctx, productdto.CreateCommand{Name: item.name, PriceCents: item.priceCents, StockTotal: item.stock})
		if err != nil {
			log.Warn("seed product failed", zap.String("name", item.name), zap.Error(err))
			continue
		}
		log.Info("seeded product", zap.String("id", created.ID), zap.String("name", item.name))
	}
}

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
	if err := config.InitGlobal(configPath); err != nil {
		return err
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("product")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		return fmt.Errorf("open database failed"+": %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := productrepo.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate product database failed"+": %w", err)
	}

	repo := productrepo.NewGormRepository(db, log)
	svc := productservice.NewService(repo)
	seedProducts(context.Background(), svc, log)

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerAuthInterceptor(cfg.Trade.InternalRPCSecret),
		grpcx.UnaryServerInterceptor(log)))
	productv1.RegisterProductServiceServer(server, producthandler.NewServer(svc, log))

	port := cfg.ServicePort("product", defaultGRPCPort)
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, port)
	listener, err := listenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen failed"+": %w", err)
	}

	beginShutdown(server, log)
	log.Info("product service listening", zap.String("addr", addr))
	if err := serveGRPC(server, listener); err != nil {
		return fmt.Errorf("product service stopped unexpectedly"+": %w", err)
	}
	return nil
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	notifySignals(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
	log.Info("product service shutting down")
	server.GracefulStop()
}
