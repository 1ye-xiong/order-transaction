#!/usr/bin/env bash
# 部署后冒烟：登录、查商品、下单、查询订单。
set -euo pipefail
BASE=${BASE:-http://127.0.0.1:8080/api/v1}

TOKEN=$(curl -fsS -X POST "$BASE/users/login" -H 'Content-Type: application/json' \
  -d '{"account":"buyer1","password":"123456"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
test -n "$TOKEN" || { echo "冒烟失败：未取得 token"; exit 1; }

curl -fsS "$BASE/products" -H "Authorization: Bearer $TOKEN" > /dev/null

PRODUCT_ID=$(curl -fsS "$BASE/products" -H "Authorization: Bearer $TOKEN" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)
ORDER=$(curl -fsS -X POST "$BASE/orders" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"quantity\":1}" | sed -n 's/.*"order_no":"\([^"]*\)".*/\1/p')
test -n "$ORDER" || { echo "冒烟失败：下单未返回订单号"; exit 1; }

curl -fsS "$BASE/orders/mine" -H "Authorization: Bearer $TOKEN" > /dev/null
echo "冒烟通过：登录/查商品/下单($ORDER)/查订单"
