# 当前核心包独立复查

日期：2026-11-12，北京时间。独立测试角色 `/root/independent_tests`，重新读取 AGENTS v2 与原始题目01业务规则。当前分支 `main`，HEAD `8db219cce237c72cc208a40f812a5e42ed1e9b32`。开始测试时只有本轮输出文件未跟踪，没有生产代码未提交修改。未修改生产、提交、推送或部署。

实际执行 `go test ./internal/order/... ./internal/product/... ./internal/payment/... ./internal/warehouse/... -coverprofile=docs/remediation/core-recheck.out -count=1`，退出0。当前Windows PowerShell将未加引号的输出参数扩展名处理为无`.out`文件，运行后仅移动原始profile到要求的`core-recheck.out`，未修改覆盖率内容。原始输出及受测源码摘要见 `core-recheck-raw.txt`。

| 实际包 | 本轮语句覆盖率 |
| --- | ---: |
| order/dto | 100.0% |
| order/entity | 100.0% |
| order/handler | 93.2% |
| order/model | 100.0% |
| order/repo | 89.0% |
| order/service | 85.8% |
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

19个实际核心包均测试通过、达到85%数值阈值。范围仅为这四个业务域；不得推断全仓自编Go代码整体覆盖率达标。包内存在真实SQLite仓储/handler测试，不能把合并profile全部称为纯隔离单元测试覆盖率，更不能以此替代整体口径核验。

## 原报告三项缺陷定向回归

实际执行 `go test ./internal/order/service -run 'TestAudit(NonSuccessNotifyNeverMarksPaid|RefundReviewRequiresMeaningfulReason|CancelRetriesReleaseAfterServiceRestart)$' -count=1 -v`，退出0，三项均通过。

- 非成功通知：PENDING、PROCESSING、空值、小写success、CANCELLED均拒绝；断言订单保持待付款、无渠道记款号、库存Confirm调用0次。
- 审核原因：批准和拒绝均拒绝空串、普通空格、制表换行、全角空格；断言申请仍待审核、订单仍退款申请中、渠道退款0次、库存恢复0次。
- 取消补偿：首次Release RPC故障返回错误；使用同一仓储重新创建Service后再次取消成功，库存成功释放1次；后续重复取消不重复释放数量。此为服务层故障注入测试，重新创建服务对象不等于真实数据库或进程重启部署测试。

原报告三个可复现服务层缺陷在本轮受测范围已修复；不据此宣称完整五场景页面、教学部署、视频、历史过程或正式评分已通过。本轮未运行race、全仓覆盖率、浏览器或教学部署验收。
