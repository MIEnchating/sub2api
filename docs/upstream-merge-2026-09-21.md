# 第二上游合并记录（2026-09-21）

## 基准与范围

- 本地原始基准：`6ba4b829ce0060967cdef01bac762da759b20c8e`。
- 主上游：`7c700729c23187d31ed320f6b19c790e2f194826`，保留其全部变化。
- 第二上游：`f0b2c0dce2707771edbaeb386bd572cdd8f41995`。
- 保留临时快照中的账号生成预览、HTML 同步邮件报告；第二上游已退役的 Prism 实现完整移除。
- 本次仅修改临时工作树和合并索引；不 fetch、commit、push、打 tag、发布或重启服务。发布版本仍为本地 `2026.9.20`，`FORK_VERSION` 更新为第二上游 `v0.2.7-custom.1`。

## 功能与语义处理

1. 合入 Codex ticket harvest/inject、账号级开关和缺票策略、Pro 292 / Team/Business 332 长度、历史与诊断、独立账号/模型调度与重试、认证刷新、取消与退出清理。票据、访问凭据和代理密码不进入公开 DTO、导出和诊断。
2. 排除共享 ticket 代理池；只保留单个 harvest proxy 及其出口测试。移除多行列表解析、轮换、池计数、代理选择 UI 和专属每代理并发配置。保留总并发/账号并发限制、单出口故障冷却和普通账号多代理路由。旧 `codex_ticket_harvest_proxy_ids` 仅作为忽略/脱敏的历史字段，不再参与运行时路由。
3. 合入审计用户白名单，保留 TypeSafe/通用 GPT 独立配置与风险路由；白名单经配置复制、引擎切换、异步队列和 cyber-policy 记录链路生效。
4. 合入 HTTP/SSE/WS、多平台和媒体请求中的上游计费错误隐藏及重试；保留主上游仍直接依赖的 429 次数/冷却、容量恢复与 WS 重试安全判断。Seedance 已接受的异步任务不因计费错误重复创建。
5. 合入显式 Codex 指纹收敛、设备别名/种子兼容和测试请求身份一致性；保留插件 HostService 账号目录、旧保护传输隔离、账号正常并发和生成预览的单次请求语义。
6. 按第二上游退役 Prism 专属代码、界面和凭据。账号保护、自适应并发和账号健康的公共实现仍被主上游风险路由、容量恢复及测试链路直接调用，作为主上游全量合并的兼容依赖保留；未恢复任何共享账号池实现。
7. 普通账号池、多代理选择、最近请求状态/详情、表格列宽、定时测试增强完整保留。主上游发布矩阵、HostService、Seedance、Codex 积分/邀请完整保留。
8. 解决自动合并引入的重复 provider、重复前端 ticket 计算、重复测试、已删除配置字段引用和账号导出变量冲突。更新 Codex 积分/邀请快照测试以兼容本地保护 SQL，并继续验证无多余调度事件。
9. `wire_gen.go` 由项目 `go generate ./cmd/server` 生成；生成器额外依赖使用临时 modfile，不修改仓库 Go 依赖。Ent schema/生成树无需改动，既有树不包含共享账号池实体。

## 明确排除的专属路径

下列第二上游路径在最终文件树中均不存在，包括此前本地已排除、此次维持排除的路径：

```text
backend/ent/schema/shared_account_listing.go
backend/ent/schema/shared_account_usage_ledger.go
backend/ent/schema/shared_account_wallet.go
backend/ent/sharedaccountlisting.go
backend/ent/sharedaccountlisting/sharedaccountlisting.go
backend/ent/sharedaccountlisting/where.go
backend/ent/sharedaccountlisting_create.go
backend/ent/sharedaccountlisting_delete.go
backend/ent/sharedaccountlisting_query.go
backend/ent/sharedaccountlisting_update.go
backend/ent/sharedaccountusageledger.go
backend/ent/sharedaccountusageledger/sharedaccountusageledger.go
backend/ent/sharedaccountusageledger/where.go
backend/ent/sharedaccountusageledger_create.go
backend/ent/sharedaccountusageledger_delete.go
backend/ent/sharedaccountusageledger_query.go
backend/ent/sharedaccountusageledger_update.go
backend/ent/sharedaccountwallet.go
backend/ent/sharedaccountwallet/sharedaccountwallet.go
backend/ent/sharedaccountwallet/where.go
backend/ent/sharedaccountwallet_create.go
backend/ent/sharedaccountwallet_delete.go
backend/ent/sharedaccountwallet_query.go
backend/ent/sharedaccountwallet_update.go
backend/internal/handler/shared_account_oauth_handler.go
backend/internal/handler/shared_account_oauth_handler_test.go
backend/internal/handler/shared_account_pool_handler.go
backend/internal/handler/shared_api_key_handler.go
backend/internal/repository/shared_account_pool_repo.go
backend/internal/repository/shared_account_pool_repo_test.go
backend/internal/repository/shared_account_revenue_repo_test.go
backend/internal/repository/shared_account_settlement_contract_test.go
backend/internal/repository/shared_account_wallet_integration_test.go
backend/internal/repository/shared_account_wallet_repo.go
backend/internal/repository/shared_account_wallet_repo_test.go
backend/internal/repository/shared_api_key_repo.go
backend/internal/repository/shared_pool_isolation_test.go
backend/internal/server/middleware/shared_pool_test.go
backend/internal/service/scheduler_snapshot_shared_pool_test.go
backend/internal/service/shared_account_pool.go
backend/internal/service/shared_account_pool_test.go
backend/internal/service/shared_account_revenue.go
backend/internal/service/shared_account_wallet.go
backend/internal/service/shared_api_key.go
backend/internal/service/shared_api_key_schedule_test.go
backend/internal/service/shared_api_key_test.go
backend/internal/service/shared_pool_abuse_test.go
backend/migrations/238_shared_account_pool.sql
backend/migrations/239_enable_shared_account_pool.sql
backend/migrations/239_shared_pool_api_keys.sql
backend/migrations/240_shared_api_key_billing_bridge.sql
backend/migrations/242_shared_api_key_schedule.sql
backend/migrations/244_shared_pool_admin_controls.sql
backend/migrations/245_shared_pool_integrity_constraints.sql
backend/migrations/246_shared_pool_revenue_indexes.sql
frontend/src/api/__tests__/admin.sharedPool.spec.ts
frontend/src/api/admin/sharedPool.ts
frontend/src/api/sharedAccountCreation.ts
frontend/src/api/sharedPool.ts
frontend/src/views/admin/SharedPoolAdminView.vue
frontend/src/views/user/SharedPoolView.vue
frontend/src/views/user/__tests__/SharedPoolView.spec.ts
openspec/changes/add-shared-account-pool/design.md
openspec/changes/add-shared-account-pool/proposal.md
openspec/changes/add-shared-account-pool/tasks.md
openspec/changes/add-shared-account-pool/verification.md
backend/internal/service/openai_codex_ticket_proxy_pool.go
backend/internal/service/openai_codex_ticket_proxy_pool_test.go
```

共享功能在通用文件中的接线也已排除：`backend/internal/{handler,service,server/middleware}/wire.go`、生成的 `backend/cmd/server/wire_gen.go`、账号 DTO、账号创建/编辑表单和测试。共享 ticket 代理池接线从 config、settings service/handler/DTO、前端 settings API/页面/语言包、部署配置及调度器移除。普通 `account_proxy_pool`、令牌刷新并发池及导入去重中的 shared-key 命名不属于共享账号池，予以保留。

## 第二上游提交核对

```text
f0b2c0dce feat: add native Codex ticket diagnostics and release v0.2.7-custom.1
e7109f3aa feat: schedule Codex ticket retries independently with diagnostics
8eebe67d1 feat: share Codex ticket proxy pool and support Team 332
3b2b43601 feat: add audit whitelist and account-scoped Codex tickets
dbdd7d189 fix: hide upstream billing errors from clients
77b35acf4 fix: default new accounts to single-machine Codex fingerprint
f2631ccf9 refactor: retire Prism and consolidate Codex fingerprint settings
3c2f05c95 fix(openai): keep ticket writes off the scheduler and account edit paths
c44e6893c fix(openai): gate Codex tickets on the outbound compact model
f79381bbd fix(openai): harden Codex ticket lifecycle and account handling
1ba0b4fed feat(openai): add admin toggle for Codex ticket harvest and inject
d14054afc feat(openai): add live harvest proxy settings and protect ticket state
bc47e212b feat(openai): harvest and inject 292 x-codex-turn-state tickets for one hour
dc55b2e29 fix: improve Prism request compatibility and release v0.2.5-custom.8
d8c34a5e3 feat: automate Prism account authentication and release v0.2.5-custom.7
36d7b153f feat: add isolated Prism channel and release v0.2.5-custom.6
bb7cdbd2a feat: add account protection adaptive concurrency
2dbdb846b fix: show complete channel test results

```

## 冲突与验证

最初的 63 个索引冲突已逐项消解；先清空未合并索引，再处理自动合并语义问题。没有遗留文本冲突。

最终候选已通过后端 unit/integration、`go vet`、golangci-lint v2.13.0、`govulncheck` 和生产构建；前端 frozen install、ESLint、typecheck、完整 Vitest（314 个文件、2399 项）与生产构建；pnpm 生产依赖审计例外校验、Linux 部署脚本、release matrix 单测及 release shell 语法检查。macOS 专用 Apple Container 测试已通过 shell 语法检查，但当前 Linux 主机缺少 `plutil`，运行时测试留给远程 macOS CI。
