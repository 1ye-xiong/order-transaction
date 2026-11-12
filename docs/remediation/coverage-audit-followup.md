# 覆盖率门禁独立审查（只读，2026-11-12）

依据 AGENTS.md、原始题目01及 `docs/remediation/coverage-current.out` 实际计算。执行 `go run ./tools/coveragegate docs/remediation/coverage-current.out` 返回失败。

- 全仓（仅排除 api/gen）：2696 / 5847 = 46.11%，距85%仍缺约2274条已覆盖语句。
- 核心订单、支付、仓库大多已过85%；当前核心低项为 `internal/product/service` 57.89%、`internal/product/repo` 79.75%。
- 最大非核心缺口：`pkg/scaffold` 39.20%（746条未覆盖）、`pkg/filex` 0%（247）、`pkg/esx` 0%（123）、`cmd/note` 0%（112）、`pkg/middleware` 0%（112），以及多个 cmd 服务入口和基础设施包0%。

HTTP五场景及集成覆盖不能代替题目要求的单元语句覆盖。若继续补测，应优先为上述基础设施与命令入口建立真实单元测试，并补足 product service/repo；不能通过排除自编代码或削弱门禁凑数。当前证据不支持“全仓85%通过”。
