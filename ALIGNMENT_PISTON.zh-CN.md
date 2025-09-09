# CodeRunr 与 Piston API 对齐计划（草案）

目标：使 `coderunr/api`（Go）在 HTTP 与 WebSocket 行为、数据契约、错误语义与运行时/包管理方面，与 `piston/api`（Node）保持高度兼容，降低客户端适配成本，并在不牺牲稳定性的前提下逐步落地。

适用范围：HTTP API（健康、运行时、执行、包管理）、WebSocket 交互执行、限额与资源参数、内容协商与错误处理、配置/启动、测试与验收。

---

## 优先级说明
- High：直接影响客户端契约/互通性，优先修复
- Medium：影响开发体验、语义一致性或边界条件
- Low：工程健壮性/可运维性优化

---

## HTTP 接口对齐 TODO

- [x] High 统一时间字段单位为毫秒（数值）
  - 执行响应中的 `duration`、`compile.duration`、`run.duration` 等以数字毫秒返回（内部仍用 `time.Duration`，JSON 输出转换为 `int` 毫秒）。
  - 涉及：`internal/types/types.go`、`internal/handler/handler.go`、`internal/job/job.go`

- [x] High Execute 请求/响应字段名与结构对齐
  - 请求：支持 Piston 字段与默认值（`files[].name/content/encoding`、`args[]`、`stdin`、`compile_timeout`、`run_timeout`、内存/CPU 限制），并允许覆盖服务器默认。
  - 响应：与 Piston 兼容；当存在 `signal` 时 `code=null`；保留扩展的 `cpu_time/wall_time/memory/status/message`（毫秒/字节）。
  - 附带（可选）`limits` 回显（timeouts/cpu_times/memory_limits，毫秒/字节），便于调试；不影响兼容性（omitempty）。
  - 涉及：`internal/types/types.go`、`internal/handler/handler.go`

- [x] High Content-Type 宽容
  - 接受 `application/json` 及带 `charset` 的常见变体；仅在需要 JSON 的 POST/DELETE 路由上校验，不影响 GET。
  - 涉及：`internal/middleware/middleware.go`、`cmd/server/main.go`

- [x] Medium 状态码与错误体对齐
  - 包安装成功返回 `201 Created`，卸载成功返回 `204 No Content`；错误 `400/404/409` 按语义返回。
  - 错误响应体统一 `{ message: "..." }`（过渡期保留兼容字段）。
  - 已补充/调整 E2E 覆盖。
  - 涉及：`internal/handler/packages.go`、`internal/handler/handler.go`

- [x] Medium Runtimes 元数据字段对齐
  - `runtimes` 响应包含 `language`、`version`、`aliases`；支持 `provides` 展开；补充 `platform/os/arch` 字段（从 `pkg-info.json.build_platform` 解析，形如 `linux/amd64`；未知则留空）。
  - 涉及：`internal/runtime/runtime.go`（解析/填充）、`internal/types/types.go`（公开字段）、`internal/handler/handler.go`（响应透出）

---

## WebSocket 对齐 TODO

- [x] High 初始化消息结构对齐（客户端 → 服务器）
  - 已支持 Piston 顶层字段与旧 `payload` 结构的双轨兼容，E2E 用例通过。
  - 涉及：`internal/handler/websocket.go`、`internal/types/types.go`

- [x] High 错误事件格式对齐（服务器 → 客户端）
  - 统一 `{ type: "error", message: "..." }`，避免使用其他键名承载错误文本。
  - 未知消息类型现在返回错误但不主动断开连接，便于客户端恢复/继续会话。
  - 涉及：`internal/handler/websocket.go`

- [x] High 数据事件/阶段事件对齐与顺序保证
  - 增加 `init_ack`；阶段事件采用 `stage_start`/`stage_end`；顺序保证：`runtime` → `init_ack` → `stage_start(run|compile)` → `data*` → `stage_end`。
  - `data` 事件：`{ type: "data", stream: "stdout"|"stderr", data: "..." }`。
  - E2E：新增事件顺序校验用例，并更新现有 WS 用例（输出上限、顶层 init、基础执行、语法错误）。
  - 涉及：`internal/handler/websocket.go`、`internal/job/job.go`、`tests/e2e/websocket_test.go`

- [x] High 流式输出上限（防失控）
  - 已实现合并 stdout/stderr 的输出预算；超限截断并发送错误，并终止进程；E2E 覆盖。
  - 涉及：`internal/job/job.go`、`internal/handler/websocket.go`

- [x] Medium stdin 换行策略一致
  - 现按原样透传 `stdin`，不再自动追加换行；E2E 覆盖。
  - 涉及：`internal/job/job.go`

- [x] Medium 通道关闭安全
  - 避免向已关闭通道写入；退出/错误路径清理 goroutine，保证有序收尾。
  - WebSocket `sendMessage` 增加互斥与 `closed` 检查，防止关闭后发送导致 panic；事件发送协程在关闭时安全退出。
  - 涉及：`internal/handler/websocket.go`

---

## 运行时与包管理对齐 TODO

- [ ] Medium 包安装/卸载行为与响应
  - 安装完成刷新运行时缓存；卸载后 `runtimes` 列表及时反映变化；状态码对齐（见上）。
  - 涉及：`internal/service/package.go`、`internal/runtime/runtime.go`、`internal/handler/packages.go`

- [ ] Medium 版本匹配与别名/提供能力
  - semver 解析与匹配策略（含预发布）对齐；`aliases`、`provides` 的查询/匹配/展示一致。
  - 涉及：`internal/runtime/runtime.go`

---

## 中间件与错误处理对齐 TODO

- [x] High JSON 中间件作用范围
  - 仅在 POST/DELETE JSON 路由应用，GET 不受限；严格但宽容常见 `Content-Type` 变体。
  - 涉及：`internal/middleware/middleware.go`、`cmd/server/main.go`

- [x] Medium 统一错误映射与日志
  - 各路由错误响应体统一 `{ message: "..." }`；包管理、执行请求、JSON 中间件统一返回 JSON 错误体（如 400/404/409/415/500）。
  - 保留兼容键位于 WS 错误事件；日志覆盖错误路径并保留栈信息（由 Recovery/Logger 中间件负责）。
  - 涉及：`internal/handler/packages.go`、`internal/handler/handler.go`、`internal/middleware/middleware.go`

---

## 进度与验证

- 已完成（High）
  - 毫秒时间单位（HTTP 执行结果）
  - WebSocket 错误事件 `{ type: "error", message: "..." }`，并保留 `error` 字段以兼容旧客户端
  - JSON Content-Type 宽容（含 charset），并仅在需要的 POST/DELETE 路由生效
- 已完成（Medium）
  - 包管理：安装 201、卸载 204；错误体 `{ message }`；E2E 覆盖
  - WebSocket 初始化：顶层字段与 payload 双轨兼容；未知类型不强制断开
  - 错误映射统一：HTTP/JSON 中间件错误统一返回 `{ message }`（含 415 Unsupported Media Type）；handler 全覆盖
  - Runtimes 元数据：对齐 `aliases`/`provides`；新增 `platform/os/arch` 字段
- 验证情况
  - 测试：`tests/e2e` 全部通过；`cli/simple-test.sh` 通过
  - WebSocket：修复关闭后发送导致的 panic；E2E WebSocket 用例全部通过
  - Execute：`code` 可空与 `signal` 互斥语义已对齐；compile-only/run-only 分支行为符合预期

---

## 下一步优先对齐任务（建议）

1) Medium WS 事件命名兼容层（可选）
  - 如需对齐 `stage:start`/`stage:end` 旧式命名，增加兼容层或配置开关。
  - 涉及：`internal/handler/websocket.go`

2) Low 启动/超时/CORS 配置优化
  - 细化 server timeout（安装等长耗时）与 CORS 白名单可配置。
  - 涉及：`cmd/server/main.go`

---

## 配置与启动对齐 TODO

- [ ] Low 启动前置校验
  - 启动 HTTP 前完成数据目录/依赖校验并 fail-fast，日志清晰。
  - 涉及：`cmd/server/main.go`、`internal/config/config.go`

- [ ] Low 隔离器路径可配置
  - Isolate/沙箱路径支持 env/config 覆盖，适配多环境部署。
  - 涉及：`internal/config/config.go`、`internal/job/job.go`

---

## 兼容性与迁移（建议）

- [ ] Phase 1（新增兼容）：接受新旧请求字段；响应同时返回毫秒数值字段与旧字段（若存在）；WS 错误事件同时提供新旧键名（`message` 与旧 key）。
- [ ] Phase 2（默认切换）：客户端与测试切换到新契约；旧字段使用打印告警。
- [ ] Phase 3（移除旧行为）：删除旧字段/事件键名支持，清理弃用路径。

---

## 验收标准与测试 TODO

- [ ] E2E：新增/扩充分支用例
  - Content-Type 变体（含 `charset`）通过
  - 执行响应 `duration` 为数字毫秒并与 Piston 预期一致
  - 覆盖限额覆盖优先级（请求覆盖服务器默认）
  - `stdin` 不自动追加换行
  - WS 事件序列、错误事件格式、输出上限
  - 包安装 201/卸载 204，错误码与错误体一致
  - Runtimes 字段命名与别名/提供能力一致
  - 目录：`coderunr/tests/e2e`

- [ ] CLI/Smoke：回归
  - `cli/simple-test.sh` 全量通过；可追加一条 WS 交互契约校验脚本。

---

## 文件定位（便于落地）

- 服务器入口/路由：`cmd/server/main.go`
- 类型与 DTO：`internal/types/types.go`
- 执行与流式：`internal/handler/handler.go`、`internal/handler/websocket.go`、`internal/job/job.go`
- 运行时/包服务：`internal/runtime/runtime.go`、`internal/service/package.go`、`internal/handler/packages.go`
- 中间件与配置：`internal/middleware/middleware.go`、`internal/config/config.go`

---

## 完善与优化建议（超出对齐范围）

- 观测性
  - 结构化日志与请求 ID；Prometheus 指标（请求耗时、阶段时长、WS 会话、输出截断、错误率）；可选 OpenTelemetry trace。
- 限流与防护
  - 路由与 IP 级限流/令牌桶；请求体大小阈值；WAF 规则（可选）。
- 响应压缩与大输出处理
  - 按阈值启用 gzip；对大输出分块/背压；明确输出截断标志与统计。
- 配置与安全
  - 配置校验与敏感字段脱敏打印；特性开关（启用 Piston 兼容层的开关与灰度比例）；CORS 源列表可配置。
- 健康检查
  - 区分 `liveness` 与 `readiness`；`readiness` 检查依赖（数据目录、包索引可达、隔离器可用）。
- 包与运行时生命周期
  - 包缓存 TTL 与 GC；预热常用运行时；失败重试与指数退避。
- 沙箱与隔离
  - 可配置网络开关、fs 只读挂载、seccomp/AppArmor（取决于运行环境），以及 CPU/内存/进程数上限更细粒度配置。

---

如需，我可以先落地 High 优先的三项（毫秒时间单位、WS 事件/错误格式对齐、JSON Content-Type 宽容）并补充相应 E2E/回归测试。
