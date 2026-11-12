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
	"google.golang.org/grpc/metadata"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	warehousev1 "github.com/BwCloudWeGo/bw-cli/api/gen/warehouse/v1"
	warehousehandler "github.com/BwCloudWeGo/bw-cli/internal/warehouse/handler"
	warehouseservice "github.com/BwCloudWeGo/bw-cli/internal/warehouse/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

const defaultGRPCPort = 9104

// orderGateway 把订单服务的发货 RPC 封装成仓库服务的 OrderClient。
type orderGateway struct {
	client orderv1.OrderServiceClient
}

func (g *orderGateway) ListShippable(ctx context.Context) ([]warehouseservice.ShippableOrder, error) {
	resp, err := g.client.ListShippableOrders(forwardIdentity(ctx), &orderv1.ListShippableOrdersRequest{})
	if err != nil {
		return nil, err
	}
	orders := make([]warehouseservice.ShippableOrder, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		orders = append(orders, warehouseservice.ShippableOrder{
			OrderNo:     item.GetOrderNo(),
			BuyerID:     item.GetBuyerId(),
			ProductName: item.GetProductName(),
			Quantity:    item.GetQuantity(),
			AmountCents: item.GetAmountCents(),
			PaidAt:      item.GetPaidAt(),
		})
	}
	return orders, nil
}

func (g *orderGateway) Ship(ctx context.Context, orderNo string, carrier string, trackingNo string, operator string) error {
	resp, err := g.client.ShipOrder(forwardIdentity(ctx), &orderv1.ShipOrderRequest{OrderNo: orderNo, Carrier: carrier, TrackingNo: trackingNo})
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	_ = resp
	return nil
}

func main() {
	if err := config.InitGlobal("configs/config.yaml"); err != nil {
		panic(err)
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("warehouse")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	orderConn, err := grpc.Dial(cfg.ServiceTarget("order"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("dial order service failed", zap.Error(err))
	}
	defer orderConn.Close()

	svc := warehouseservice.NewService(&orderGateway{client: orderv1.NewOrderServiceClient(orderConn)})
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	warehousev1.RegisterWarehouseServiceServer(server, warehousehandler.NewServer(svc))

	port := cfg.ServicePort("warehouse", defaultGRPCPort)
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.String("addr", addr), zap.Error(err))
	}

	go shutdownOnSignal(server, log)
	log.Info("warehouse service listening", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		log.Fatal("warehouse service stopped unexpectedly", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("warehouse service shutting down")
	server.GracefulStop()
}

// Only forward known identity fields from the trusted internal hop.
func forwardIdentity(ctx context.Context) context.Context {
	user, role := grpcx.IdentityFromContext(ctx)
	return metadata.AppendToOutgoingContext(ctx, grpcx.MetadataUserID, user, grpcx.MetadataUserRole, role, grpcx.MetadataRequestID, grpcx.RequestIDFromContext(ctx))
}
