package dto

import (
	"strings"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
)

func TestFromProduct(t *testing.T) {
	product := &entity.Product{
		ID: "P-1", Name: "限量商品", PriceCents: 10000, StockTotal: 10, Reserved: 3, OnSale: true,
		UpdatedAt: time.Date(2026, 4, 28, 10, 30, 0, 0, time.UTC),
	}
	dto := FromProduct(product)
	if dto.ID != "P-1" || dto.Name != "限量商品" || dto.PriceCents != 10000 {
		t.Fatalf("dto = %+v", dto)
	}
	if dto.StockTotal != 10 || dto.Reserved != 3 || dto.Available != 7 || !dto.OnSale {
		t.Fatalf("库存字段错误: %+v", dto)
	}
	if dto.UpdatedAt != "2026-04-28 10:30:00" {
		t.Fatalf("时间格式 = %s", dto.UpdatedAt)
	}
}

func TestFromProducts(t *testing.T) {
	empty := FromProducts(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("空列表应返回空切片, got %#v", empty)
	}
	products := []*entity.Product{
		{ID: "P-1", Name: "a", PriceCents: 1, StockTotal: 1, OnSale: true, UpdatedAt: time.Now()},
		{ID: "P-2", Name: "b", PriceCents: 2, StockTotal: 2, Reserved: 2, OnSale: false, UpdatedAt: time.Now()},
	}
	items := FromProducts(products)
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Available != 1 || !strings.HasPrefix(items[0].ID, "P") {
		t.Fatalf("item 0 = %+v", items[0])
	}
	if items[1].Available != 0 || items[1].OnSale {
		t.Fatalf("item 1 = %+v", items[1])
	}
}
