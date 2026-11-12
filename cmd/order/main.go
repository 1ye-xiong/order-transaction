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
	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	orderhandler "github.com/BwCloudWeGo/bw-cli/internal/order/handler"
	orderrepo "github.com/BwCloudWeGo/bw-cli/internal/order/repo"
	orderservice "github.com/BwCloudWeGo/bw-cli/internal/order/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

const defaultGRPCPort = 9100

// productStock 把商品服务的库存 RPC 封装成订单服务的 StockClient。
type productStock struct {
	client productv1.ProductServiceClient
}

func (p *productStock) Reserve(ctx context.Context, orderNo string, productID string, quantity int64) error {
	resp, err := p.client.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: orderNo, ProductId: productID, Quantity: quantity})
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return fmt.Errorf("%s", resp.GetMessage())
	}
	return nil
}

func (p *productStock) Release(ctx context.Context, orderNo string) error {
	_, err := p.client.ReleaseStock(ctx, &productv1.ReleaseStockRequest{OrderNo: orderNo})
	return err
}

func (p *productStock) Confirm(ctx context.Context, orderNo string) error {
	_, err := p.client.ConfirmStock(ctx, &productv1.ConfirmStockRequest{OrderNo: orderNo})
	return err
}

func (p *productStock) Restore(ctx context.Context, orderNo string) error {
	_, err := p.client.RestoreStock(ctx, &productv1.RestoreStockRequest{OrderNo: orderNo})
	return err
}

func (p *productStock) GetProduct(ctx context.Context, productID string) (string, int64, error) {
	product, err := p.client.GetProduct(ctx, &productv1.GetProductRequest{Id: productID})
	if err != nil {
		return "", 0, err
	}
	return product.GetName(), product.GetPriceCents(), nil
}

// channelGateway 把 payment 服务的 RPC 封装成订单服务的 ChannelClient。
type channelGateway struct {
	client paymentv1.PaymentServiceClient
}

func (c *channelGateway) Pay(ctx context.Context, orderNo string, buyerID string, amountCents int64) (string, error) {
	resp, err := c.client.Pay(ctx, &paymentv1.PayRequest{OrderNo: orderNo, BuyerId: buyerID, AmountCents: amountCents})
	if err != nil {
		return "", err
	}
	if !resp.GetAccepted() {
		return "", fmt.Errorf("%s", resp.GetMessage())
	}
	return resp.GetChannelTradeNo(), nil
}

func (c *channelGateway) Refund(ctx context.Context, refundNo string, tradeNo string, amountCents int64, reason string) (bool, string, error) {
	resp, err := c.client.Refund(ctx, &paymentv1.RefundRequest{RefundNo: refundNo, ChannelTradeNo: tradeNo, AmountCents: amountCents, Reason: reason})
	if err != nil {
		return false, "", err
	}
	return resp.GetSuccess(), resp.GetMessage(), nil
}

func main() {
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("order")
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
	if err := orderrepo.AutoMigrate(db); err != nil {
		log.Fatal("migrate order database failed", zap.Error(err))
	}

	productConn, err := grpc.Dial(cfg.ServiceTarget("product"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("dial product service failed", zap.Error(err))
	}
	defer productConn.Close()
	paymentConn, err := grpc.Dial(cfg.ServiceTarget("payment"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("dial payment service failed", zap.Error(err))
	}
	defer paymentConn.Close()

	repo := orderrepo.NewGormRepository(db)
	stock := &productStock{client: productv1.NewProductServiceClient(productConn)}
	channel := &channelGateway{client: paymentv1.NewPaymentServiceClient(paymentConn)}
	svc := orderservice.NewService(repo, channel, stock, cfg.Trade.OrderPayTimeout(), cfg.Trade.PayNotifySecret, log)

	// 超时关闭扫描：每秒扫描一次过期待付款订单（幂等）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := svc.CloseExpiredOrders(ctx); err != nil {
					log.Warn("close expired orders failed", zap.Error(err))
				} else if n > 0 {
					log.Info("closed expired orders", zap.Int("count", n))
				}
			}
		}
	}()

	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	orderv1.RegisterOrderServiceServer(server, orderhandler.NewServer(svc, log))

	port := cfg.ServicePort("order", defaultGRPCPort)
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.String("addr", addr), zap.Error(err))
	}

	go shutdownOnSignal(server, log)
	log.Info("order service listening", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		log.Fatal("order service stopped unexpectedly", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("order service shutting down")
	server.GracefulStop()
}
