# 最终冻结版本核心包独立复测

日期：2026-11-12。测试角色独立执行；被测版本为 HEAD `8db219cce237c72cc208a40f812a5e42ed1e9b32` 加当前尚未提交整改工作区，并非该提交的干净快照。生产范围参考同期 `followup-source-manifest.json`；本轮未修改生产代码。

执行命令：

```powershell
go test ./internal/order/... ./internal/product/... ./internal/payment/... ./internal/warehouse/... '-coverprofile=docs/remediation/followup-core.out' -count=1
```

结果：19 个实际核心包全部通过，且每包覆盖率均 ≥85%。四业务域合并语句覆盖率为 **89.1%**；该数值不代表全仓库覆盖率。无排除生产文件、跳过关键测试或弱化既有断言。

| 包 | 覆盖率 |
|---|---:|
| order/dto | 100.0% |
| order/entity | 100.0% |
| order/handler | 93.2% |
| order/model | 100.0% |
| order/repo | 89.3% |
| order/service | 86.7% |
| product/dto | 100.0% |
| product/entity | 100.0% |
| product/handler | 86.8% |
| product/model | 100.0% |
| product/repo | 85.5% |
| product/service | 87.9% |
| payment/entity | 100.0% |
| payment/handler | 100.0% |
| payment/model | 100.0% |
| payment/repo | 91.7% |
| payment/service | 86.7% |
| warehouse/handler | 92.9% |
| warehouse/service | 100.0% |

本轮新增 `followup_errors_test.go` 验证退款申请落库失败后的状态回滚及回滚失败上报、取消/超时关闭后查询失败、渠道受理但支付流水失败、FAILED 通知流水失败、异常记录与重复通知流水持久化失败均不得被成功确认。此前本轮事务触发器回归与扫描失败不饿死其他订单回归也包含在上述执行中。

证据：`followup-core-raw.txt` 保留逐包原始结果；`followup-core.out` 保留覆盖率 profile，SHA256 为 `3D798852F2D5C934701D50FAF9788CC9EAF011F2F458B0F4276B4B8AEC1365CA`。旧 `core-recheck.*` 保留为历史记录，不作为本次冻结后结果。

范围说明：此套测试包含测试替身、SQLite 仓储集成及 gRPC handler 合约测试，不能将全部覆盖率称为纯隔离单元测试。竞态、全仓测试、vet 与真实 HTTP 验收分别参考同期 `followup-order-race.txt`、`followup-all-tests.txt`、`followup-vet-final.txt` 与 `followup-acceptance.md`，本报告不将覆盖率当作部署或浏览器验收证据。最终评分仍由原评分标准和评审决定。
