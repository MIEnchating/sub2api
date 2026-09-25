# 上游合并排除列表

以下产品决策适用于后续合并和验证修复，不能通过恢复上游实现来绕过。

| 功能 | 排除范围 | 必须保留 |
| --- | --- | --- |
| 共享账号池 | 第二上游的共享账号池页面、API、调度、权限、计费、迁移、测试和专属资源 | 普通账号池及无关账号代理池功能 |
| 批量生图 | 两个上游中的批量生图页面、菜单、快捷入口、分组权限和计费配置、API、队列、worker、供应商适配、专属测试、文档及 ORM | 普通生图、异步单图任务、已经发布的历史 SQL 迁移及其校验和兼容记录 |

机器检查规则位于 `.github/upstream-exclusions.json`。同步脚本在最终合并审查后、每轮完整验证（包括修复后的重跑）执行 `python3 .github/check-upstream-exclusions.py .`。合并审查分别记录 `excluded_shared_account_pool_paths`、`excluded_batch_image_paths` 和未解决冲突。

## 批量生图删除记录

- 前端：`frontend/src/views/user/BatchImageGuideView.vue`、`frontend/src/api/batchImage.ts`、`frontend/src/composables/useBatchImageAccess.ts` 及中英文专属文案已删除。
- 入口：删除 `/batch-image` 和 `/docs/batch-image` 路由、侧栏菜单、菜单设置项、用户及管理员首页快捷入口。
- 分组：删除批量生图开关、折扣和冻结倍率的界面、请求字段、服务逻辑、缓存映射及 ORM 字段。
- 后端：删除 `backend/internal/{handler,service,repository}/batch_image*.go`，移除 `/v1/images/batches` 系列路由、依赖注入和后台任务。
- 配置：删除 `BatchImageConfig`、相关默认值和校验、开发部署中的 `BATCH_IMAGE_*` 环境变量。
- 数据模型：删除 `backend/ent/schema/batch_image_*.go` 并重新生成 ORM；删除 `docs/BATCH_IMAGE_MVP.md`。

历史迁移 159–169、187 和 234 中与批量生图有关的文件保持原样，精确路径列在机器规则的 `preserved_paths` 中；其他已发布迁移（包括 `241_channel_monitor_v2_cache_eligibility.sql`）也必须保留。第二上游新增的共享账号池退役迁移 `255_retire_shared_account_pool.sql` 及其测试不属于已发布兼容记录，继续排除。数据库中的旧表、记录、分组字段和冻结余额不会被删除或修改；这些兼容性记录不代表功能仍可使用。新上游迁移不自动进入保留列表。

若已有部署运行过批量生图，应在升级前用旧版本完成或取消未终结任务并核对冻结余额。删除后的版本不再处理旧队列或结算任务；本次代码修改不连接生产数据库、不自动退款或删除上游资源。

本轮基于 `origin/main`（`3233df636`）、`upstream/main`（`1c0a69c0c`）和 `overdraft/sub2api-custom`（`5e1584ff6`）核对，采用已逐项处理文本与语义冲突的合并树 `c5e76798b`，并补回当前基线的服务版本 `2026.9.22`。第二上游的质量保护、定时测试增强、上游计费限流、Codex 代理选择和账号状态更新均保留；共享账号池与批量生图运行时实现继续排除。源码文件没有待解决的冲突标记。

2026-09-22 后续合并审查：`upstream/main` 仍为 `1c0a69c0c`，第二上游推进到 `c975fc661`。该增量仅包含渠道质量检查、模型身份校验、执行快照及配套迁移，不包含新的共享账号池或批量生图路径，因此本轮 `excluded_shared_account_pool_paths` 和 `excluded_batch_image_paths` 均为空；既有排除路径保持不变。`backend/cmd/server/VERSION`、`backend/internal/service/account_test_service.go` 和 `frontend/src/components/tests/__tests__/AdminTestResultHistory.spec.ts` 的文本冲突已逐项解决，`unresolved_conflicts` 为空。

2026-09-23 主上游合并冲突审查：当前阶段合并 `upstream/main`（`a3eb7ef30`）到本项目基线 `9f5e40e61`。保留 OpenCode Go 用量窗口、Claude Code 版本同步、简易模式密钥额度窗口、月度备份归档、线下提现幂等登记及网关兼容性修复，同时保留已有账号质量保护、Codex 打票与代理池、上游计费限流及网关运行策略。账号更新的行锁查询同时保留用量快照、打票数据和质量调度状态，响应关闭取消与最终账号健康观测同时生效。

本阶段明确排除 `backend/cmd/server/wire_gen.go` 冲突侧重新出现的 `batchImageCleanupService`、`batchImageWorkerRuntime` 清理依赖；没有新增共享账号池专属路径。历史批量生图 SQL 与 `migrations_runner.go` 校验和兼容记录逐字节保持基线内容，普通生图和异步单图任务继续保留。第二上游现有功能不因本次主上游冲突处理而删除；已审阅第二上游 `2519a1f5b` 的增量记录，其后续合并由同步流程的第二上游阶段处理，本记录不替代最终双上游审查。

语义复查同时修复账号编辑弹窗重复导入，以及仅携带 `x-codex-turn-metadata` 时跳过账号身份隔离的问题；保留本项目的身份命名空间、指纹字段删除策略和上游的 ASCII 安全编码。账号全选测试补齐组件卸载和防抖刷新等待，避免定时任务跨用例访问已重置的 mock。相关 103 个服务层测试、36 个仓储测试通过，单元与集成测试包均可编译，前后端构建通过，后端静态检查为零问题。完整后端测试已运行并重跑，涉及本地监听、Docker 和外网的检查受当前沙箱限制，须由外层验证环境完成；这不改变功能排除规则。

前端最终重跑：353 个测试文件、2752 项测试全部通过，进程正常退出；类型检查、lint 和生产构建通过。源码无残留冲突标记，功能排除检查通过。本阶段只修改工作树，Git 索引的冲突标记由外层同步流程统一暂存后清除。


## 2026-09-25 第二上游冲突审查

审查基线为 `origin/main=b7715623e`，主上游为 `upstream/main=a3eb7ef30`，第二上游为 `overdraft/sub2api-custom=3ffa4c943`；发布累计基准为 `v2026.9.24`（`0b8e1a894`）。主上游已完整包含在本项目中。本轮第二上游新增一个合并提交，没有新增非合并提交；不能据此忽略该合并提交携带的手工修复和文档。

累计审查覆盖已合并但未发布的 `f8dd91ed4` 版本同步和 `b7715623e` 自动发布恢复修复，包括等待远程工作流的新运行次数、收集失败日志后集中修复和完整重验、保护已占用标签、恢复未完成的稳定发布、确认正式资产后清除待发布状态。这些变化继续保留；本轮没有执行仓库 fetch、commit、push、打标签、发布或服务重启。

### 文本和语义处理

- 逐段解决 22 个文件、34 个冲突块；设置服务、管理 API、DTO 和前端保留本项目网关运行策略、打票代理脱敏展示、掩码回填及热更新缓存失效，兼容第二上游 Codex 292/332 打票。
- `backend/internal/handler/wire.go` 和 `backend/cmd/server/wire_gen.go` 对齐依赖顺序，保留账号健康管理，OpenCode 用量服务只注入一次；账号 DTO 的用量与门票配置字段各保留一次。
- `backend/internal/repository/account_repo.go` 保留行锁查询中的 OpenCode/Ollama 用量、Codex 门票与质量调度状态；保留本项目保护状态的额外查询及对应 SQL mock。`backend/internal/service/admin_account.go` 在校验前删除受管用量字段，同时保留账号保护、上游计费限流和打票策略校验。
- `backend/internal/service/openai_alpha_search.go` 在出站身份收口前仅执行一次账号身份隔离，避免重复哈希。元数据测试同时覆盖有 Codex UA 与仅有元数据的请求，保留 ASCII 安全编码及指纹删除规则。
- `backend/internal/service/scheduler_snapshot_service.go` 重建历史正分组缓存时按分组读取账号；新的简易模式请求仍通过 `bucketFor` 归一为全局桶。保留第二上游新增的普通/混合调度边界断言和生命周期测试。
- 保留请求日志集成测试的清理、Codex 高用量快照节流测试和 `docs/ANTIGRAVITY_ATTRIBUTION_429.md`。服务版本保持 `2026.9.24`，第二上游标识 `FORK_VERSION` 更新为 `v0.2.8-custom.1`。
- 验证发现 `tools/check_pnpm_audit_exceptions.py` 会把 registry 错误 JSON 当成无漏洞结果；现对网络错误、无效 JSON 和缺失/类型错误的审计结果明确失败，新增 `.github/test_upstream_audit.py` 回归。该修复不会放宽已有漏洞例外规则。

第二上游的组合质量规则、最新结果展示、定时测试与自动重试、缓存质量恢复、Codex 门票及账号身份隔离、普通账号代理池、API key 回退、上游计费限流和请求用量归因均保留。普通生图与异步单图任务继续保留。全部 SQL 迁移相对发布基准没有变化；批量生图保留路径及迁移校验和兼容记录逐字节一致，不操作历史表、记录或冻结余额。

### 本轮明确核对并继续排除的路径

以下专属文件在候选中不存在；混合用途文件仅排除对应批量生图代码，文件内其他功能仍保留。排除检查已通过。

共享账号池：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图：
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/ent/batchimageevent.go`
- `backend/ent/batchimageevent/batchimageevent.go`
- `backend/ent/batchimageevent/where.go`
- `backend/ent/batchimageevent_create.go`
- `backend/ent/batchimageevent_delete.go`
- `backend/ent/batchimageevent_query.go`
- `backend/ent/batchimageevent_update.go`
- `backend/ent/batchimageitem.go`
- `backend/ent/batchimageitem/batchimageitem.go`
- `backend/ent/batchimageitem/where.go`
- `backend/ent/batchimageitem_create.go`
- `backend/ent/batchimageitem_delete.go`
- `backend/ent/batchimageitem_query.go`
- `backend/ent/batchimageitem_update.go`
- `backend/ent/batchimagejob.go`
- `backend/ent/batchimagejob/batchimagejob.go`
- `backend/ent/batchimagejob/where.go`
- `backend/ent/batchimagejob_create.go`
- `backend/ent/batchimagejob_delete.go`
- `backend/ent/batchimagejob_query.go`
- `backend/ent/batchimagejob_update.go`
- `backend/ent/client.go`
- `backend/ent/ent.go`
- `backend/ent/group.go`
- `backend/ent/group/group.go`
- `backend/ent/group/where.go`
- `backend/ent/group_create.go`
- `backend/ent/group_update.go`
- `backend/ent/hook/hook.go`
- `backend/ent/intercept/intercept.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/predicate/predicate.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/ent/schema/group.go`
- `backend/ent/tx.go`
- `backend/internal/config/config.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/repository/group_repo.go`
- `backend/internal/repository/usage_billing_repo.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_group_duplicate.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/api_key_auth_cache.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_mvp_smoke_test.go`
- `backend/internal/service/batch_image_processor.go`
- `backend/internal/service/batch_image_processor_test.go`
- `backend/internal/service/batch_image_provider.go`
- `backend/internal/service/batch_image_provider_gemini.go`
- `backend/internal/service/batch_image_provider_gemini_test.go`
- `backend/internal/service/batch_image_provider_vertex.go`
- `backend/internal/service/batch_image_provider_vertex_test.go`
- `backend/internal/service/batch_image_public.go`
- `backend/internal/service/batch_image_public_test.go`
- `backend/internal/service/batch_image_queue.go`
- `backend/internal/service/batch_image_settlement.go`
- `backend/internal/service/batch_image_settlement_test.go`
- `backend/internal/service/batch_image_test.go`
- `backend/internal/service/batch_image_worker.go`
- `backend/internal/service/batch_image_worker_runtime.go`
- `backend/internal/service/batch_image_worker_runtime_redis_test.go`
- `backend/internal/service/batch_image_worker_runtime_test.go`
- `backend/internal/service/batch_image_worker_test.go`
- `backend/internal/service/group.go`
- `backend/internal/service/usage_billing.go`
- `backend/internal/service/wire.go`
- `deploy/EDGE_SECURITY.md`
- `deploy/docker-compose.dev.yml`
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/batchImage.ts`
- `frontend/src/api/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/user/dashboard/UserDashboardQuickActions.vue`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/en/common.ts`
- `frontend/src/i18n/locales/en/dashboard.ts`
- `frontend/src/i18n/locales/en/index.ts`
- `frontend/src/i18n/locales/en/landing.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/i18n/locales/zh/common.ts`
- `frontend/src/i18n/locales/zh/dashboard.ts`
- `frontend/src/i18n/locales/zh/index.ts`
- `frontend/src/i18n/locales/zh/landing.ts`
- `frontend/src/router/index.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/user/BatchImageGuideView.vue`

源码冲突标记已清除；本阶段仅修改工作树，Git 索引的未合并状态由外层同步流程统一暂存清除。

验证结果：前端 354 个文件、2806 项测试全部通过，类型检查、ESLint、i18n 与生产构建通过；合并服务回归 215 项、仓储/DTO/退役路由回归 76 项通过；同步、发布恢复与审计回归 40 项通过；部署脚本检查、`go vet ./...`、golangci-lint（0 issues）和后端生产构建通过。完整后端普通、unit、integration 测试均已执行并重跑，分别有 2885、4037、2648 项通过，失败包均涉及当前沙箱的网络/监听权限，仓储 Docker 测试按既有 harness 跳过。在线 Go/前端漏洞数据库查询受网络限制，未将错误响应视为通过；外层仍须执行完整安全门禁。本轮未发现未消除的产品、权限、计费或数据库风险。


## 2026-09-25 最终双上游兼容复核

在候选 `1be8e72ec` 上继续复核上一阶段结果，`origin/main=b7715623e`、`upstream/main=a3eb7ef30`、`overdraft/sub2api-custom=3ffa4c943` 均未移动。两上游都已成为候选祖先，Git 索引无未解决条目，源码无冲突标记。复核覆盖 `v2026.9.24`（`0b8e1a894`）至候选的全部累计 23 个文件，不仅是本轮合并：包括 `f8dd91ed4` 版本同步、`b7715623e` 发布恢复，以及第二上游合并提交自身的修复和文档。该窗口内两上游去重后的新增非合并提交数仍为 0。

逐项核对第二上游合并提交的手工新增内容，保留 OpenCode Go 用量与 Codex 门票 DTO/依赖注入、账号行锁状态、受管字段校验顺序、Alpha Search 单次身份隔离、调度历史分组边界、快照节流、请求日志夹具清理及 Antigravity 429 文档。服务版本使用本项目 `2026.9.24`，第二上游版本信息通过 `FORK_VERSION=v0.2.8-custom.1` 保留；账号健康管理、网关运行策略、打票代理脱敏和热更新继续与第二上游功能共同生效。

最终复核发现并修复 `tools/check_pnpm_audit_exceptions.py` 的条目级漏检：`critical` 结果的 `via` 为空、缺失严重级别或缺失包名原先可能静默通过。现明确拒绝不完整条目，支持数字公告编号而不抛出类型异常，并保留间接依赖及原有漏洞例外匹配规则。`.github/test_upstream_audit.py` 新增相应回归。该修复未修改依赖版本或放宽漏洞例外。

已从两个上游的实际树重新核对上述 119 个批量生图排除路径，其中 64 个专属文件缺席，55 个混合用途文件仅移除专属实现。共享账号池的两条迁移/测试路径继续缺席；另明确核对并继续排除以下混合用途文件中的共享账号池专属延迟传递代码：

- `backend/internal/service/usage_billing.go`：不恢复 `UsageBillingCommand.DurationMS`。
- `backend/internal/service/gateway_usage_billing.go`：不恢复 `buildUsageBillingCommand` 中向上述字段赋值的代码。

这两处源自第二上游 `448b458ac`（共享请求延迟持久化），其唯一消费者为已退役的 `settleSharedAccountUsage`。普通请求的 `UsageLog.DurationMs`、日志持久化与 DTO 映射不受影响，普通账号池、账号代理池、普通生图和异步单图任务继续保留。全部 SQL 迁移和批量生图历史校验和兼容记录与发布基准逐字节一致；未触及历史表、记录或冻结余额。

本阶段仅修改临时工作树；未对目标仓库执行 fetch、commit、push、标签操作、发布或服务重启。

最终复跑结果：前端锁定依赖离线安装、ESLint、类型检查、354 个文件的 2806 项测试、3 项 i18n 检查及生产构建全部通过；服务合并专项 215 项、仓储/DTO/退役路由专项 76 项、同步/发布恢复/审计 44 项、发布辅助 10 项通过。部署脚本及语法检查、`go vet ./...`、golangci-lint（0 issues）和后端生产构建通过。后端完整普通/unit/integration 检查分别有 2885/4040/2643 项通过，失败均可归因于本地监听、Docker 或外网访问限制；在线 Go 漏洞扫描及 pnpm 审计因网络权限/DNS 未完成，错误响应被检查器明确拒绝，未将其记为安全通过。外层流程仍需完成这些环境相关验证；这些限制不构成产品合并风险。最终 decision 已通过目标仓库 schema 的全部字段与类型校验，未发现剩余产品、代码、安全、权限、计费或数据库风险。
