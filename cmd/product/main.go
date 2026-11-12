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

func main() {
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("product")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
	if err != nil {
		log.Fatal("open database failed", zap.Error(err))
	}
	if err := productrepo.AutoMigrate(db); err != nil {
		log.Fatal("migrate product database failed", zap.Error(err))
	}

	repo := productrepo.NewGormRepository(db, log)
	svc := productservice.NewService(repo)
	seedProducts(context.Background(), svc, log)

	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	productv1.RegisterProductServiceServer(server, producthandler.NewServer(svc, log))

	port := cfg.ServicePort("product", defaultGRPCPort)
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.String("addr", addr), zap.Error(err))
	}

	go shutdownOnSignal(server, log)
	log.Info("product service listening", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		log.Fatal("product service stopped unexpectedly", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("product service shutting down")
	server.GracefulStop()
}
