GO ?= go
PROTOC ?= protoc
PROTO_PATH ?= api/proto
PROTO_OUT ?= api/gen

export PROTOC
export PROTO_PATH
export PROTO_OUT

.PHONY: proto test tidy cover run-gateway run-user run-note run-cli install-cli install-bw-cli tools run-order run-product run-payment run-warehouse run-all smoke

tools:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	$(GO) run ./tools/protogen

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

run-user:
	$(GO) run ./cmd/user

run-note:
	$(GO) run ./cmd/note

run-gateway:
	$(GO) run ./cmd/gateway

run-cli:
	$(GO) run ./cmd/bw-cli

install-cli:
	$(GO) install ./cmd/bw-cli

install-bw-cli: install-cli

run-order:
	$(GO) run ./cmd/order

run-product:
	$(GO) run ./cmd/product

run-payment:
	$(GO) run ./cmd/payment

run-warehouse:
	$(GO) run ./cmd/warehouse

# 教学演示（缩短待付款时长为 60 秒）本地一键启动全部服务
run-all:
	$(GO) run ./cmd/user
	$(GO) run ./cmd/product
	$(GO) run ./cmd/order
	$(GO) run ./cmd/payment
	$(GO) run ./cmd/warehouse
	$(GO) run ./cmd/gateway

# 覆盖率：整体与核心业务包分别输出（门禁 >=85%）
cover:
	$(GO) test ./... -race -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | tail -1
	$(GO) test ./internal/order/... ./internal/product/... ./internal/payment/... ./internal/warehouse/... -race -coverprofile=coverage-core.out -covermode=atomic
	$(GO) tool cover -func=coverage-core.out | tail -1

smoke: run-all