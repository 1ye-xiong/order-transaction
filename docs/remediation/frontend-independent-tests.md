# 新前端独立测试记录

日期：2026-11-12（北京时间）。角色：独立测试 `/root/independent_tests`。约束：已重新读取项目 AGENTS v2、tester 入口、原始《Go项目业务场景题.md》通用要求及题目01；本次仅新增测试和记录，保留既有后端修改。当前分支 `repair/topic01`，基础提交 `f2b20529e3ab908b3b4af61ef2abf0c1f3e98791`，受测为有未提交修改的工作区，不能仅以 HEAD 表示本次版本。相关源码 SHA256 见 `frontend-independent-raw.txt`。原始通用要求 DOCX 本轮未解析，不用项目摘要替代原始 Markdown。

## 实际执行

| 检查 | 结果 | 范围与限制 |
| --- | --- | --- |
| `pnpm exec tsc --noEmit` | 退出0 | TypeScript严格类型检查 |
| `pnpm build` | 退出0 | Vite真实生产打包，输出 `web/dist`；不是浏览器验收 |
| `node --test web/frontend/api.contract.test.mjs` | 6条通过，退出0 | 实际导入 `api.ts`，fetch为受控HTTP响应替身 |
| `go test ./internal/gateway/router -run TestFrontend -count=1 -v` | 2条通过，退出0 | 实际路由注册；构建首页、资源、SPA路径、legacy、API 404和未构建回退 |
| `go test ./internal/gateway/handler ./internal/order/dto -run TestFrontend -count=1 -v` | 2条通过，退出0 | 第2页参数透传、默认分页、非法参数拦截；付款到期时间含时区 |

API契约断言覆盖统一 `data` 包装解包、登录token及服务端role、CurrentUser直接用户载荷、Bearer JWT、分页查询字符串、403错误原因、HTTP200下发货/支付/异常退款业务失败不得当成功、审核返回REFUND_FAILED保留失败状态、整数分和不安全金额校验。对照实际gateway LoginResponse、CurrentUser和warehouse ShipResponse，字段一致。

分页测试捕获真实handler发往下游的请求，第2页正确为page=2/page_size=20；缺省1/50；page=0、-1、非数字、超int32和page_size=0、101、非数字均400且不调用下游。时间测试分别使用UTC、UTC+8、UTC-7，RFC3339解析后均为同一到期时刻。

## 发现与复查

1. 初查订单状态翻译缺少真实 `REFUND_REQUESTED`，已交开发修复；现源码已包含“退款待审核”。
2. 初查写操作开始前未失效在途轮询，存在旧响应短暂覆盖风险。已交开发修复；现源码 action 开头递增generation，轮询检查locked，退出按钮busy时禁用。现有refresh还在页面/账号切换时递增generation，落状态前核对generation。此项是源码复查，尚无挂载React组件控制乱序响应的运行测试。
3. 运营渠道配置及核查页不自动轮询，避免覆盖正在填写的配置和已查流水；买家订单有20条分页和总数边界按钮。页码改变触发新refresh，旧代响应不能覆盖新代。
4. 审核批准提示为“操作已受理，状态以服务端结果为准”，并展示服务端状态，不把批准等同退款成功；发货/付款/异常退款的false响应由api抛错。登录后再请求CurrentUser确定服务端身份，不依赖客户端指定角色。

## 未验证及交付边界

未执行浏览器点击、完整五场景React交互、视觉可用性、真实设备适配和React挂载后的轮询乱序模拟；不得将类型/打包/契约测试写成页面闭环通过。fetch替身不代替线上服务接口验收。未知role的防御处理、本机Node24要求和发布制品版本仍需最终交付核对。真实教学部署、视频和历史过程证据不在本轮测试结论内。

测试文件：`web/frontend/api.contract.test.mjs`、`internal/gateway/router/frontend_static_test.go`、`internal/gateway/handler/frontend_pagination_test.go`、`internal/order/dto/frontend_expiry_test.go`。原始输出见 `frontend-independent-raw.txt`，分页/时区输出由本次工具调用保留并补入同文件。
