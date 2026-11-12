package client

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	warehousev1 "github.com/BwCloudWeGo/bw-cli/api/gen/warehouse/v1"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

// Clients 聚合 HTTP gateway 使用的所有 gRPC client。
type Clients struct {
	User      userv1.UserServiceClient
	Note      notev1.NoteServiceClient
	Order     orderv1.OrderServiceClient
	Product   productv1.ProductServiceClient
	Payment   paymentv1.PaymentServiceClient
	Warehouse warehousev1.WarehouseServiceClient
	Config    *config.Config

	conns []*grpc.ClientConn
}

// New 连接配置的 gRPC 目标地址并创建强类型服务 client。
func New(cfg *config.Config, log *zap.Logger) (*Clients, error) {
	userTarget := cfg.ServiceTarget("user")
	noteTarget := cfg.ServiceTarget("note")
	orderTarget := cfg.ServiceTarget("order")
	productTarget := cfg.ServiceTarget("product")
	paymentTarget := cfg.ServiceTarget("payment")
	warehouseTarget := cfg.ServiceTarget("warehouse")

	dial := func(name string, target string) (*grpc.ClientConn, error) {
		conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("dial %s service: %w", name, err)
		}
		return conn, nil
	}

	userConn, err := dial("user", userTarget)
	if err != nil {
		return nil, err
	}
	noteConn, err := dial("note", noteTarget)
	if err != nil {
		userConn.Close()
		return nil, err
	}
	orderConn, err := dial("order", orderTarget)
	if err != nil {
		userConn.Close()
		noteConn.Close()
		return nil, err
	}
	productConn, err := dial("product", productTarget)
	if err != nil {
		userConn.Close()
		noteConn.Close()
		orderConn.Close()
		return nil, err
	}
	paymentConn, err := dial("payment", paymentTarget)
	if err != nil {
		userConn.Close()
		noteConn.Close()
		orderConn.Close()
		productConn.Close()
		return nil, err
	}
	warehouseConn, err := dial("warehouse", warehouseTarget)
	if err != nil {
		userConn.Close()
		noteConn.Close()
		orderConn.Close()
		productConn.Close()
		paymentConn.Close()
		return nil, err
	}

	log.Info("grpc clients initialized",
		zap.String("user_target", userTarget),
		zap.String("note_target", noteTarget),
		zap.String("order_target", orderTarget),
		zap.String("product_target", productTarget),
		zap.String("payment_target", paymentTarget),
		zap.String("warehouse_target", warehouseTarget),
	)
	return &Clients{
		User:      userv1.NewUserServiceClient(userConn),
		Note:      notev1.NewNoteServiceClient(noteConn),
		Order:     orderv1.NewOrderServiceClient(orderConn),
		Product:   productv1.NewProductServiceClient(productConn),
		Payment:   paymentv1.NewPaymentServiceClient(paymentConn),
		Warehouse: warehousev1.NewWarehouseServiceClient(warehouseConn),
		Config:    cfg,
		conns:     []*grpc.ClientConn{userConn, noteConn, orderConn, productConn, paymentConn, warehouseConn},
	}, nil
}

// Close 释放 gateway 的所有 gRPC client 连接。
func (c *Clients) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
}
