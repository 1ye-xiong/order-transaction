package handler

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
)

func identityCtx(userID string, role string) context.Context {
	md := metadata.Pairs(
		grpcx.MetadataUserID, userID,
		grpcx.MetadataUserRole, role,
	)
	return metadata.NewIncomingContext(context.Background(), md)
}

// 角色校验：买家专属接口拒绝其他角色；无身份拒绝。
func TestRequireBuyer(t *testing.T) {
	if _, err := requireBuyer(identityCtx("u1", "buyer")); err != nil {
		t.Fatalf("买家应通过: %v", err)
	}
	for _, role := range []string{"ops", "cs", "warehouse", ""} {
		if _, err := requireBuyer(identityCtx("u1", role)); err == nil {
			t.Fatalf("角色 %s 不应通过买家校验", role)
		}
	}
	if _, err := requireBuyer(context.Background()); err == nil {
		t.Fatal("无身份请求应被拒绝")
	}
}

// 员工角色校验：允许名单内的角色通过。
func TestRequireRole(t *testing.T) {
	if _, err := requireRole(identityCtx("u2", "cs"), "cs"); err != nil {
		t.Fatalf("客服应通过: %v", err)
	}
	if _, err := requireRole(identityCtx("u2", "buyer"), "cs", "ops"); err == nil {
		t.Fatal("买家不应通过客服/运营校验")
	}
	if _, err := requireRole(identityCtx("u2", "ops"), "cs", "ops"); err != nil {
		t.Fatalf("运营应通过: %v", err)
	}
}

// 错误映射：领域错误 → 应用错误 kind。
func TestMapOrderError(t *testing.T) {
	cases := map[error]apperrors.Kind{
		entity.ErrOrderNotFound:      apperrors.KindNotFound,
		entity.ErrNotOrderOwner:      apperrors.KindForbidden,
		entity.ErrInvalidQuantity:    apperrors.KindInvalidArgument,
		entity.ErrReasonRequired:     apperrors.KindInvalidArgument,
		entity.ErrTrackingNoRequired: apperrors.KindInvalidArgument,
	}
	for err, kind := range cases {
		mapped := mapOrderError(err)
		appErr, ok := apperrors.As(mapped)
		if !ok || appErr.Kind != kind {
			t.Fatalf("%v 应映射为 %s, got %+v", err, kind, mapped)
		}
	}
	// 其他错误归入冲突类
	appErr, _ := apperrors.As(mapOrderError(context.DeadlineExceeded))
	if appErr.Kind != apperrors.KindConflict {
		t.Fatalf("默认错误应映射为 CONFLICT, got %+v", appErr)
	}
}

// 订单 DTO → proto 转换字段完整。
func TestToOrderProto(t *testing.T) {
	item := dtoOrderFixture()
	proto := toOrderProto(item)
	if proto.OrderNo != item.OrderNo || proto.Status != item.Status || proto.AmountCents != item.AmountCents {
		t.Fatalf("proto = %+v", proto)
	}
}

func dtoOrderFixture() *dto.OrderDTO {
	return &dto.OrderDTO{
		OrderNo: "ORD-1", BuyerID: "buyer1", ProductName: "商品",
		UnitPriceCents: 10000, Quantity: 1, AmountCents: 10000, Status: entity.StatusPaid,
	}
}
