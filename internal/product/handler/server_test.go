package handler

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/product/repo"
	"github.com/BwCloudWeGo/bw-cli/internal/product/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
)

func ctxWithRole(role string) context.Context {
	md := metadata.Pairs(grpcx.MetadataUserID, "u1", grpcx.MetadataUserRole, role)
	return metadata.NewIncomingContext(context.Background(), md)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := repo.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewServer(service.NewService(repo.NewGormRepository(db)), nil)
}

func TestProductRoleGuard(t *testing.T) {
	s := newTestServer(t)
	_, err := s.Create(ctxWithRole("buyer"), &productv1.CreateProductRequest{Name: "x", PriceCents: 1, StockTotal: 1})
	if apperrors.GRPCCode(err) != codes.PermissionDenied {
		t.Fatalf("buyer should be denied: %v", err)
	}
	_, err = s.Update(ctxWithRole("buyer"), &productv1.UpdateProductRequest{Id: "x"})
	if apperrors.GRPCCode(err) != codes.PermissionDenied {
		t.Fatalf("buyer should be denied on update: %v", err)
	}
}

func TestProductCRUD(t *testing.T) {
	s := newTestServer(t)
	created, err := s.Create(ctxWithRole("ops"), &productv1.CreateProductRequest{Name: "e", PriceCents: 10000, StockTotal: 5})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	updated, err := s.Update(ctxWithRole("ops"), &productv1.UpdateProductRequest{Id: created.Id, SetPrice: true, PriceCents: 8000})
	if err != nil || updated.PriceCents != 8000 {
		t.Fatalf("update: %v / %+v", err, updated)
	}
	if _, err := s.Update(ctxWithRole("ops"), &productv1.UpdateProductRequest{Id: "NOPE", SetPrice: true, PriceCents: 1}); apperrors.GRPCCode(err) != codes.NotFound {
		t.Fatalf("unknown product should be NotFound, got %v", err)
	}
	list, err := s.List(ctxWithRole("buyer"), &productv1.ListProductsRequest{})
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("list: %v / %d", err, len(list.Items))
	}
	if _, err := s.List(context.Background(), &productv1.ListProductsRequest{IncludeOffSale: true}); err != nil {
		t.Fatalf("anonymous list should be allowed: %v", err)
	}
	got, err := s.GetProduct(ctxWithRole("buyer"), &productv1.GetProductRequest{Id: created.Id})
	if err != nil || got.Name != "e" {
		t.Fatalf("get: %v / %+v", err, got)
	}
	if _, err := s.GetProduct(ctxWithRole("buyer"), &productv1.GetProductRequest{Id: "NOPE"}); apperrors.GRPCCode(err) != codes.NotFound {
		t.Fatalf("unknown get should be NotFound, got %v", err)
	}
}

func TestStockOps(t *testing.T) {
	s := newTestServer(t)
	created, _ := s.Create(ctxWithRole("ops"), &productv1.CreateProductRequest{Name: "c", PriceCents: 5000, StockTotal: 3})
	ctx := context.Background()

	res, _ := s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-1", ProductId: created.Id, Quantity: 2})
	if !res.Ok {
		t.Fatalf("reserve = %+v", res)
	}
	dup, _ := s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-1", ProductId: created.Id, Quantity: 2})
	if !dup.Ok {
		t.Fatal("duplicate reserve should be idempotent")
	}
	fail, _ := s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-2", ProductId: created.Id, Quantity: 2})
	if fail.Ok || fail.Message == "" {
		t.Fatalf("insufficient stock should fail with reason: %+v", fail)
	}
	rel, _ := s.ReleaseStock(ctx, &productv1.ReleaseStockRequest{OrderNo: "ORD-1"})
	if !rel.Ok {
		t.Fatal("release should succeed")
	}
	rel2, _ := s.ReleaseStock(ctx, &productv1.ReleaseStockRequest{OrderNo: "ORD-1"})
	if !rel2.Ok {
		t.Fatal("duplicate release should succeed")
	}
	_, _ = s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-3", ProductId: created.Id, Quantity: 1})
	conf, _ := s.ConfirmStock(ctx, &productv1.ConfirmStockRequest{OrderNo: "ORD-3"})
	if !conf.Ok {
		t.Fatal("confirm should succeed")
	}
	rest, _ := s.RestoreStock(ctx, &productv1.RestoreStockRequest{OrderNo: "ORD-3"})
	if !rest.Ok {
		t.Fatal("restore should succeed")
	}
	bad, _ := s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-BAD", ProductId: "NOPE", Quantity: 1})
	if bad.Ok || bad.Message == "" {
		t.Fatal("unknown product reserve should fail with reason")
	}
}

// 预留失败原因映射：非法数量、下架商品与占用状态流转失败。
func TestStockFailureBranches(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	created, _ := s.Create(ctxWithRole("ops"), &productv1.CreateProductRequest{Name: "e", PriceCents: 9000, StockTotal: 3})

	invalid, _ := s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-INV", ProductId: created.Id, Quantity: 0})
	if invalid.Ok || invalid.Message == "" {
		t.Fatalf("数量为 0 应失败并带原因: %+v", invalid)
	}

	if _, err := s.Update(ctxWithRole("ops"), &productv1.UpdateProductRequest{Id: created.Id, SetOnSale: true, OnSale: false}); err != nil {
		t.Fatalf("off sale: %v", err)
	}
	offSale, _ := s.ReserveStock(ctx, &productv1.ReserveStockRequest{OrderNo: "ORD-OFF", ProductId: created.Id, Quantity: 1})
	if offSale.Ok || offSale.Message == "" {
		t.Fatalf("下架商品应预留失败: %+v", offSale)
	}

	// 未知订单确认扣减应失败（占用缺失）
	missing, _ := s.ConfirmStock(ctx, &productv1.ConfirmStockRequest{OrderNo: "ORD-NONE"})
	if missing.Ok {
		t.Fatal("无占用记录时确认应失败")
	}

	// 未知订单释放/恢复幂等成功
	rel, _ := s.ReleaseStock(ctx, &productv1.ReleaseStockRequest{OrderNo: "ORD-NONE"})
	if !rel.Ok {
		t.Fatal("未知订单释放应幂等成功")
	}
	restore, _ := s.RestoreStock(ctx, &productv1.RestoreStockRequest{OrderNo: "ORD-NONE"})
	if !restore.Ok {
		t.Fatal("未知订单恢复应幂等成功")
	}
}
