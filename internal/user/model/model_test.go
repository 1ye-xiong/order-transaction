package model

import "testing"

func TestModel(t *testing.T) {
	if (UserModel{}).TableName() != "xls_user" {
		t.Fatal()
	}
}
