package model

import "testing"

func TestTableNames(t *testing.T) {
	if (ChannelConfigModel{}).TableName() != "channel_config" {
		t.Fatal("channel_config 表名错误")
	}
	if (ChannelRecordModel{}).TableName() != "channel_records" {
		t.Fatal("channel_records 表名错误")
	}
}
