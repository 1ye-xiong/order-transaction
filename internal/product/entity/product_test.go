package entity

import "testing"

func TestProductAvailable(t *testing.T) {
	p := Product{StockTotal: 10, Reserved: 3}
	if got := p.Available(); got != 7 {
		t.Fatalf("available = %d, want 7", got)
	}
}

func TestNewProductValidation(t *testing.T) {
	cases := []struct {
		name       string
		prodName   string
		priceCents int64
		stockTotal int64
		wantErr    bool
	}{
		{"正常", "商品", 100, 5, false},
		{"名称为空", "", 100, 5, true},
		{"价格为0", "商品", 0, 5, true},
		{"库存为负", "商品", 100, -1, true},
	}
	for _, tc := range cases {
		_, err := NewProduct(tc.prodName, tc.priceCents, tc.stockTotal)
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: err = %v, wantErr = %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestChangePriceAndRestock(t *testing.T) {
	p, _ := NewProduct("商品", 100, 5)
	if err := p.ChangePrice(200); err != nil {
		t.Fatalf("ChangePrice: %v", err)
	}
	if p.PriceCents != 200 {
		t.Fatalf("price = %d, want 200", p.PriceCents)
	}
	if err := p.ChangePrice(0); err == nil {
		t.Fatal("ChangePrice(0) 应失败")
	}
	if err := p.Restock(5); err != nil {
		t.Fatalf("Restock: %v", err)
	}
	if p.StockTotal != 10 {
		t.Fatalf("stock = %d, want 10", p.StockTotal)
	}
	if err := p.Restock(-1); err == nil {
		t.Fatal("负数补货应失败")
	}
}

func TestSetOnSale(t *testing.T) {
	p, _ := NewProduct("商品", 100, 5)
	p.SetOnSale(false)
	if p.OnSale {
		t.Fatal("SetOnSale(false) 未生效")
	}
}

func TestReservationStates(t *testing.T) {
	if ReservationReserved != "reserved" || ReservationConfirmed != "confirmed" || ReservationReleased != "released" || ReservationRestored != "restored" {
		t.Fatal("占用状态常量被意外修改")
	}
}
