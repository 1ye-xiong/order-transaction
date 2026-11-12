package model

import "testing"

func TestTableNames(t *testing.T) {
	if (ProductModel{}).TableName() != "products" {
		t.Fatal("products 表名错误")
	}
	if (ReservationModel{}).TableName() != "product_reservations" {
		t.Fatal("product_reservations 表名错误")
	}
}
