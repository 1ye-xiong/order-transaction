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
	"google.golang.org/grpc/credentials/insecure"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	paymenthandler "github.com/BwCloudWeGo/bw-cli/internal/payment/handler"
	paymentrepo "github.com/BwCloudWeGo/bw-cli/internal/payment/repo"
	paymentservice "github.com/BwCloudWeGo/bw-cli/internal/payment/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

const serviceName = "payment-service"
const defaultGRPCPort = 9103

// orderNotifier 把订单服务的支付通知 RPC 封装成渠道的 Notifier。
type orderNotifier struct {
	client orderv1.OrderServiceClient
}

func (n *orderNotifier) SendPaymentNotify(ctx context.Context, req paymentservice.NotifyRequest) error {
	_, err := n.client.HandlePaymentNotify(ctx, &orderv1.HandlePaymentNotifyRequest{
		OrderNo:        req.OrderNo,
		AmountCents:    req.AmountCents,
		ChannelTradeNo: req.ChannelTradeNo,
		Status:         req.Status,
		NotifiedAtUnix: req.NotifiedAtUnix,
		Signature:      req.Signature,
	})
	return err
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
	cfg.Log.Service = cfg.ServiceName("payment")
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
	if err := paymentrepo.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate payment database failed"+": %w", err)
	}

	// 渠道需要把支付通知回传给订单服务。
	orderConn, err := grpc.Dial(cfg.ServiceTarget("order"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial order service failed"+": %w", err)
	}
	defer orderConn.Close()

	notifier := &orderNotifier{client: orderv1.NewOrderServiceClient(orderConn)}
	repo := paymentrepo.NewGormRepository(db)
	svc := paymentservice.NewService(repo, notifier, cfg.Trade.PayNotifySecret, log)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	paymentv1.RegisterPaymentServiceServer(server, paymenthandler.NewServer(svc, log))

	port := cfg.ServicePort("payment", defaultGRPCPort)
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, port)
	listener, err := listenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen failed"+": %w", err)
	}

	beginShutdown(server, log)
	log.Info("payment service listening", zap.String("addr", addr))
	if err := serveGRPC(server, listener); err != nil {
		return fmt.Errorf("payment service stopped unexpectedly"+": %w", err)
	}
	return nil
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	notifySignals(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
	log.Info("payment service shutting down")
	server.GracefulStop()
}
