package model

import "testing"

func TestTableNames(t *testing.T) {
	if (OrderModel{}).TableName() != "orders" {
		t.Fatal("orders 表名错误")
	}
	if (OrderRecordModel{}).TableName() != "order_records" {
		t.Fatal("order_records 表名错误")
	}
	if (RefundRequestModel{}).TableName() != "refund_requests" {
		t.Fatal("refund_requests 表名错误")
	}
	if (PayExceptionModel{}).TableName() != "pay_exceptions" {
		t.Fatal("pay_exceptions 表名错误")
	}
}
