# 浏览器逐项验收、竞态复测与回退演练证据（2026-11-13）

受测版本：`main HEAD bd1706d`（工作区干净）；部署：本机 docker compose 六容器 `127.0.0.1:18080`（`deploy/docker-compose.yml`，`ORDER_IMAGE=order-transaction:local-release`，健康检查 `healthz` 通过）。

## 1. 真实浏览器逐项验收（Playwright + 系统 Edge，全程录屏）

- 脚本与原始输出：`browse-results.json`、`browser-run.log`、`s5-result.json`；录屏：`walkthrough-s1s3s4s5.webm`（约 13 分钟，50MB）；截图：`shots/`。
- 工具版本：playwright 1.65.0（channel msedge，headless），Node v24.20.0。

| 场景 | 步骤与结果 |
| --- | --- |
| S1 正常成交 | buyer1 登录 → 商品页购买 1 件 → 我的订单付款 → 渠道成功通知入账（已付款）→ warehouse01 登记发货（弹窗输入顺丰速运/运单号）→ 待发货列表清除 → ops01 运营概览汇总可见 → 业务核查按订单号查到 4 行流水。截图 s1-01～s1-06。 |
| S3 售前退款 | buyer1 新单付款 → 申请退款（弹窗输入原因）→ cs01 拒绝（弹窗输入审核理由）→ 订单回到已付款 → 买家再次申请 → cs01 批准 → 订单变为已退款。截图 s3-01～s3-03。 |
| S4 迟到支付 | ops01 渠道演示配置迟到支付 660 秒（>600 秒付款期限）→ buyer1 下单立即支付 → 订单按 600 秒服务端期限超时关闭（真实等待，未缩短教学时钟）→ 渠道迟到通知到达后订单不复活、登记 `late_payment` 异常（PENDING，截图 s4-04）→ cs01 退回迟到支付 → REFUNDED（截图 s4-05）。 |
| S5 越权 | buyer2 仅可见本人订单（页面无 buyer1 订单号）→ 页面内 fetch：跨买家查询订单 403；匿名访问渠道配置 401（`s5-result.json`：`{"owner":403,"anonChannel":401}`，401=未携带凭据的预期拒绝）。截图 s5-01、s5-03。 |

结果：`BROWSER-VERIFY-RESULT` S1/S3/S4 全 PASS；S5 首轮因断言写死 403（实际 401）判 FAIL，经 `verify_s5.js` 复测语义正确判 PASS。失败过程原始输出保留于 `browser-run.log`（含前几次脚本缺陷导致的 FAIL，未删除）。

## 2. 最终提交竞态检测复测（绑定 bd1706d）

命令（Linux 容器，等效 CI 环境）：
`docker run --rm -e GOPROXY=https://goproxy.cn,direct -e GOCACHE=/tmp/gc -e GOMODCACHE=/tmp/gm -e CGO_ENABLED=1 -v <repo>:/src -w /src golang:1.25 go test -race -count=1 ./...`

原始输出：`race-head-container.log`。结果：67 个包全部 `ok`，无 `FAIL`、无 `DATA RACE`；四核心域与 gateway 全部包通过。

## 3. 回退演练（down 保留数据卷 → up → 数据保持）

- `docker compose -f deploy/docker-compose.yml down`（网络移除，`deploy_dbdata` 数据卷保留）
- `docker compose -f deploy/docker-compose.yml up -d` → `healthz` 返回 `{"status":"ok"}`
- 数据保持验证：buyer1 登录后 `orders/mine` total=9，其中浏览器验收 S1 订单 `ORD1794531065719cce1b5790e` 状态仍为 `SHIPPED`。

## 4. 冒烟（非破坏）

`healthz` OK；buyer1 登录成功（role=buyer）；`GET /products`、`GET /orders/mine` 均 200，不消耗库存。
