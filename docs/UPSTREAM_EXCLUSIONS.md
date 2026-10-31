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

## 2026-10-07 最终双上游兼容复核

审查候选为 `4b6a61c45`，本项目基线为 `origin/main=be74b223b`，主上游为 `upstream/main=3f1a2ea0a`，第二上游为 `overdraft/sub2api-custom=ebe7dd8d9`。两上游均为候选祖先，索引无未解决条目，源码无冲突标记。本轮仅修改工作树，不执行 fetch、提交、推送、标签操作、发布或服务重启。

累计窗口从指定的已发布提交 `4b76ca9d07647443c85f3db27b593144261c78ce` 开始，候选净变化为 28 个文件、1073 行新增、168 行删除；两上游去重后新增 5 个非合并提交。审查也覆盖此前已合并但尚未发布的 `be74b223b`：审计例外、前端历史审计记录及 `FORK_VERSION` 的变化，以及 `6c8875099` 的项目版本同步。

保留远程 Codex 模型目录的 API key 发现配置、EasyPay 订单签名重放防护、随机管理员登录邮箱、登录接口一致的邮箱校验、bcrypt 的 72 字节口令上限、Vue 3.5.43 与 source-map-js 1.2.2 的锁定依赖更新及部署文档。第二上游合并提交自身新增的三处 `UseKeyModal.spec.ts` 断言修正逐项核对并保留，`FORK_VERSION` 为 `v0.2.14-custom.1`。项目 `backend/cmd/server/VERSION` 保持 `2026.10.3`，分别保留本项目和第二上游版本身份。

本轮从第二上游实际树明确复核并继续排除前述批量生图专属文件，以及混合用途文件中的路由、依赖注入、分组权限、计费、配置、ORM、菜单和文案；其中 `deploy/docker-compose.dev.yml` 合入管理员邮箱更新后仍无 `BATCH_IMAGE_*` 配置。共享账号池的 `255_retire_shared_account_pool.sql`、`retire_shared_account_pool_integration_test.go` 继续缺席；`usage_billing.go` 的 `UsageBillingCommand.DurationMS` 和 `gateway_usage_billing.go` 对该字段的赋值继续排除。普通日志的 `UsageLog.DurationMs` 不受影响。

全部 SQL 迁移及 `backend/internal/repository/migrations_runner.go` 与指定发布提交逐字节一致，包含批量生图历史迁移 159–169、187、234 及校验和兼容记录。未修改历史表、记录或冻结余额。普通账号池、账号代理池、普通生图、异步单图、组合质量规则、定时测试与自动重试、Codex 门票和身份隔离、API key 回退、上游计费限流及本项目网关策略继续保留。

支付兼容性仍有一个明确限制：EasyPay 回调新增标准字段白名单，带有额外字段的兼容供应商回调会被拒绝；既有主动订单查询与对账恢复路径保留。该限制来自主上游的防伪策略，不能通过放宽回调验证消除，否则会重新开放签名重放攻击。

本轮验证：功能排除检查、47 项同步与审计回归、10 项发布辅助回归、管理员初始化包测试、EasyPay 防伪回调专项、支付返回地址和恢复令牌专项、普通生图/异步单图/退役批量生图路由专项通过。后端生产构建、`go vet ./...`、全部 unit/integration 测试包编译、部署 Shell 语法检查、四项 Compose/资源检查和安装令牌检查通过。前端锁定离线安装因缺少 Vue 3.5.43 缓存未完成，未宣称前端测试或构建通过；完整支付供应商测试被沙箱监听权限阻断，Apple 容器测试缺少 `plutil`，这些属于验证环境限制。最终 JSON 已通过目标仓库决策 schema 校验。

全仓库 golangci-lint 长时间未返回结果，已停止该检查，未宣称其通过；完整前后端和在线安全门禁仍由外层验证流程执行。

## 2026-10-08 冲突修复复核

本轮审查发布基准 `v2026.10.3`（`4b76ca9d07647443c85f3db27b593144261c78ce`）至候选工作树的累计差异，主上游为 `upstream/main=5fc0e486c`，第二上游为 `overdraft/sub2api-custom=ebe7dd8d9`。主上游和第二上游的账号、网关、协议兼容、运维、支付、前端竞态修复及测试增强继续保留。

逐项解决 `account_test_service.go`、`openai_gateway_forward.go`、`PricingEntryCard.vue` 的文本与语义冲突：账号测试同时保留日志归因、后台测试标记、保护统计排除和指纹租约释放；OpenAI 加密内容重试同时保留 429 重试标记并兼容特定 `thinking_signature_invalid` 响应；价格默认值仅对空白条目首次加入单个模型时填充。批量生图访问 composable 及其专属重试测试被删除：

- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`

本轮没有共享账号池专属路径新增或恢复；`255_retire_shared_account_pool.sql` 及其测试继续缺席。批量生图历史迁移 159–169、187、234 和 `migrations_runner.go` 校验兼容记录保持原样，普通生图、异步单图任务、普通账号池和账号代理池继续保留。排除检查 `python3 .github/check-upstream-exclusions.py .` 通过；本阶段未执行 fetch、提交、推送、标签、发布或服务重启。

## 2026-10-08 最终兼容修复

复核主上游合并后的模型目录语义冲突时，发现通用 `ConfiguredTestModelIDs` 快捷路径会把未配置映射的 Antigravity 账号误转成无显示元数据的默认映射列表。已在 `backend/internal/handler/admin/account_handler.go` 将 Antigravity 专用目录处理提前，仅读取显式保存的映射；无映射账号继续返回带显示名称和创建时间的 `antigravity.DefaultModels()`，显式映射仍保留自定义模型。相关管理处理器测试通过，源码无未解决冲突标记，功能排除检查继续通过。

## 2026-10-12 主上游冲突与累计兼容复核

本轮只修改临时工作树。基线为 `origin/main=819a47789`，主上游为 `upstream/main=ab80abc6d`，第二上游为 `overdraft/sub2api-custom=0976d0215`。审查覆盖 `v2026.10.3`（`4b76ca9d07647443c85f3db27b593144261c78ce`）到候选工作树的全部累计差异，两个上游去重后共 109 个非合并提交；也逐项复核第二上游最新合并提交自身的修复。未执行 fetch、提交、推送、标签、发布或服务重启。索引由外层流程统一暂存，本阶段只清除工作文件的冲突。

### 保留的上游变化

- 平台清单与 provider profile、多协议端点和模型目录路由、Cline 与 Command Code 接入及钱包/用量/冷却、OpenCode Zen 路由和不支持模型限制。应用层平台校验覆盖管理 API、服务、仓储和 Ent。
- 分组名称与描述多语言、语言代码大小写归一、保留 Origin 的 Vary 响应头、用户显示本地化与管理员原始编辑数据分离。
- GPT-6.1 Sol 默认上下文 500k、订阅凭据豁免模型长上下文附加费（保留 API Key 价卡、自定义区间价格）、OAuth 搜索历史工具声明和 Responses Lite additional_tools 支持。
- 累计窗口内的 EasyPay 签名重放防护、安全管理员初始化、口令长度限制、远程 Codex 目录与账号映射、协议转换和流式心跳、逐轮 WS 计费与图片输入用量、渠道健康样本要求、TPS 及账号名称展示、支付倒计时和各类前端过期请求保护。
- Go 1.27.2、x/net 等安全依赖、Vue/source-map-js 锁定依赖与 CI 配套升级；本项目同步邮件的阶段状态、失败归因与文本/HTML 报告改进。

### 冲突与语义修复

- 解决 10 个文件的文本冲突。服务 VERSION 保留 `2026.10.3`，第二上游版本身份更新到 `FORK_VERSION=v0.2.15-custom.1`。
- Ent 分组增加 i18n 后重新计算 runtime 描述符、mutation 容量和迁移索引；批量生图字段继续缺席。重新生成全部 Ent 与手工修复结果一致。
- 模型路由测试采用新的通用函数名称，Responses 和 adaptive Anthropic 路径继续传递自定义 prompt；未登记的协议端点跳过。补强测试验证提示词确实发送到三种协议端点。
- 渠道模型同步同时保留平台能力判断、去重、默认价格、DeepSeek 独立价格拆分、并发请求及用户修改保护。配置配额测试覆盖完整平台清单；Codex 配置测试保留 API key 模型发现和 WebSocket/goals 断言。
- 合入第二上游最新合并的 Antigravity 通配符过滤、账号质量暂停筛选回归、updater Go 版本和相关测试适配。普通账号池、账号代理池、组合质量规则、定时测试/自动重试、Codex 门票与身份隔离、API key 分组回退、网关运行策略、429 重试与计费不变量保持。
- 修复自动合并未提示的 Cline/Command Code 钱包错误分支顺序：恢复可充值钱包冷却优先于通用永久停用；Cline 冷却持久化失败也不误置永久错误，普通账号账单错误仍停用。保留第二上游对应回归。
- 累计复核与全量测试发现上次合并仍允许替换已有模型时填充默认价格；现遵循两上游规则，仅空条目首次增加单个模型时填充，保留本项目倍率字段与手动编辑/过期响应保护。

### 本轮明确排除的位置

下列共享账号池迁移/测试继续不存在；两个混合用途计费文件仅排除共享池专属 DurationMS 传递，普通 UsageLog.DurationMs 保留：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`
- `backend/internal/service/usage_billing.go`
- `backend/internal/service/gateway_usage_billing.go`

以下批量生图位置逐项核对，混合用途文件仅排除批量生图字段、权限、计费、路由或文案，其他实现保留；两个访问 composable/重试测试路径继续不存在：
- `backend/ent/group.go`
- `backend/ent/group/group.go`
- `backend/ent/group_create.go`
- `backend/ent/group_update.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/group.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/group_repo.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_group_duplicate.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/group.go`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`

全部已发布 SQL 迁移与发布基准逐字节一致，包含规则指定的 14 个历史迁移/校验兼容文件。本轮新增 `242_drop_platform_check_constraints.sql` 和 `242_group_i18n.sql`；迁移器以完整文件名为主键，两个 242 文件均可登记并执行。未操作历史表、记录、冻结余额或生产数据库。普通生图、异步单图任务及其权限和计费继续保留。

### 剩余兼容限制

累计窗口中 EasyPay 回调使用标准字段白名单，发送额外自定义字段的兼容支付商回调会被拒绝；既有主动查询与对账恢复仍保留。保留此防伪边界，不能以兼容为由重新开放签名重放。

验证：功能排除检查、历史 SQL/校验兼容文件比对、Ent 重新生成一致性通过；后端专项 1070 项（包含子测试）通过，涵盖提示词透传、统一协议分流、钱包冷却、平台权限、长上下文计费、普通/异步单图及退役路由。后端构建和 `go vet ./...` 通过。后端使用 `/tmp` 隔离 modfile 和已缓存 Go 1.27.1/依赖作兼容验证，未更改仓库的 Go 1.27.2 或安全依赖版本，不将其等同于完整版本门禁。Docker 仓储集成由既有 harness 跳过，在线审计超时产生的空报告被明确拒绝；完整 Go 1.27.2、Docker 集成及在线安全扫描仍由外层流程执行。同步/审计 47 项、发布辅助 10 项、部署配置和脚本检查通过。

前端最终验证：392 个测试文件、3169 项测试全部通过；最终类型检查、ESLint、i18n 及生产构建通过。一次并行全量复跑曾发生 worker RPC/用例超时，降低到两个 worker 后完整重跑通过，未放宽测试超时或修改产品代码绕过失败。源码无冲突标记，`git diff --check` 通过；最终决策按目标仓库 schema 校验通过。


## 2026-10-12 第二上游冲突与累计复核

本阶段在主上游合并候选 `78030d735` 上处理第二上游 `0976d0215`，本项目基线为 `origin/main=819a47789`，主上游为 `ab80abc6d`。继续审查 `v2026.10.3`（`4b76ca9d07647443c85f3db27b593144261c78ce`）以来全部累计变化，包括两个上游去重后的 109 个非合并提交、第二上游合并提交自身的手工修复，以及之前尚未发布的同步邮件、支付安全、管理员初始化、依赖升级等变化。仅修改临时工作树；不执行仓库 fetch、commit、push、打标签、发布或服务重启，Git 索引由外层同步流程统一暂存。

逐段解决 5 个文件的 7 处文本冲突：VERSION 保持本项目 `2026.10.3`，第二上游版本身份由 `FORK_VERSION=v0.2.15-custom.1` 保留；Antigravity 显式映射在通用配置模型之前处理，过滤通配符并保留默认模型元数据；后台测试同时保留 reasoning effort、自定义提示词、日志归因、指纹租约释放及账号健康统计隔离；价格默认值仅在空条目首次加入单个模型时填充；Codex 配置测试同时断言 API key 模型发现、goals 与对应 WebSocket 配置，移除重复断言。

语义复核另发现并修复价格默认值的过期响应：父页面在查询期间重置空条目时，旧响应原先会恢复已清空的模型和价格。`PricingEntryCard.vue` 现仅对原始条目允许等待父组件接收更新的兼容处理，父组件替换后的空条目不再被回填。新增回归先复现失败，再验证修复；后台模型身份测试补充实际出站请求的健康统计排除断言。

第二上游的组合质量规则、定时测试与自动重试、模型身份校验、Codex 门票与账号身份隔离、普通账号代理池、API key 分组回退、上游计费限流，以及模型同步中的 DeepSeek 独立价格和并发修改保护继续保留。Cline/Command Code 可充值钱包冷却优先于永久账单错误，冷却持久化失败不误停用账号。主上游的平台清单及多协议路由、分组多语言、GPT-6.1 Sol 上下文与订阅计费、搜索历史和 Lite 工具、逐轮 WS 计费、图片用量、协议转换与界面竞态修复均保留。

全部 321 个历史 SQL／迁移兼容文件与发布基准逐字节一致；新增的两个 242 迁移以完整文件名分别登记。未修改历史表、记录或冻结余额。普通账号池、普通生图和异步单图继续保留。

累计窗口仍有一项支付兼容限制：EasyPay 回调白名单拒绝额外自定义字段；主动查询与对账恢复仍保留，不能通过放宽校验重新引入签名重放。

以下为本阶段从两个上游实际树逐项明确排除的位置。共享池两个计费文件仅排除专属 `DurationMS` 传递，普通 `UsageLog.DurationMs` 保留；批量生图 65 个专属文件继续缺席，70 个混合用途文件只排除专属字段、调用、测试、文案和配置，不删除其余实现。历史 SQL 及校验和兼容记录不列入排除。

共享账号池：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`
- `backend/internal/service/usage_billing.go`
- `backend/internal/service/gateway_usage_billing.go`

批量生图：
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/cmd/server/wire_gen_test.go`
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
- `backend/internal/config/config_test.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/admin/group_handler_simple_mode_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/gateway_model_allowlist_listing_test.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/security_audit_media_submit_test.go`
- `backend/internal/handler/security_audit_order_test.go`
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
- `backend/internal/repository/usage_billing_repo_unit_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/middleware/group_model_allowlist_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/prompt_audit_route_coverage_test.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_group_duplicate.go`
- `backend/internal/service/admin_group_duplicate_test.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/admin_service_group_test.go`
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
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
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
- `frontend/src/views/admin/__tests__/ChannelsView.pricingSync.spec.ts`
- `frontend/src/views/admin/__tests__/GroupsView.codexManifest.spec.ts`
- `frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`

本阶段最终验证：前端 392 个文件、3170 项测试全部通过，类型检查、ESLint、i18n 和生产构建通过；后端服务／图片路由专项 103 项（含子测试）通过，管理模型目录和 EasyPay 防伪专项通过；后端构建、相关服务／管理处理器 `go vet` 通过；同步／审计回归 47 项通过。后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile 作兼容验证，未变更仓库 Go 1.27.2 或安全依赖版本。较宽的账号测试选择因一项 httptest 本地监听受限而中断，随后明确选取无需监听的协议、提示词、日志、健康隔离和钱包回归验证通过；不将兼容验证等同于完整工具链、Docker 集成或在线安全门禁。历史 SQL／兼容记录、功能排除、源码冲突标记和 `git diff --check` 最终检查通过，决策 JSON 按目标仓库 schema 验证通过。

## 2026-10-12 最终双上游兼容审查

候选为 `2691bcb334`，两个上游及本项目基线与上一节相同。重新阅读两路提交历史、双方差异、两次合并的冲突解决差异，以及 `v2026.10.3` 到候选的累计 389 个文件变化。累计包含 109 个去重后的上游非合并提交，也覆盖此前已合并未发布的支付安全、初始化安全、WS 计费、前端竞态、安全依赖和同步邮件改进。没有执行仓库 fetch、提交、推送、标签、发布或服务重启。

最终复核修复两处遗漏：

- 主上游分组多语言与第二上游 API Key 回退分组融合不完整：`backend/internal/handler/dto/group_localize.go` 现在同时本地化主分组和回退分组，用户 API Key 及嵌套用量记录不再携带回退分组完整译文表。回归先复现错误，再检查本地化结果、分组 ID、管理员原始数据及其他语言响应；不修改回退调度、权限或计费。
- 共享账号池排除规则此前不能识别第二上游残留 DTO 名称 `SharedTotalCallCount`、`SharedListingStatus`。候选原本已删除这两个字段；现补强 `.github/upstream-exclusions.json`，并用 Go 字段名与前端 snake_case 字段名四种用例验证。普通账号池、账号代理池、Gemini 共享配额和 WS 连接池仍允许。

本轮逐项确认上一节列出的 135 个批量生图路径继续排除：65 个专属文件不存在，70 个混合用途文件仅排除专属代码。共享账号池除上一节四处外，补充明确排除 `backend/internal/handler/dto/types.go` 的上述两个专属字段；普通账号 DTO 字段及 `UsageLog.DurationMs` 保留。没有新增数据清理迁移；321 个历史 SQL／校验兼容文件（包含机器规则指定的 14 个路径）与发布基准逐字节一致，两个新增 242 迁移仍按完整文件名登记。

此前已解决的版本身份、Ent 字段索引、Antigravity 模型目录、提示词透传、账号健康隔离、钱包冷却、渠道价格同步与过期响应保护均复核保留。第二上游的组合质量规则、定时测试和自动重试、模型身份校验、Codex 门票与身份隔离、普通账号代理池、API key 分组回退和上游计费限流继续保留；普通生图和异步单图继续可用。累计窗口剩余的具体兼容限制仍为 EasyPay 拒绝附加自定义回调字段；保留主动查询／对账恢复与防伪校验。

本轮验证：前端 392 个测试文件、3170 项测试全部通过，类型检查、ESLint、i18n 和生产构建通过。后端专项 278 项（含子测试）、DTO 全包、DTO `go vet`、服务构建及仓储／迁移集成测试编译检查通过；同步／审计 47 项通过。后端沿用 `/tmp` 隔离 modfile 和缓存 Go 1.27.1 做兼容验证，没有更改仓库 Go 1.27.2 或安全依赖。需要本地监听的 WS 图片用量测试受沙箱限制；无监听的图片用量归集、普通／异步单图和退役路由测试通过。不把编译检查等同于数据库集成或完整安全门禁。功能排除、历史文件比对、冲突标记、`git diff --check` 和目标仓库完整决策 schema 校验通过。

## 2026-10-12 全量验证集中修复（第 1/3 轮）

已逐一读取失败清单中的七份独立日志。后端普通测试、单元测试、集成测试、静态检查、golangci-lint、漏洞检查和生产构建都在包加载前失败，共同错误为 `go.mod requires go >= 1.27.2 (running go 1.27.0; GOTOOLCHAIN=local)`，日志没有报告具体业务测试断言失败。

同步脚本改为按候选 `backend/go.mod` 为每个后端检查选择镜像，保留显式自定义镜像配置；样例配置不再固定旧版本。检查工具缓存按 Go 版本及平台隔离，golangci-lint 与 CI 同步到 v2.14.0。CI、安全扫描及 Release 的构建和发布节点按选定源码精确验证 Go 版本，避免固定版本断言误匹配或阻止旧标签构建。三个 Docker 构建入口已使用 1.27.2，无需修改；发布归档、校验和、源码提交绑定和镜像标签回归通过。

本次仅修改自动化、工作流、相应测试和文档；未修改 Go 安全版本、依赖、业务代码、历史 SQL 或迁移校验和兼容记录，也没有新增排除路径或未解决冲突。共享账号池与批量生图继续排除，普通生图与异步单图任务保持。同步回归 52 项、发布辅助回归 11 项、功能排除检查和脚本语法检查通过。

执行限制：当前修复沙箱连接 Docker socket 返回 `permission denied`，因此未实际重跑上述七项容器检查。正在运行的外层旧脚本已加载 `GO_IMAGE=golang:1.27.0`；只修改候选工作树不能更新其内存函数、外部脚本或配置。本次没有访问外部配置或密钥，没有对候选或远程仓库执行 fetch、commit、push、打标签、发布或重启服务。必须由使用新版脚本和满足 Go 1.27.2 要求镜像的外层环境重新完成全量验证；不能将上述辅助回归视为七项后端检查已通过。

## 2026-10-12 全量验证集中复核（第 2/3 轮）

再次逐一读取失败清单中的七份独立日志，全部仍在包加载前报错：`go.mod requires go >= 1.27.2 (running go 1.27.0; GOTOOLCHAIN=local)`。受阻检查为后端普通测试（`go test ./...`）、单元测试（`go test -tags=unit ./...`）、集成测试（`go test -tags=integration ./...`）、静态检查（`go vet ./...`）、golangci-lint、govulncheck 和后端生产构建。golangci-lint 退出码为 3，其余为 1；没有执行到具体业务测试断言，不能据此认定业务测试通过或失败。

完整日志显示第 2 轮全量验证在 `18:37:15Z` 开始，七项后端检查在 `18:37:38Z` 至 `18:37:41Z` 重复工具链错误。候选已包含上一轮的动态镜像选择、工具缓存隔离和 CI／Release 工具链校验，修改候选脚本不会替换外层父进程已加载的函数及配置。显式 `SUB2API_GO_IMAGE` 仍按设计优先于默认值；即使采用新版脚本，保留旧镜像覆盖值也会继续失败。

本轮核对 CI、安全扫描、Release 构建／发布节点、完整及简化 GoReleaser 配置、归档校验和、源码提交绑定及镜像元数据，未发现这七项日志之外的新发布错误。工具链回归 5 项、发布辅助回归 11 项均通过，功能排除、脚本语法及差异空白检查通过。仅补充配置样例中的生效边界说明及本记录；不修改业务实现、Go 1.27.2 安全要求、依赖、历史 SQL 或迁移校验和兼容记录，没有新增排除路径或未解决冲突。

七项后端检查仍未解除阻塞，不能标记本次集中修复成功。恢复条件是外层执行环境采用修复后的同步脚本，并清除旧 `SUB2API_GO_IMAGE` 覆盖值以按候选选择 `golang:1.27.2`，或提供满足候选最低版本的自定义镜像，再对保留候选执行完整验证。此条件涉及本轮禁止修改的外部运行状态，不能通过工作树内降级安全版本、绕过检查、修改父进程或隐式重启来解决。本轮未读取外部配置或密钥，未执行 fetch、commit、push、标签、发布、服务重启或通知；没有推送候选。

## 2026-10-12 全量验证集中复核（第 3/3 轮）

逐一读取本轮失败清单的全部七份独立日志，结果如下。日志位于 `state/20261012T173001Z-1529495-check-*`，下表列出唯一后缀，便于定位；七项均因 `go.mod requires go >= 1.27.2 (running go 1.27.0; GOTOOLCHAIN=local)` 在模块加载阶段失败，未执行到业务测试断言。

| 检查 | 独立日志后缀 | 退出码 |
| --- | --- | --- |
| 后端普通测试 `go test ./...` | `_.log.02up8J` | 1 |
| 后端单元测试 `go test -tags=unit ./...` | `_.log.Bol49X` | 1 |
| 后端集成测试 `go test -tags=integration ./...` | `_.log.DVD9GZ` | 1 |
| 后端静态检查 `go vet ./...` | `_.log.TDENCZ` | 1 |
| `golangci-lint` | `golangci-lint.log.oNDcNI` | 3 |
| 后端漏洞检查 `govulncheck ./...` | `_.log.1RV0p5` | 1 |
| 后端生产构建 | `_.log.39UVU2` | 1 |

完整日志记录这七项检查在 `18:48:53Z` 至 `18:48:57Z` 再次失败。候选的动态 Go 镜像选择、按工具链隔离 lint／漏洞工具缓存、CI 与 Release 的精确版本校验均已存在；本轮工具链回归 5 项及 Release／归档／校验和／源码绑定／镜像元数据回归 11 项全部通过。这些使用替身的辅助回归不代表实际后端测试、漏洞扫描或发布构建通过。

本轮仅补充审查记录。剩余七项共同依赖外层运行环境更新已加载的旧脚本和 Go 镜像配置，无法仅在本轮允许修改的候选工作树内安全解除；继续修改候选脚本也不会更新父进程。恢复条件沿用上一节，不降低 Go 1.27.2 安全要求，不访问外部配置或密钥，不执行提交、推送、发布或重启。没有新增排除路径或未解决冲突；业务实现、历史 SQL、迁移校验和兼容记录保持原样。本轮不能报告集中修复成功，候选尚未推送。

## 2026-10-13 定时同步恢复

复用 2026-10-12 已审查候选及两路上游，不重新引入任何已排除功能。修复实际调度环境固定 Go 1.27.0、修复后仍复用父进程旧验证函数、失败阶段误报，以及修复提交检查失败后提前退出的问题。后端镜像按候选 go.mod 选择，验证函数每轮重新加载，工具缓存按 Go 版本隔离。CI／Release 同样从选定源码读取工具链要求。

集成测试改为提供 Docker 访问和临时 PostgreSQL，避免无 Docker 或无 MIGRATION_TEST_DATABASE_URL 时悄悄跳过。真实执行发现账号并发状态测试的临时表缺少已有迁移 233 中的 rate_limit_429_retry_count，补齐测试夹具；生产模型和历史 SQL 不变。CI 同步提供临时 PostgreSQL。

EasyPay 实现与主上游完全一致；按既定用户决策将上游 d1aac6b98 的附加回调字段白名单兼容限制记录为已接受发布风险，保留签名、防伪、防重放及对账恢复，不接受新增签名绕过。共享账号池与批量生图排除范围不变，无新增排除路径。

## 2026-10-13 远程检查集中修复（第 1/3 轮）

本轮逐一读取失败清单的两份独立日志：`20261013T173001Z-3247539-remote.log.4OOoDh`（CI）和 `20261013T173001Z-3247539-remote.log.LDsHHx`（Security Scan）。两者仅记录提交 `95c94f86fc9a34d96dd0332360ba030ae9dd2443` 的工作流“未确认结束或等待超时”；完整日志显示分别等待约 90 分钟，没有作业 ID、具体失败测试或 API 错误。因此不能将这两项认定为业务测试失败，也不能报告远程验证通过。

修复了可从源码确认的共同问题：工作流列表查询不再丢弃错误或将无效 JSON 当成空列表，连续三次查询错误后保留诊断并停止；重跑查询同样保留错误并限制连续失败次数。等待超时记录已观察到的作业 ID 和状态，CI、安全扫描及 Release 的独立失败日志包含查询诊断。尚未确认结束或作业日志采集不完整时保留候选，不进入猜测性的代码修复和推送。失败日志下载会尝试所有失败作业，任一下载失败都返回失败；已有成功/失败判断、重跑次数、提交匹配和发布门禁仍然有效。

验证：同步及安全辅助回归 85 项通过（含新增远程查询与日志回归 9 项），Release 工具链、打包矩阵、归档校验和、源码绑定及镜像元数据回归 11 项通过；同步/验证/镜像脚本语法检查、功能排除检查和 diff 空白检查通过。没有证据表明本轮 Release 工作流或发布产物存在独立故障，未凭超时日志修改打包或发布版本。

两项真实远程检查仍未确认完成：旧逻辑已经丢弃查询错误，现有日志无法恢复直接原因；本轮未访问凭据或远程 API，不能确定是查询失败、缺失运行记录还是远程排队/执行。工作树修复不会替换外层进程已经加载的旧等待函数，须由外层后续加载修复后的脚本再确认远程结果。本轮只修改工作树及本地测试临时目录，未对目标仓库执行 fetch、commit、push、标签、发布或重启操作。无新增排除路径和未解决冲突，业务代码、历史迁移及校验和兼容记录均保持原样。

## 2026-10-16 主上游冲突修复

本阶段审查 `origin/main` / HEAD `70f1aea3f` 与 `upstream/main` `b0a02a330` 的在途合并，读取第二上游 `419b579ab` 的提交和差异。累计发布基准为 `v2026.10.13`（`13e903e42`）；基准到合并前 HEAD 只有版本号同步，主上游新增 63 个非合并提交。本阶段不替代外层后续第二上游合并及最终双上游审查；第二上游新到达的 QQ 群助手、活动、转账确认、质量统计恢复及顺序渠道检查等更新仍应由该阶段完整合并。

23 个冲突路径逐项融合：保留本项目日期版本、Codex 门票时序归一化和调度凭据 `chatgpt_plan_type`，加入最小输出 token 配置、显式缓存候选凭据和 Gemini 缓存服务生命周期；重试耗尽同时保留模型级隔离、钱包错误换号和脱敏。Responses HTTP / WS 采用统一时间戳，同时保留错误事件身份、序列和计费信息脱敏。账号筛选采用主上游恢复的内联布局，并保留第二上游质量暂停筛选和对应行为测试。

语义补充：Gemini 模型列表功能迁移到新文件后继续使用本项目账号 429 重试策略；绑定上游网关缓存的请求则绕过该透明重试，维持主上游“不重试、不惩罚整个网关账号”的约束。响应体重建保留重试标记和 Request。显式缓存纯存储费不套用按次账号统计价，普通请求仍按实际回退分组和渠道计费。DeepSeek Pro 已有价格规则不回退为过时的 Pro→Flash 切换；新增缓存存储价格完整保留。Antigravity 普通图片测试、映射后模型协议判断、自定义提示词及图片输出，与现有截断/未完成响应拒绝校验共同生效。旧计费与流式响应测试同步新函数签名，并修复新增 Vertex 缓存错误用例引用不存在夹具的问题。

本轮明确排除的批量生图位置：

- `backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen.go`：不恢复批量清理器、worker 参数或停止钩子，仅加入 Gemini 显式缓存服务；现有账号健康服务保留。
- `backend/cmd/server/wire_gen_test.go`：不恢复批量清理器和 worker 测试参数。
- `backend/internal/repository/wire.go`：不恢复 `NewBatchImageRepository`。
- `frontend/src/i18n/locales/en/batchImage.ts`、`frontend/src/i18n/locales/zh/batchImage.ts`：删除合并产生的专属文案文件。
- `frontend/src/i18n/locales/en/admin/accounts.ts`、`frontend/src/i18n/locales/zh/admin/accounts.ts`：Vertex API Key 提示中只删除批量生图说明，保留显式/隐式缓存和认证限制。

本轮无新增共享账号池专属路径；机器排除规则继续通过。普通账号代理池、API Key 分组回退、质量保护、定时测试、模型身份校验、Codex 门票与身份隔离、上游计费限流以及普通/异步单图保持。发布基准中的 349 个历史 SQL / 校验和兼容文件逐字节不变，仅新增主上游 243–245 三个 Gemini 缓存迁移；不删除历史表、记录或冻结余额。EasyPay 仍拒绝非标准附加回调字段，沿用此前已接受的兼容限制；本轮同时保留主上游主动查询确认、商户绑定、退款待定扣款和对账恢复。

没有执行 fetch、git add、commit、push、标签、发布或服务重启；真实索引的冲突 stage 记录供外层继续核对。

支付安全复核先用回归复现 EasyPay 查询金额为字符串 `NaN` 时误通过金额确认的问题，再同时校验查询金额与回调金额必须为有限正数；保留正常金额容差、签名和商户校验。新增回归使用内存 HTTP transport，不依赖网络监听。

本阶段验证：前端 393 个测试文件、3197 项测试全通过，类型检查、ESLint、i18n 完整性和生产构建通过；后端配置、协议转换、Antigravity 基础包，以及服务/处理器/管理接口/仓储/依赖注入专项通过，支付防伪、金额有限性、退款待定扣款、回退路由专项通过；后端构建、相关包 go vet、Responses 时间戳专项和仓储/迁移 integration 标签编译通过。后端兼容验证使用缓存 Go 1.27.1 及 `/tmp` 隔离 modfile，并将本机缺失的五项 golang.org/x 模块仅在该临时文件映射到缓存版本，未改动仓库 Go 1.27.2 或依赖要求。这不等同于正式工具链全量验证、数据库集成执行或漏洞扫描；需要本地监听的支付测试受环境限制未完成。外层仍应对候选执行完整正式验证。功能排除、真实源码冲突标记和 `git diff --check HEAD` 通过，66 条真实冲突索引 stage 记录未改动。

## 2026-10-16 第二上游集中冲突修复（第 1/3 轮）

审查发布基准 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）到当前候选的整个累计窗口：主上游 `b0a02a330` 有 63 个非合并提交，第二上游 `419b579ab` 有 103 个；两路去重为 104 个，其中第二上游独有 41 个。`origin/main` 到当前 HEAD `61cd26286` 已包含本轮主上游合并；此前未发布的 Gemini 显式缓存、Vertex 认证/模型目录、Argon/Haiku、思考兼容、Responses 时间戳、Web Search、工作区隔离、支付查询确认和退款待定扣款均纳入本次复核。发布基准到同步前项目基线仅有日期版本同步。

56 个冲突路径逐项核对，保留本项目日期版本与第二上游 `FORK_VERSION`。依赖注入同时包含 QQ 服务、Gemini 缓存和账号健康生命周期；Ent 继续采用移除批量字段后的索引。设置 API、DTO、前端、持久化及缓存保留本项目网关运行策略、账号身份/TLS 保护、自适应并发、代理池、429 重试和计费不变量。定时测试的恢复状态扫描与新日程列同时合入，账号测试用量观测覆盖插件/TLS 路径；计划任务自行管理重试预算，避免透明 429 重试形成额外付费探测。

第二上游主动采票退役与本项目已有采票功能发生语义冲突。本次按“保留本项目已有功能”的要求保留 Codex 292/332 采票、240 秒上限、专用代理、账号开关、脱敏诊断、HTTP/WS 注入及连接复用时的新鲜度检查。新迁移 `269_retire_codex_ticket_harvesting.sql` 适配为只清理已经不用的账号私有采票代理字段与旧代理 ID 设置；保留正在使用的总开关、全局代理、票据及 Cookie。导入、创建、更新和仓储写入同样只清理这些废弃字段。对应单元与迁移回归验证保留有效配置、非对象 extra、幂等性与原业务字段。没有改写已发布 SQL。

DeepSeek Pro 保持已确认的独立价卡，不重新采用已撤销的 Pro→Flash 定时降价；已公开的 Flash 别名继续保留。渠道模型同步保留每模型默认价、手工价格与过期响应保护，新增账号模型目录替换和批量同步完整合入。前端继续使用第二上游新增的真实消息编译，测试改为断言实际翻译和新的统计恢复探测语义。

第二上游其余功能均保留：QQ 群助手的签名回调、绑定、邮箱确认、群发现、提及、引用、聊天与普通单图、活动/签到/邀请/抽奖、充值资格、保底与自定义命令、记录/绑定管理、余额转账和手续费确认；账号平台计数与标签页、模型映射替换、质量暂停详情、每日时段与时区、组合规则、顺序检查、统计恢复探测、失败退避和用量归集、样本不足展示、调用错误页、冷却默认值与图片依赖安全升级。QQ 使用单条消息任务，未引入批量生图功能。

QQ 转账安全复核补充了具体修复：锁定绑定及双方账号，核对确认时的邮箱身份和余额；稳定的账号锁顺序防止反向转账锁序冲突；令牌在等待锁后再次检查到期，入账失败或令牌过期会回滚扣款。金额文本在 decimal 缩放之前限制长度、精度和普通十进制格式，避免极端科学计数指数造成无界分配。确认令牌请求体不写审计日志。新增事务回归覆盖解绑、邮箱变化、收款账号缺失、余额不足、到期及入账失败。CI 和外层完整验证脚本补传 `QQBOT_TEST_DSN`，使新增数据库测试不会因缺少该变量而静默跳过。

本阶段明确排除批量生图的冲突位置：

- `backend/cmd/server/wire_gen.go`：合入 QQ 处理器和清理钩子，排除 `batchImageHandler`、清理器和 worker。
- `backend/ent/mutation.go`、`backend/ent/runtime/runtime.go`：维持删除批量分组字段后的容量和字段索引。

累计窗口同时复核主上游阶段的排除：`backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen_test.go`、`backend/internal/repository/wire.go` 的批量依赖；`frontend/src/i18n/locales/en/batchImage.ts`、`frontend/src/i18n/locales/zh/batchImage.ts` 专属文件仍不存在；`frontend/src/i18n/locales/en/admin/accounts.ts`、`frontend/src/i18n/locales/zh/admin/accounts.ts` 不含批量生图说明，其 Vertex 认证和缓存说明保留。

共享账号池明确排除 `backend/internal/handler/dto/types.go` 冲突侧的 `SharedTotalCallCount`、`SharedListingStatus`，保留普通账号、代理池及 429 重试字段。`backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go` 继续不引入，不清理历史共享账号数据。普通生图与异步单图任务继续支持。发布基准的 323 个历史 SQL/迁移校验和兼容源码文件逐字节未变，历史批量表、记录与冻结余额不做处理。

剩余具体兼容限制沿用此前已接受的 EasyPay 非标准附加回调字段拒绝策略；保留主动查询/对账、商户绑定、防伪与有限正金额校验。该限制不是新发现的采票、QQ 或数据库风险。

只修改本临时工作树，未对目标仓库执行 fetch、git add、commit、push、标签、发布或服务重启；155 条真实冲突索引记录仍保留供外层临时索引核验。

本阶段验证：前端 407 个文件、3302 项测试全通过，类型检查、ESLint 与生产构建通过；同步自动化回归共 100 项，98 项通过、2 项按条件跳过。后端专项测试及子用例 2349 项通过，覆盖采票与 compact 出站门控、定时测试、计费、账号设置、QQ 转账等；需要监听端口的用例仅在临时运行脚本中排除，仓库测试未禁用。后端生产构建、相关包 go vet、QQ/仓储/迁移 integration 标签编译通过。后端验证沿用 `/tmp` 隔离 modfile 和缓存 Go 1.27.1，仅在临时文件映射本机缺失的依赖，仓库 Go 1.27.2 和依赖要求未降级；这些兼容验证不代表正式全量 CI、实际数据库集成执行或漏洞扫描通过。功能排除、真实源码冲突标记、历史迁移及校验兼容源码、差异空白和原始冲突索引不变检查均通过，外层可继续使用临时索引核验并执行完整正式验证。

## 2026-10-16 最终双上游兼容审查

最终候选为 `972c6a86a`，已包含主上游 `b0a02a330` 和第二上游 `419b579ab`。本阶段开始时外层已经完成两次合并，工作树干净，真实索引没有未合并条目；上一阶段的 155 条冲突索引记录描述的是当时状态，不代表本阶段仍有冲突。本阶段没有执行 fetch、暂存、提交、推送、标签、发布或服务重启，只补充本审查记录。

累计窗口仍从已发布的 `v2026.10.13`（`13e903e42`）计算，而非从第二次合并前的 HEAD 计算：两上游去重后 104 个非合并提交，主上游 63 个，第二上游独有 41 个；候选相对发布树涉及 499 个文件。发布基准至 `origin/main=70f1aea3f` 仅有服务版本同步。通过临时对象目录重建两次合并的冲突差异，复核手工融合，并对照两个上游的提交、文件增删与当前实现，未发现需要再次改写业务代码的遗漏或冲突。

- 主上游的 Gemini 显式缓存及存储计费、Vertex API Key／服务账号模型目录、Argon／Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、模型级错误策略、支付主动查询和退款待定扣款均保留。缓存绑定错误仍遵守不透明重试、不惩罚整个网关账号的边界；纯存储费不进入按次账号统计价。
- 第二上游的 QQ 群助手、签名回调、邮箱绑定、提及和引用、普通单图、活动／签到／邀请／抽奖、资格窗口、保底、自定义命令、记录管理、转账预览及手续费确认完整保留；账号平台统计、模型目录替换及批量同步、质量暂停详情、每日时段／时区、顺序检查、统计恢复探测、失败退避、测试用量归集、样本不足展示、调用错误页和图片依赖安全升级均保留。
- 复核已有功能与新实现的交叉边界：采票设置及 HTTP／WS 注入保留，迁移 269 只清理废弃私有代理字段；账号身份、TLS、普通代理池、自适应并发和 429 策略继续生效。定时测试自行控制重试预算。DeepSeek Pro 独立价卡、Flash 别名、手工价格及过期查询响应保护继续保留。
- QQ 转账仍在确认事务内核对绑定、双方邮箱、有效账号和余额，按稳定顺序锁定账号，在等待后再次校验令牌有效期，失败回滚；提交后使双方余额及认证缓存失效。公开确认入口保留限流、二次点击确认和令牌请求体脱敏。EasyPay 查询金额和回调金额均拒绝非有限值与非正数。

本轮明确复核并排除以下位置，混合用途文件的其他实现保持：

- 共享账号池：`backend/internal/handler/dto/types.go` 的 `SharedTotalCallCount`／`SharedListingStatus`；继续排除 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go`。
- 批量生图：`backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen.go`、`backend/cmd/server/wire_gen_test.go`、`backend/internal/repository/wire.go` 的批量依赖；`backend/ent/mutation.go`、`backend/ent/runtime/runtime.go` 的批量字段容量及索引；`frontend/src/i18n/locales/en/batchImage.ts`、`frontend/src/i18n/locales/zh/batchImage.ts` 专属文件；`frontend/src/i18n/locales/en/admin/accounts.ts`、`frontend/src/i18n/locales/zh/admin/accounts.ts` 中批量生图说明。

重新逐字节比对发布树中的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go`，共 323 个文件，均未改变；迁移 108 的兼容加宽、243–245 的缓存表及 269–277 的采票兼容／定时测试／QQ 迁移作为新增文件保留。不删除历史表、记录或冻结余额；普通生图和异步单图的路由、处理器及服务继续存在。

本阶段补充验证：前端 25 个文件、167 项相关回归及类型检查通过；同步自动化回归 98 项通过、2 项按条件跳过；QQ 转账授权／回滚／金额解析和 EasyPay 有限正金额专项通过。服务层 333 个测试函数连同子用例共 1037 项通过，覆盖采票、定时测试、缓存存储及账号统计计费；仓储、管理员处理器和普通处理器的相关专项通过。一次扩大范围的服务层检查因采票代理测试需要本地监听而中断，随后在临时运行命令中选择无需监听的专项，仓库测试未禁用。后端仍使用 `/tmp` 隔离 modfile 与缓存工具链进行兼容验证，未修改仓库工具链或依赖要求，外层应继续执行正式完整验证。完整验证与构建结果以上一阶段记录为准，本段不将专项检查表述为全量通过。功能排除检查、实际源码冲突标记检查和差异空白检查均通过。

剩余具体兼容限制仍为此前已接受的 EasyPay 非标准附加回调字段拒绝策略，需要标准回调或主动查询／对账恢复；没有新增产品、权限、计费或数据库风险。最终未解决冲突为空。

## 2026-10-16 全量验证集中修复（第 1/3 轮）

逐项读取七项失败检查的独立日志后，集中处理以下共同原因：

- 模型同步事务测试的 SQL mock 补入现有账号保护字段保留表达式，继续验证模型映射与元数据原子保存，以及 outbox 失败回滚；未放宽生产写入保护。
- 重试耗尽的计费失败分支删除提前设置的响应提交标记，使账号切换仍可继续；非计费错误写出前的提交标记保留。
- 结构化输入协议回退测试显式设置账号 429 重试预算为零，独立验证协议回退次数；生产默认预算与共享重试预算实现保持，既有 429 专项继续验证。
- QQ 全群命令集成测试跟随第二上游明确的 `/我的额度` → `/我的余额` 改名，补充未入队时的具体命令诊断。权限、绑定确认、转账事务和日志隐私逻辑未变。
- 修复事务回滚与类型断言的 errcheck、Antigravity 错误文本及两个无调用包装函数的 lint 问题，保留完整响应校验和普通单图解析。
- Vitest 设置最小 worker 数，兼容外层 `--maxWorkers=2`。验证安装步骤拒绝复用指向其他工作树的依赖链接，并清理 Vue 全局类型与 TypeScript 增量缓存，避免新旧 Vue 类型及运行时混用。Release 工作流、归档及发布元数据回归纳入每轮完整验证。
- 前端完整重跑额外发现余额通知的三条计时器断言把 Vitest 自身的 10ms 任务上报 timeout 计入组件泄漏。改为直接断言验证码倒计时的 `setInterval` 未启动，继续保护删除邮箱或卸载后的异步响应不创建计时器，不修改组件生命周期逻辑。

本轮没有新增共享账号池或批量生图排除路径，既有排除列表继续生效；没有未解决的合并冲突。所有 SQL 迁移及 `migrations_runner.go` 保持本轮修复前内容，不处理历史表、记录或冻结余额。仅修改候选工作树，未 fetch、暂存、提交、推送、打标签、发布或重启服务。

验证记录：后端普通专项及 unit 标签的 112 个测试／子用例通过，本地 golangci-lint 零问题；使用缓存 Go 1.27.1、golangci-lint 2.13 和 `/tmp` 隔离 modfile 依赖映射，未改动仓库 Go 1.27.2 或依赖要求，不替代外层正式工具链验证。QQ 命令配置专项通过；`TestQQBotFullGroupCommandsAndMentionedChat` 的 integration 标签编译通过，但因本地缺少 `QQBOT_TEST_DSN` 跳过执行，沙箱也无法连接 Docker，须由外层数据库环境复验。前端完整执行 407 个文件、3302 项测试，3299 项通过，额外发现的三条余额通知断言修复后该文件 5 项全部通过；类型检查、生产构建、i18n 与 ESLint 通过。验证脚本回归 12 项、Release 工作流／打包元数据回归 11 项及 Release actionlint 通过。功能排除、真实冲突标记与差异空白检查通过，外层仍须在本轮修复结束后执行完整验证。

## 2026-10-17 第二上游集中冲突修复（第 1/3 轮）

本轮项目基线为 `origin/main=HEAD=c29dab54f`，主上游 `upstream/main=b0a02a330` 已包含在基线中；第二上游推进到 `807fb4b89`，新增 `33fb94be9` 的 QQ 群共享上下文和 `807fb4b89` 的 GPT 直充。累计审查仍以已发布的 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）为起点：主上游 63 个、第二上游 105 个非合并提交，两路去重 106 个，第二上游独有 43 个。此前已合并但未发布的更新继续纳入审查，不能只按本轮两个提交描述发布内容。

### 文本与语义融合

- `backend/cmd/server/VERSION` 保留本项目日期版本 `2026.10.13`；`FORK_VERSION` 保留第二上游 `v0.2.17-custom.11`。
- `backend/cmd/server/wire_gen.go` 提前构造唯一的设置服务供 GPT 直充与 QQ 共同使用，补回本项目 `NewProxyRepository` 的配置参数；合入 `gptDirectHandler`，排除冲突侧 `batchImageHandler`，保留普通/异步单图、Gemini 缓存和账号健康生命周期。
- `backend/internal/server/middleware/audit_log.go` 同时保留 QQ 转账预览/确认与五个 GPT 直充接口的整段凭证请求体排除规则；审计回归同时覆盖这七个入口，防止解决一侧冲突时泄露另一侧的令牌。
- 逐文件重建本轮 58 个上游路径的三方合并：54 个与无冲突合并结果一致，3 个文本冲突按上述语义融合，另 1 个为补强的审计回归；没有选择性遗漏第二上游的其他增量。

### 保留行为与累计复核

QQ 群上下文按应用/群隔离、同群任务串行，兼容旧 `history_turns` 配置，保留显式零禁用、消息条数和 token 上限、空闲过期、重投去重及明确引用。旧个人会话不迁入共享历史，绑定/转账等命令及其引用结果不写入共享上下文；普通单图继续支持。这里的“共享上下文”不是被排除的共享账号池。

GPT 直充保留管理员配置、默认关闭的菜单开关、购买、验码、订阅查询、确认兑换、任务查询和浏览器恢复；用户接口保留认证与面板限流，服务器固定产品/渠道，写请求不自动重试、不跟随重定向，任务凭据不进入浏览器 URL，公开配置与上游返回按白名单投影。直充 CDK 独立于本站余额兑换码，不改写本站账户余额或原有计费。

累计窗口保留 Gemini 显式缓存/存储计费、Vertex API Key 与服务账号模型目录、Argon/Haiku、思考兼容、Responses 时间戳、Web Search、工作区隔离、模型级错误处理、支付查询确认与退款待定扣款；同时保留第二上游账号平台统计、模型目录替换/同步、质量保护与恢复探测、每日时段/时区、顺序检查、失败退避、测试用量归集、QQ 活动/邀请/抽奖/转账、调用错误页和安全依赖更新。复核本项目 Codex 采票/身份保护、普通代理池、429 重试、DeepSeek Pro 独立价卡、纯缓存存储不按次计价、QQ 转账身份行锁/到期复核/失败回滚及 EasyPay 有限正金额校验均仍存在。迁移 269 继续只清理废弃私有采票代理字段，不移除有效采票设置。

### 本轮明确排除位置

- 批量生图：`backend/cmd/server/wire_gen.go` 冲突侧的 `batchImageHandler` 及批量依赖；累计复核 `backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen_test.go`、`backend/internal/repository/wire.go` 的批量依赖仍被排除，`backend/ent/mutation.go`、`backend/ent/runtime/runtime.go` 不恢复批量分组字段。`frontend/src/i18n/locales/en/batchImage.ts`、`frontend/src/i18n/locales/zh/batchImage.ts` 仍不存在；`frontend/src/i18n/locales/en/admin/accounts.ts`、`frontend/src/i18n/locales/zh/admin/accounts.ts` 保留 Vertex 认证/缓存说明但不恢复批量生图说明。
- 共享账号池：`backend/internal/handler/dto/types.go` 不恢复 `SharedTotalCallCount` / `SharedListingStatus`；`backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go` 继续排除。普通账号池及代理池保持。

发布基准的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go`（共 323 个文件）逐字节未变；本轮新增迁移 278 仅创建群历史表与引用可共享标志，不搬运旧个人会话。不删除批量生图历史表、记录或冻结余额。

剩余已知兼容限制仍是 EasyPay 拒绝非标准附加回调字段，此类支付需使用标准回调或由主动查询/对账恢复；未发现新增的产品、权限、计费或数据库风险。本阶段仅修改临时工作树，没有 fetch、git add、commit、push、标签、发布或服务重启，真实索引的 9 条冲突 stage 记录保持原样供外层核验。

本阶段验证：前端 7 个相关测试文件、63 项测试全部通过，类型检查、ESLint、i18n 检查及生产构建通过。后端 QQ 群历史、直充设置/鉴权、七个敏感接口审计及服务清理专项通过；直充请求隔离/幂等/禁止重定向与重试、QQ 转账授权/回滚/金额边界、缓存及账号统计计费、EasyPay 有限金额专项通过；后端生产构建与 QQ/迁移 integration 标签编译通过。后端采用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile 映射本机缺失依赖，仓库 Go 1.27.2 与依赖声明未改动；直充 HTTP 用例通过临时 Go overlay 使用内存 transport，保留原断言且未修改仓库用例。一次扩大审计匹配范围的检查命中了需要本地监听的既有内容审核测试，随后改为精确匹配本次相关用例；未禁用仓库测试。以上是兼容专项验证，不代表正式工具链全量测试、真实数据库集成或漏洞扫描通过，外层仍需完整验证。

最终功能排除、真实源码冲突标记、差异空白及原始冲突索引不变检查通过；源码未解决冲突为空。

## 2026-10-17 最终双上游兼容审查

本阶段起始候选为 `25930a081`，项目基线为 `origin/main=c29dab54f`，已包含主上游 `b0a02a330` 和第二上游 `807fb4b89`。外层已完成合并，进入本阶段时工作树干净、索引没有未合并条目；上节的 9 条冲突 stage 记录属于前一阶段。本阶段不 fetch、不暂存、不提交、不推送、不打标签、不发布、不重启服务。

累计审查从已发布的 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）计算，覆盖两上游去重后的 106 个非合并提交（主上游 63 个、第二上游独有 43 个）。核对累计提交记录、双方文件差异、前次融合差异和当前实现；重新逐文件复算本轮第二上游的 58 个路径，54 个与自动三方合并一致，版本、依赖注入和审计脱敏三个冲突文件已语义融合，另一个差异为敏感接口联合审计回归。

最终复核发现并修复 GPT 直充传输层自动重放：`bytes.Reader` 使 `http.NewRequestWithContext` 自动设置 `GetBody`，兑换请求又携带 `Idempotency-Key`，因此 Go 默认传输层会在复用连接丢失响应时自动重放兑换 POST。新增 `backend/internal/service/gpt_direct_transport_test.go`，以真实 `http.Transport` 和内存连接模拟上游接收兑换后断开响应；修复前一次调用产生两次兑换请求，回归失败。`backend/internal/service/gpt_direct_service.go` 现对 POST 清空 `GetBody`，关闭请求体重放；修复后回归通过，同时验证用户主动重试仍可沿用原幂等键取得回执。未修改直充的购买、验码、订阅查询、任务查询、鉴权、渠道隔离、审计脱敏或本站余额计费。

此前未发布的更新继续保留：Gemini 显式缓存与存储计费、Vertex 认证及模型目录、Argon/Haiku、思考兼容、Responses 时间戳、Web Search、工作区隔离、模型级错误策略、支付主动查询和退款待定扣款；第二上游的账号平台统计、模型同步、质量保护、每日时段/时区、顺序检查、统计恢复探测、失败退避、测试用量归集、QQ 活动/绑定/邀请/抽奖/转账、调用错误页和安全依赖更新。QQ 群共享上下文及 GPT 直充完整保留；群上下文不是共享账号池，仍按应用/群隔离并排除账号命令及私密引用。已有 Codex 采票、身份保护、普通代理池、429 策略、DeepSeek Pro 独立价卡、QQ 转账事务保护和纯缓存存储不按次计费均保留。

本阶段明确复核并继续排除以下位置，混合用途文件中的其他功能保留：

- 共享账号池：`backend/internal/handler/dto/types.go` 中的 `SharedTotalCallCount` / `SharedListingStatus`，以及专属的 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go`。
- 批量生图：`backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen.go`、`backend/cmd/server/wire_gen_test.go`、`backend/internal/repository/wire.go` 中的批量依赖；`backend/ent/mutation.go`、`backend/ent/runtime/runtime.go` 中的批量分组字段；`frontend/src/i18n/locales/en/batchImage.ts`、`frontend/src/i18n/locales/zh/batchImage.ts` 专属文案；`frontend/src/i18n/locales/en/admin/accounts.ts`、`frontend/src/i18n/locales/zh/admin/accounts.ts` 中的批量说明。
- 累计双方文件差异中再次确认不引入的批量专属路径：`backend/ent/batchimagejob.go`、`backend/ent/batchimagejob/batchimagejob.go`、`backend/ent/batchimagejob/where.go`、`backend/ent/batchimagejob_create.go`、`backend/ent/batchimagejob_update.go`、`backend/ent/schema/batch_image_job.go`、`backend/internal/handler/batch_image_handler.go`、`backend/internal/repository/batch_image_repo.go`、`backend/internal/service/batch_image.go`、`backend/internal/service/batch_image_processor_test.go`、`backend/internal/service/batch_image_public.go`、`backend/internal/service/batch_image_public_test.go`、`backend/internal/service/batch_image_settlement.go`。

发布树中的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go`，共 323 个文件，重新逐字节比对一致。本阶段不改写迁移，不删除历史表、记录或冻结余额。累计新增迁移继续保留，迁移 269 只清理废弃的私有采票代理字段，迁移 278 不搬运旧个人会话。普通生图、编辑及异步单图提交/查询路由继续存在。

剩余具体兼容限制仍为 EasyPay 拒绝非标准附加回调字段，此类订单需要标准回调或主动查询/对账恢复；本次发现的直充自动重放问题已修复，不再列作剩余风险。最终没有未解决的文本或语义冲突。

本阶段验证：前端 8 个相关文件、66 项测试及类型检查通过；同步自动化回归 101 项，99 项通过、2 项按条件跳过。后端普通专项 37 项、unit 标签专项 547 项测试及子用例通过，覆盖直充、QQ 群历史/转账、敏感接口审计、EasyPay 金额、服务清理、缓存存储/账号统计计费和定时测试；后端生产构建通过。新增连接中断回归使用真实 `http.Transport`，已确认修复前失败、修复后通过。后端沿用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，既有直充 HTTP 用例通过临时内存 transport overlay 执行原断言；仓库 Go 1.27.2、依赖要求和既有测试未改写。本阶段为兼容专项复验，不将其表述为正式全量测试、实际数据库集成或漏洞扫描通过。功能排除、真实源码冲突标记和差异空白检查通过，真实索引保持未暂存状态，外层继续完整验证。


## 2026-10-19 主上游集中冲突修复（第 1/3 轮）

本轮项目基线为 `origin/main=HEAD=5226b8e08`，合并主上游 `upstream/main=8f2cbb53d`；第二上游 `overdraft/sub2api-custom=807fb4b89` 已完整进入基线。使用外层已获取的引用，不 fetch、不暂存、不提交、不推送、不打标签、不发布、不重启服务；两个冲突文件的 6 条真实索引 stage 记录保持原样，供外层继续核对 `:2` / `:3`。

累计审查从已发布的 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）起算，覆盖两上游去重后 109 个非合并提交：主上游 66 个、第二上游独有 43 个（第二上游累计 105 个，包含共同祖先）。本次主上游新增 3 个非合并提交，涉及 11 个路径；审查同时覆盖此前已合并但未发布的更新，没有将累计窗口缩减到本次增量。

### 冲突及语义处理

- `backend/internal/service/gemini_chat_completions_compat_service.go`：延迟发送 `message_start`，先识别余额错误以允许首帧前换号；同时保留主上游的断连计费结果。所有退出路径统一返回已收集用量、断连标志和完成标志，在首次客户端写入前记录上游完成状态及首帧时间，避免首帧写失败丢失状态或误用图片兜底计费。
- `backend/internal/service/gemini_messages_compat_service.go`：同时保留原生 SSE 前缀缓冲与 `clientDisconnect` / `finished` 状态；保留余额错误脱敏、已输出后禁止重放、模型观测和账号错误策略。Messages、Chat Completions、原生 Gemini 三个入口继续按已收集用量处理客户端取消；完成前图片按实际支持的输出计数，完成后的模型计价规则保留。
- Chat Completions 同步原生及 Messages 的残行语义：取消或读错误携带的未终止 SSE 行不进入计费，正常 EOF 的最后一行仍处理。新增首帧写失败（未完成、已完成、空流）、取消与 EOF 尾帧、原生缓冲前缀后断连共 8 个回归子场景。已有上游断连与余额故障转移回归全部保留。
- 主上游 `9c41b3269` 的输入 token 下限修复完整保留，避免只报缓存命中时出现负输入用量；`ab71e12ec` 的异步单图下载策略完整保留，包括私网/元数据地址拒绝、直连 socket IP 校验、逐跳重定向校验、代理分支解析检查、连接池复用和图片类型校验。
- 逐文件复算本次三方合并：除上述两个冲突文件及追加回归的测试文件，其余 8 个路径与自动三方合并结果完全一致；没有省略主上游其他更新。

### 累计保留行为

主上游的 Gemini 显式缓存、存储计费及预留、Vertex API Key/服务账号模型目录、Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、模型级错误处理、支付通知主动确认、Stripe 本地订单快照与退款待定扣款均保留。

第二上游的账号平台统计、模型目录替换/同步、质量暂停与恢复探测、每日时段/时区、顺序检查、失败退避、测试用量归集、调用错误页及安全依赖升级继续保留。QQ 群助手、绑定、签到/邀请/抽奖/转账及群共享上下文完整保留；群上下文按应用/群隔离，不把命令或私密引用加入群历史，它不是共享账号池。GPT 直充保持默认关闭、认证、配置/渠道隔离、凭证审计脱敏、响应白名单和显式重试；此前修复的兑换 POST 禁止传输层自动重放仍在。

本项目的 Codex 采票/身份保护、普通账号代理池、429 重试、DeepSeek Pro 独立价卡、纯缓存存储不按次计价、QQ 转账绑定/双方邮箱行锁/到期复核/失败回滚、EasyPay 有限正金额校验继续保留。迁移 269 仅清理废弃私有采票代理字段，不删除有效采票配置。

### 本轮明确复核并继续排除的路径

共享账号池专属迁移及测试继续不引入：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

混合 DTO `backend/internal/handler/dto/types.go` 不恢复 `SharedTotalCallCount` / `SharedListingStatus`；普通账号、调度、代理池及 QQ 群上下文保持。

累计上游窗口涉及的批量生图专属路径均不存在：
- `backend/ent/batchimagejob.go`
- `backend/ent/batchimagejob/batchimagejob.go`
- `backend/ent/batchimagejob/where.go`
- `backend/ent/batchimagejob_create.go`
- `backend/ent/batchimagejob_update.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_processor_test.go`
- `backend/internal/service/batch_image_public.go`
- `backend/internal/service/batch_image_public_test.go`
- `backend/internal/service/batch_image_settlement.go`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`

混合用途文件继续排除批量依赖、分组字段及专属说明，其他实现保留：
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/cmd/server/wire_gen_test.go`
- `backend/internal/repository/wire.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `frontend/src/i18n/locales/en/admin/accounts.ts`
- `frontend/src/i18n/locales/zh/admin/accounts.ts`

重新逐字节比较发布基准的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go`，共 323 个文件，全部未变。批量生图历史迁移 159–169、187、234 及校验和兼容记录保留，不操作旧表、记录或冻结余额；新增的 Gemini 缓存、QQ 和定时测试迁移保持。普通生图/编辑和异步单图提交/查询路由仍存在。

剩余具体兼容限制沿用累计窗口已记录的 EasyPay 非标准附加回调字段拒绝策略：此类订单需要标准回调或主动查询/对账恢复。保留商户绑定、签名防伪及有限正金额校验，不为兼容未知字段放宽安全边界。

另保留主上游图片下载实现明确注明的代理信任边界：`backend/internal/service/image_storage.go:72` 的代理 Transport 不校验最终目标 socket，`:129` 的路由分支只检查本地 DNS 解析，然后将原域名交给环境代理。若代理侧解析结果不同或发生 DNS 重绑定，且代理自身不限制私网目的地，仍可能访问代理可达的私网资源；直连分支的实际 socket IP 校验不受此限制。环境代理需要执行目的地址限制；本轮保留代理下载能力，没有把本地预检描述成代理目标 IP 的强保证。该项属于代码的剩余安全边界，不是验证环境或工具限制。

没有未解决的文本或语义冲突。

### 本阶段验证

- 前端完整测试：409 个文件、3315 项测试全部通过；类型检查、ESLint、i18n 检查与生产构建通过。
- 后端 Gemini、图片转存、显式缓存、定时测试及 GPT 直充传输回归：413 个测试函数及子用例共 1405 项通过，新增 8 个断连子场景全部通过。Antigravity、协议转换和配置包测试通过；QQ 转账授权/回滚、群上下文及 EasyPay 金额边界专项通过。
- 后端生产构建、相关包 unit 标签 go vet 和 golangci-lint 2.14.0 通过，静态扫描为零问题。仓储、QQ 与迁移 integration 标签编译通过；这不表示已执行真实数据库集成测试。
- 同步自动化回归 101 项，99 项通过、2 项按条件跳过；Release 工作流/打包元数据回归 11 项通过。
- 后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，将缺失的五项 golang.org/x 模块仅映射到本机缓存版本。仓库 Go 1.27.2 与依赖声明未改动。静态扫描的临时 go 包装器只在 go/packages 的非模块标准库探测中移除 modfile 参数，业务模块加载仍使用隔离 modfile。
- 较宽的初次专项命中 Vertex 令牌和图片转存测试的本地监听限制，随后仅在临时测试选择中排除需要监听的用例并完成上述专项复跑；仓库测试未禁用或改写以绕过失败。本阶段不冒充正式工具链全量测试、实际数据库集成或在线漏洞扫描；外层继续完整验证。
- 最终功能排除、源码冲突标记、差异空白、323 个历史迁移/校验和源码逐字节不变及真实冲突索引不变检查通过。最终决策 JSON 已用项目 schema 校验通过。

## 2026-10-19 最终双上游兼容审查

本阶段起始候选为 `1b9b9af78`，项目基线为 `origin/main=5226b8e08`，已包含主上游 `8f2cbb53d` 和第二上游 `807fb4b89`。进入本阶段时工作树干净、真实索引没有未合并条目；上节的 6 条冲突 stage 记录属于已由外层完成的主上游合并阶段。本阶段仅修改临时工作树，不更新远程引用，不暂存、提交、推送、打标签、发布或重启服务。

累计审查从已发布的 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）计算，覆盖去重的 109 个上游非合并提交（主上游 66 个、第二上游独有 43 个），起始候选相对发布树共 534 个文件变化。已核对双方提交记录、累计涉及路径、与候选的逐文件差异及前阶段的三方融合；没有把审查窗口缩减为本轮主上游的 3 个提交。第二上游独有提交涉及的 286 个路径中，216 个与第二上游文件逐字节相同，66 个保留本项目兼容融合，4 个为上游自身删除或替换的路径。66 个差异包含依赖注入的批量生图移除、既有设置及账号保护、普通代理池、重试边界、审计脱敏、QQ 转账事务保护、GPT 直充禁止自动重放及测试清理，没有另行删减第二上游功能。

### 本阶段新增修复

三协议写失败回归复现了前阶段未覆盖的计费边界：Messages 和原生 Gemini 的流式处理忽略下游 `Write` 错误，在客户端已无法接收时仍读取后续上游事件；没有读取错误、仅以 EOF 结束时还会保留正常请求的图片兜底计数。Chat Completions 已有写失败处理，作为对照保留。

- `backend/internal/service/gemini_stream_client_abort.go` 新增请求内的写失败记录器，记录首次错误或短写，后续写入直接返回该错误。
- `backend/internal/service/gemini_messages_compat_service.go` 的 Messages 和原生流在读取下一事件前停止，所有返回路径保留已收集用量、完成标志及断连状态；未完成且尚无图片时不触发图片兜底。已收到完成帧的计费口径、首帧前余额故障转移、输出后的错误脱敏与禁止重放、模型观测、账号策略及原生 SSE 前缀缓冲继续保留。
- `backend/internal/service/gemini_stream_client_abort_test.go` 增加三协议对照，覆盖未完成、已完成、缓冲前缀和兼容空流的写失败，断言不再读取后续上游用量。本阶段开始时 Messages/原生的 5 个子场景失败，修复后专项通过；原有断连、正常 EOF 和上游读取错误断言保留。

### 累计功能与排除边界

主上游的 Gemini 显式缓存与存储计费/预留、Vertex 认证和模型目录、Argon/Haiku、思考摘要、最小输出 token、Responses 时间戳、Web Search、工作区隔离、模型级错误处理、支付主动确认、Stripe 订单快照、退款待定扣款，以及本轮负输入 token 修复和异步单图安全下载均保留。

第二上游的账号平台统计、模型替换与同步、质量暂停/恢复探测、每日时段及时区、顺序检查、失败退避、测试用量归集、调用错误页、安全依赖更新、QQ 活动/绑定/邀请/抽奖/转账及群共享上下文、GPT 直充完整保留。群上下文按应用/群隔离并排除命令和私密引用，不属于共享账号池。本项目 Codex 采票/身份保护、普通账号代理池、429 策略、DeepSeek Pro 独立价卡、纯缓存存储不按次计价、转账绑定及双方邮箱行锁、到期复核和失败回滚继续生效。

本阶段重新确认上节列出的 15 个累计批量生图专属路径仍不存在；混合用途文件中的批量依赖、ORM 字段和专属说明仍排除。另明确检查 `backend/internal/handler/wire.go`、`backend/internal/service/wire.go` 的批量依赖，`frontend/src/router/index.ts`、`frontend/src/components/layout/AppSidebar.vue` 的批量路由和导航，以及 `backend/internal/server/routes/gateway.go` 的 `/images/batches` 路由均未恢复；普通图片生成/编辑及异步单图提交/查询路由仍保留。共享账号池专属的 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go` 继续排除；混合 DTO `backend/internal/handler/dto/types.go` 不恢复 `SharedTotalCallCount`、`SharedListingStatus`。

发布树中的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go` 共 323 个文件再次逐字节比对一致，包括批量生图历史迁移及校验和兼容记录。不操作历史表、数据或冻结余额；累计新增迁移保留，269 仅清理废弃私有代理字段，278 不搬运旧个人会话。

仍存在两项既有具体边界：EasyPay 拒绝非标准附加回调字段，此类订单需标准回调或主动查询/对账恢复；图片下载经环境代理时，本地 DNS 预检不能保证代理最终解析地址，代理自身仍须限制私网目的地。此次写失败计费问题已修复，不列为剩余风险。工具链或验证环境限制不作为产品风险。

### 本阶段验证

- 后端 414 个专项测试函数，共 1420 项测试及子用例全部通过，包含新增的 11 个三协议写失败场景；余额故障转移、断连计费、图片下载、显式缓存、定时测试及 GPT 直充传输回归保持通过。Antigravity、协议转换、配置包以及 QQ 转账/群历史、EasyPay 金额和回调安全专项通过。
- 前端重新完整运行 409 个测试文件、3315 项测试，全部通过；使用仓库指定的 pnpm 9.15.9，类型检查、ESLint、i18n 检查和生产构建通过。
- 后端生产构建、服务包 unit 标签 go vet、golangci-lint 2.14.0（零问题），以及仓储、QQ、迁移 integration 标签编译通过。首次 lint 指出的 QF1006 循环条件已简化，随后重新运行全部 414 个后端专项（1420 项测试及子用例）、构建、go vet 和完整 lint，均通过。同步自动化回归 101 项（99 项通过、2 项条件跳过），Release 元数据回归 11 项通过。
- 后端沿用缓存 Go 1.27.1 与 `/tmp` 隔离 modfile，仓库 Go 1.27.2 和依赖声明未改动。一次较宽的 QQ 测试选择命中本地监听限制，随后收窄为无需监听的转账、群历史和支付安全专项并通过；没有改写或禁用仓库测试。以上不代表正式工具链全量测试、真实数据库集成或在线漏洞扫描通过，外层仍执行完整验证。
- 重算本轮主上游 11 个路径的三方合并：7 个与自动结果逐字节一致，其余为两个已融合冲突文件、断连辅助实现和追加回归测试。历史 SQL/校验和源码不变、功能排除、真实源码冲突标记、差异空白、决策 JSON schema 及真实索引不变检查通过；源码未解决冲突为空。


## 2026-10-20 第二上游集中冲突修复

本阶段基线为 `origin/main=043894559`，主上游为 `8f2cbb53d`，第二上游为 `5d05f665e`，三方合并共同祖先为 `807fb4b89`。只修改临时工作树；真实索引的 8 个冲突文件及其 stage 1/2/3 保留，交由外层临时索引检查后暂存。本阶段不 fetch、提交、推送、打标签、发布或重启服务。

审查窗口从已发布的 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）到候选累计计算，覆盖 116 个去重非合并上游提交：主上游 66 个、第二上游独有 50 个。本轮第二上游新增 7 个非合并提交、182 个路径；逐文件重算自动三方结果，对冲突和有意适配逐项核对。累计两上游相对发布基准涉及的路径中，没有发现排除功能以外的上游文件丢失。

### 冲突处理及本轮保留功能

- `backend/internal/service/update_service.go` 和测试：融合本项目仓库身份、双上游版本/提交比较与第二上游发布就绪清单、缓存渠道隔离、宿主机托管更新。源码/发布构建均只提示已完成的本项目发布，不把分支版本或其他仓库版本当成可安装更新；托管更新器不可用时禁止退回容器内替换二进制，并禁止托管部署的手动二进制回滚。新增回归覆盖草稿/预发布、错误清单地址、外部镜像、版本不匹配、缓存的双上游数据及实时托管状态。
- 更新清单、宿主机更新器、安装脚本、文档和页面手动命令统一绑定 `MIEnchating/sub2api`、`ghcr.io/mienchating/sub2api` 与本项目日期版本。固定摘要、版本/源码 SHA 双校验、健康检查、旧镜像恢复、任务重启恢复、从发布镜像提取更新器均保留。项目 `VERSION` 仍为 `2026.10.13`，第二上游 `FORK_VERSION` 为 `v0.2.17-custom.18`；不伪造已发布版本。
- `.github/workflows/release.yml` 保留本项目 DockerHub 凭证判断、日期标签与正式工具链校验，同时保留第二上游跨架构更新器打包、镜像摘要元数据及最终上传就绪清单的步骤。CI 的 PostgreSQL 端口索引改用字符串，符合工作流表达式类型要求。
- `backend/cmd/server/wire_gen.go` 保留本项目代理池/TLS 配置与账号健康清理，接入直充奖励后台任务及清理；复用提前初始化的渠道仓储，避免重复声明，不恢复批量生图任务。
- 定时测试仓储冲突保留 `excluded_account_ids` 的查询/扫描与行锁，并保留本项目显式关闭/回滚约定。前端统计恢复测试继续检查失败后重新冷却的既有不变量。
- 第二上游 QQ 自主群聊（默认关闭、按应用/群隔离、限额、去重、隐私过滤）、价格通知、奖励兑换码、余额/抽奖/实物奖励与直充奖励幂等结算完整保留。定时测试的账号排除、输出 token 上限、管理员移组撤销旧保护、通过/失败动作，以及渠道健康卡片、最近请求、模型过滤、Pelican 展示完整保留；业务调度继续执行实时保护准入。

### 累计保留与排除边界

主上游累计的 Gemini 显式缓存/存储计费和预留、Vertex 身份与模型目录、Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、模型级错误处理、支付主动确认、Stripe 本地订单快照及退款待定扣款保持；Gemini 负输入 token 修复、三协议客户端断连按已收集用量计费和异步单图下载保护保持。

第二上游此前未发布的账号平台统计、模型替换/同步、质量暂停与恢复探测、每日时段/时区、顺序检查、失败退避、测试用量、调用错误页、安全依赖、QQ 活动/绑定/邀请/转账/群共享上下文及 GPT 直充均保留。本项目 Codex 采票/身份保护、普通账号代理池、429 策略、DeepSeek Pro 独立价卡、纯缓存存储不按次计价、转账绑定与双方邮箱行锁、到期复核、失败回滚及直充 POST 禁止自动重放继续生效。

本轮明确复核并继续排除的共享账号池专属路径：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

混合 DTO `backend/internal/handler/dto/types.go` 不恢复 `SharedTotalCallCount` / `SharedListingStatus`。普通账号池、代理池、QQ 群上下文和活动抽奖均不属于排除功能。

累计窗口明确复核并继续排除的批量生图专属路径：
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`

混合文件 `backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen.go`、`backend/cmd/server/wire_gen_test.go`、`backend/internal/{handler,service,repository}/wire.go` 不恢复批量依赖。`frontend/src/router/index.ts`、`frontend/src/components/layout/AppSidebar.vue` 和 `backend/internal/server/routes/gateway.go` 不恢复批量入口；普通生成/编辑与异步单图提交/查询继续保留。分组权限/计费配置、环境配置及 ORM 的排除由机器规则复查。

发布树中的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go` 共 323 个文件逐字节一致，包括批量生图迁移 159–169、187、234 及校验和兼容记录。不删除历史表、记录、冻结余额。本轮新增 279–285 的直充奖励、QQ 奖励、定时测试成员撤销/排除、价格通知、输出 token 和自主聊天迁移全部保留。

剩余具体边界：EasyPay 的标准回调字段白名单仍拒绝额外自定义字段，需标准回调或主动查询/对账恢复；图片下载经过环境代理时，本地 DNS 预检无法保证代理实际目的地址，代理须自行限制私网访问。新增宿主机更新器仅恢复镜像，不备份或回退已执行的数据库迁移；上线前需数据库备份及新旧版本模式兼容确认，不能把镜像恢复当作数据库恢复。这些是实际代码/兼容边界，工具链与验证环境限制不计入产品风险。

### 本阶段验证

- 前端完整测试 415 个文件、3363 项测试通过；类型检查、ESLint、i18n 5 项检查及生产构建通过。
- 后端服务 658 个测试函数、1761 项测试及子用例全部通过，包含更新服务新增边界回归，以及累计 Gemini 缓存/断连/图片下载、定时测试、渠道监控、支付配置和直充传输保护。QQ 奖励/自主聊天/价格通知、仓储、管理接口、清理函数和 EasyPay 回调/金额安全专项通过；宿主机更新器和清单策略完整单元测试通过。
- 后端生产构建通过；仓储、QQ、迁移的 integration 标签编译通过，不代表已运行真实数据库集成测试。
- 同步自动化 101 项（99 项通过、2 项条件跳过），发布工具 13 项、工作流 actionlint 和部署脚本语法检查通过。
- 本地后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，五项缺失 golang.org/x 依赖仅映射到本机缓存；仓库声明的 Go 1.27.2 与依赖版本未改动。较宽测试遇到禁止监听端口后，仅在临时测试选择中排除对应场景，未改写或禁用仓库测试。完整后端正式工具链测试、真实数据库集成与在线漏洞检查仍由外层验证，本阶段不冒充这些检查已通过。

- 静态检查发现的 4 项上游遗留问题已集中修复：QQ 价格通知和最近请求测试显式检查类型断言，测试数据库关闭错误显式处理，奖励码字符判断改为等价的直接区间判断；同时补齐新增仓储/兑换结果集关闭错误的显式处理。相关仓储、管理处理器和 QQ 单元回归已复跑通过。`go vet`（unit 标签）和最终生产构建通过。
- 最终功能排除、源码真实冲突标记、差异空白、323 个历史迁移/校验和文件不变、真实冲突索引不变和指定决策 schema 检查通过；工作树文本/语义冲突为空，保留真实索引未合并条目供外层复核。

- 上述修复后再次运行覆盖更新策略、宿主机更新器、QQ、仓储、服务、处理器和命令包的 golangci-lint（含 gosec），结果为 `0 issues`；服务 658 个函数/1761 项测试及子用例和生产构建再次通过。


## 2026-10-20 最终双上游兼容审查

本阶段进入时，外层已生成候选 `ac29d4ffe`，工作树干净、真实索引没有未合并条目。项目基线为 `origin/main=043894559`，候选已包含 `upstream/main=8f2cbb53d` 与 `overdraft/sub2api-custom=5d05f665e`。上节的 8 个冲突文件及 stage 记录属于此前第二上游合并阶段，本阶段没有清除或改写真实索引。仅修改临时工作树，不 fetch、暂存、提交、推送、打标签、发布或重启服务。

累计审查仍从已发布的 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）计算，覆盖 116 个去重非合并提交：主上游 66 个、第二上游独有 50 个；起始候选相对发布树共 631 个文件变化。已读取双方提交记录，并检查累计提交涉及路径在候选中的保留情况，没有发现排除功能以外的上游文件丢失。第二上游独有提交涉及 386 个路径；没有把此次审查缩减为本轮新增 7 个提交。

重新计算本轮第二上游 182 个路径的三方结果：159 个与自动融合结果逐字节一致；8 个原始文本冲突文件和 15 个有意适配文件逐项复核。发布工作流、日期版本、更新清单仓库身份、宿主机更新器、定时测试行锁/账号排除、代理池/TLS、直充奖励清理及原有冷却恢复测试均保留双方有效行为。

### 本阶段修复

- `backend/internal/service/update_service.go`：补回第二上游的可选缓存保护。此前手工融合遗漏 `getFromCache` / `saveToCache` 的 nil 检查，无缓存运行时检查更新会 panic。新增 `TestUpdateServiceWithoutCache` 先复现空指针，再覆盖普通/强制检查以及发布服务可用/不可用的四种组合；失败时保留警告和双上游信息，不把缓存变成强制依赖。
- 同文件将主/第二上游比较基准更新到候选实际包含的两个提交，并通过既有基准签名淘汰旧缓存，避免把已经合并的更新再次提示为待合并。托管更新器注释同步实际行为：源码与发布镜像均可由宿主机管理；已配置但不可用时仍禁止回退到容器内二进制替换。

### 累计保留、排除和数据库边界

主上游累计的 Gemini 显式缓存、存储计费与预留、Vertex 认证/目录、Argon/Haiku、思考摘要、最小输出 token、Responses 时间戳、Web Search、工作区隔离、模型级错误处理、支付主动确认、Stripe 订单快照与退款保护、负输入 token 修复、三协议断连计费和安全图片转存均保留。

第二上游的模型同步、账号统计、质量保护/恢复、每日时段、顺序检测、失败退避、测试用量、调用错误页、安全依赖、QQ 活动/绑定/邀请/抽奖/转账/群上下文、GPT 直充，以及本轮不可变镜像更新、自主群聊、价格通知、奖励兑换与直充奖励、定时测试账号排除/输出上限/移组撤销、渠道健康卡片/最近请求/Pelican 均保留。本项目 Codex 采票与身份保护、普通代理池、429 策略、独立价卡、缓存存储计费和转账授权/事务保护继续生效。

重新逐项确认上节列出的 65 个批量生图专属路径不存在，9 个混合用途路径中的批量依赖、路由与导航未恢复；完整路径沿用上节清单。共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql` 与 `backend/migrations/retire_shared_account_pool_integration_test.go` 继续明确排除，混合 DTO `backend/internal/handler/dto/types.go` 不含 `SharedTotalCallCount` / `SharedListingStatus`。普通账号池、代理池、普通生图、编辑与异步单图提交/查询继续保留，QQ 群上下文不属于共享账号池。

发布树中的 322 份历史 SQL 和 `backend/internal/repository/migrations_runner.go` 共 323 个文件逐字节不变；包含历史批量生图迁移及其校验和兼容记录。累计新增迁移及本轮 279–285 全部保留；不操作历史表、记录或冻结余额。

剩余具体边界仍为：EasyPay 拒绝非标准附加回调字段，需要标准回调或主动查询/对账；图片下载经过环境代理时，本地 DNS 预检无法保证代理最终目的地址，代理需限制私网目的地；宿主机更新失败仅恢复镜像，不备份或回退已执行的数据库迁移，部署前仍需数据库备份和模式兼容确认。已修复的无缓存 panic 和旧基准误报不列为剩余风险。


### 本阶段验证

- 后端累计服务专项重新无缓存运行：659 个测试函数、1766 项测试及子用例通过，覆盖新增四种无缓存更新场景，以及 Gemini 缓存/断连/图片下载、定时测试、支付与直充等累计边界。更新清单策略、宿主机更新器及其命令包测试通过。
- 前端针对版本菜单、应用状态与保护编辑器重新运行 3 个文件、57 项测试，全部通过。前阶段同一前端树的完整 415 个文件/3363 项测试、类型检查、ESLint、i18n 与生产构建日志已核对；本阶段没有修改前端代码。
- 同步自动化 101 项（99 项通过、2 项条件跳过）、发布工具 13 项、actionlint 及安装脚本语法检查通过。
- 后端沿用缓存 Go 1.27.1 与 `/tmp` 隔离 modfile，仓库 Go 1.27.2 及依赖声明不变；上述验证不代表正式工具链的全量测试、真实数据库集成或在线漏洞扫描已通过，外层继续执行完整验证。

- 本阶段后端生产构建与服务包 unit 标签 go vet 通过；源码冲突标记、累计差异空白、功能排除、323 个历史迁移/校验和文件不变、真实索引不变及指定决策 schema 校验通过。
- 最终服务包 golangci-lint（含 gosec）通过，结果为 `0 issues`。所有本阶段修复已纳入上述专项重跑、生产构建和静态检查。

## 2026-10-21 主上游集中冲突修复

本阶段在 `origin/main=ad0b087c1` 上处理已开始的 `upstream/main=7da09cab4` 合并，合并共同祖先为 `8f2cbb53d`。真实索引的 13 个冲突文件及 stage 1/2/3 保留，交由外层检查和暂存；本阶段只改工作树，没有 fetch、暂存、提交、推送、打标签、发布或重启服务。

累计审查从已发布 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）计算：候选包含主上游 83 个非合并提交及第二上游独有的 50 个非合并提交，共 133 个。本次主上游增量为 17 个非合并提交、85 个路径；逐文件重算三方结果，除冲突融合和明确的兼容适配外均保留自动合并结果。已读取第二上游 `5d05f665e..a08b78173` 的 9 个新增提交及其 131 个变更路径，涉及 QQ 成长/周期奖励、排行领取、指令排版、健康通知和定时测试；它们尚未进入当前主上游合并阶段，仍由外层第二上游阶段完整合并，不能将本阶段 resolved 理解为已经合并这些提交。

### 冲突和语义修复

- 三份 CI/发布工作流继续从 `backend/go.mod` 精确核验 Go 补丁版本，自动跟随本次 `1.27.3`；保留本项目日期版本、发布清单及镜像发布逻辑。四处 Docker 构建镜像同步到 `1.27.3-alpine`，避免镜像工具链低于模块声明。
- `account.go` 保留本项目的 429 重试校验和 `fmt`，融合主上游零分配模型映射签名、只读调度快照和 Vertex `service_account` 配额；普通账号池的类型边界不扩大。调度快照保留质量保护、Codex、上游计费限流字段，模型限流写时复制继续生效。`account_shared_snapshot_test.go` 是进程内只读快照测试，不是共享账号池，不删除。
- `gateway_forward.go` 将原生 Messages 与 Chat/Responses 兼容路径统一接入流错误分类：保留上游余额耗尽的专门换号/脱敏行为、输出后的部分用量计费和已有 429 标记；兼容流输出前不写错误帧，输出后不重放请求。保留上游断连后读取最终用量、缺少终止事件时报错及 handler 提交部分用量的新行为。
- `gateway_forward_compat_stream.go` 与两个 OpenAI/Anthropic 原生转换文件继续读取后台可热更新的流超时策略；构造计时器时应用实际运行值。关闭普通读间隔限制后，客户端断连仍有读取上限。新增 `gateway_merge_stream_policy_test.go` 覆盖两种兼容协议的输出前/后余额错误、账号状态、用量、客户端脱敏及静态/动态超时组合。
- 创建/编辑账号弹窗保留第二上游质量保护、代理配置和模型同步，加入 Vertex 配额编辑；账号页同时保留平台统计页签、模型同步类型和上游筛选记忆。筛选记忆额外保留第二上游 `quality_paused` 状态，新增三个恢复筛选回归。
- 分组服务与页面保留新的 Gemini/Antigravity 普通生图默认值、Gemini 系入口和映射后模型门禁。新增 `246_enable_gemini_image_generation_groups.sql` 按上游设计把存量 Gemini/Antigravity 分组的普通生图开关置为 true；Composite 不在迁移范围内，之后仍可关闭开关。该迁移不涉及批量生图、历史数据删除或冻结余额。

### 排除与累计保留

本轮从混合文件中明确排除批量生图代码：
- `backend/internal/repository/usage_billing_repo_unit_test.go`：不恢复冻结、扣取、释放批量余额的六项测试，保留新的账号配额 SQL 选择/回退测试。
- `backend/internal/service/admin_group.go`：不恢复 `AllowBatchImageGeneration` 的分组派生配置。
- `backend/internal/service/admin_service_group_test.go`：不恢复批量分组测试，并删除自动合并区残留的批量字段断言；普通生图默认值和关闭权限测试保留。
- `frontend/src/views/admin/GroupsView.vue`：不恢复 `resetDisabledBatchImagePricing` 调用，保留普通生图开关默认值。

累计核对继续排除前节列出的 65 个批量生图专属文件（包括 `frontend/src/i18n/locales/en/batchImage.ts`、`frontend/src/i18n/locales/zh/batchImage.ts`）；路由、导航、依赖注入、配置与 ORM 不恢复批量入口。共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql` 与 `backend/migrations/retire_shared_account_pool_integration_test.go` 继续排除；DTO 不恢复 `SharedTotalCallCount` / `SharedListingStatus`。普通账号池、账号代理池、普通生图、编辑与异步单图任务均保留。

累计窗口的 Gemini 显式缓存/存储计费与预留、Vertex 认证/模型目录、Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、支付主动确认、Stripe 订单快照及退款保护仍保留。第二上游已经进入候选的账号平台统计/模型同步、质量保护与恢复、每日时段/顺序检查/退避/测试用量、定时测试账号排除与输出上限、QQ 绑定/邀请/转账/活动/群上下文/自主聊天/价格通知/奖励及直充、渠道健康卡片/最近请求/Pelican、不可变镜像宿主机更新均保留。本项目的 Codex 采票及身份保护、普通代理池、429 策略、独立价卡、纯缓存存储计费和转账权限/事务边界保持。

发布树中的 322 份历史 SQL 与 `backend/internal/repository/migrations_runner.go` 共 323 个文件逐字节未变，包括退役批量生图迁移及其校验和兼容记录。没有删除历史表、记录或冻结余额。

仍存在的具体产品边界：EasyPay 拒绝带非标准附加字段的回调，需标准回调或主动查询/对账恢复；图片下载经环境代理时，本地 DNS 预检不能约束代理实际目的地址，代理必须限制私网访问；宿主机更新失败只恢复镜像，不回退已执行的数据库迁移，部署前需备份并确认模式兼容。此处记录的是代码与数据库边界，不把工具链或沙箱限制视为合并风险。

### 本阶段验证

- 后端服务 865 个测试函数、2262 项测试及子用例全部通过，包含新增兼容流计费、脱敏、动态超时回归及累计缓存/支付/定时测试/更新服务专项。账号配额仓储、网关处理器、DTO 和迁移专项通过；生产构建与相关包 unit 标签 go vet 通过。仓储、QQ、迁移的 integration 标签编译通过，不代表运行了真实数据库集成。
- 前端类型检查、完整 ESLint、修改后的账号筛选专项（12 项）、i18n（5 项）与生产构建通过。初次全量测试的 3367 项断言通过，但共享依赖目录的结果缓存不可写；改用本工作树可写缓存后，高并发重跑暴露资源拥挤导致的超时，随后以两个 worker 重跑完整套件。没有删减测试、修改断言或提高超时阈值。
- 同步自动化 101 项（99 项通过、2 项条件跳过）、发布工具 13 项与工作流 actionlint 通过。真实索引未变、源码真实冲突标记为空、差异空白和功能排除检查通过，69 个批量专属/混合排除路径与 2 个共享账号池专属路径已核对；其中 65 个批量专属路径不存在，4 个混合文件只移除对应退役代码。
- 本地后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，五项缺失 golang.org/x 依赖映射到已有本机缓存，仓库的 Go 1.27.3 与依赖声明保持不变。较宽服务测试遇到八个 WebSocket 监听场景受限，仅从临时执行选择中排除这些场景后重跑，仓库测试未删除或禁用；正式工具链全量测试、真实数据库集成和在线漏洞扫描仍由外层完成。
- 前端最终低并发完整重跑正常退出：415 个文件、3370 项测试全部通过（包含新增的三项筛选恢复回归）；最终类型检查和修改文件 ESLint 也通过。上述高并发超时未在完整重跑中复现。
- 后端 golangci-lint 未计为通过：本机工具使用隔离模块镜像加载相关服务/仓储/处理器后长时间没有返回诊断，限制两个并发并设置三分钟超时的重试仍未完成，已停止两次进程。已完成的生产构建、go vet 与上述回归结果不受影响；外层须补跑正式 lint/gosec 与在线安全检查。这是验证范围限制，不是已发现的代码风险，也不改变文本/语义冲突已解决的结论。

## 2026-10-21 第二上游集中冲突修复

本阶段处理临时工作树中已开始的 `overdraft/sub2api-custom=a08b78173` 合并，起始 `HEAD=7b441ae08` 已包含 `upstream/main=7da09cab4`，项目基线为 `origin/main=ad0b087c1`。第二上游共同祖先为 `5d05f665e`，增量为 9 个非合并提交、131 个路径。本阶段遵照任务只改工作树，不抓取仓库、不暂存、不提交或推送、不打标签或发布、不重启服务；真实索引的 8 个冲突文件及 stage 1/2/3 保留供外层复核。这些索引条目不表示工作树仍有未处理的文本冲突。

累计审查仍以实际已发布 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）为起点，覆盖主上游 83 个及第二上游独有 59 个非合并提交，共 142 个；不能只记录本轮 9 个提交。逐路径重算第二上游三方结果：120 个路径与自动融合结果逐字节一致；8 个冲突文件逐段核对；另 3 个路径是内容检测回归测试及两份依赖配置的必要适配。另在累计代码中修复后台探测请求头初始化，并更新已合入上游的比较基准。

### 冲突与语义处理

- `.github/workflows/backend-ci.yml`、`release.yml`、`security-scan.yml` 保留从 `backend/go.mod` 精确核对 Go 补丁版本的实现，兼容上游 `1.27.3`，保留本项目发布清单、日期版本和镜像策略。服务 `backend/cmd/server/VERSION` 保持已发布的 `2026.10.13`，第二上游标识 `FORK_VERSION` 为 `v0.2.17-custom.25`；本阶段未发布新版本。
- `backend/cmd/server/wire_gen.go` 将渠道缓存、渠道服务、计费服务和价格解析器只初始化一次，满足新增 QQ 模型报价依赖，同时保留本项目 TLS 配置参数。消除自动合并留下的重复局部变量声明；不恢复批量 worker/清理依赖。
- `backend/internal/qqbot/reward_codes.go` 按第二上游最终设计迁移到自动成长、周期和排行奖励，保留历史奖励查询、停用及履约记录。旧兑换命令在持久化前清除正文、附件、引用等所有口令副本；不把已删除的生成/hash 函数碎片插入新的脱敏函数。旧兑换入口和创建 API 的撤除是上游奖励流程演进，既有奖励表和历史记录仍保留。
- `backend/internal/qqbot/group_messages_integration_test.go` 保留本项目入队诊断、未绑定 QQ 身份的邀请查询及邀请参与/充值门槛断言，适配新排版，同时增加成长任务和未绑定余额提示的上游覆盖。普通 `/生图` 与双事件去重继续保留；第二上游明确退役的 `/画图` 等旧别名不复活。
- `backend/internal/service/account_test_service.go` 先应用定时 HTML/SVG 内容要求，再生成本项目账号身份保护快照，避免内容被误判为完整性篡改。兼容重试保持正文、推理强度及账号身份一致，关闭保护时不注入身份元数据。
- 新增回归实际复现并修复 `backend/internal/service/intelligent_test_protection.go` 的空请求头 panic：后台检测构造的 `http.Request` 没有 Header，现只在启用保护且需写入身份时初始化。测试覆盖保护开启/关闭以及不支持默认输出上限的重试，保持普通检测和显式输出上限规则。
- `frontend/package.json` 与 `frontend/pnpm-lock.yaml` 去除自动合并产生的重复 `follow-redirects` override，继续保留 `>=1.16.1` 安全下限及其他双方依赖升级。
- `backend/internal/service/update_service.go` 的主/第二上游比较基准分别更新到 `7da09cab4` / `a08b78173`，使用已有基准签名使旧缓存失效，避免把本次已融合的内容再次提示为待合并。

### 累计保留、排除与数据库兼容

主上游累计的 Gemini 显式缓存、存储计费及预留、Vertex 认证/模型目录/配额、Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、支付主动确认、Stripe 订单快照与退款保护、流式断连后用量回收及计费、调度快照与模型限流写时复制、普通生图权限、账号筛选记忆等全部保留。

第二上游累计的账号平台统计/模型同步、质量保护与恢复、每日时段、顺序检测/退避/测试用量、账号排除与移组撤销、输出上限、QQ 绑定/邀请/转账/活动/群上下文/自主聊天/价格通知、GPT 直充、渠道健康卡片和不可变镜像宿主机更新均保留。本轮新增自动成长与周期奖励、每日排行及历史待领补发、健康异常/恢复通知、实际计费器驱动的模型报价、聊天输出包装清理、交互确认与指令分页、跨协议 HTML/SVG 定时检测全部合入。手动禁用账号不产生排队/跳过结果，质量暂停账号仍参与恢复检测。本项目 Codex 采票与身份保护、普通代理池、429 策略、独立价卡、缓存存储计费、转账授权及事务不变量继续生效。

本轮逐项重新确认此前列出的 65 个批量生图专属路径不存在（包含 24 个 ORM/schema 路径、handler/repository/service 专属实现和测试、前端 API/composable/文案/页面及文档）；混合文件 `backend/cmd/server/wire_gen.go` 明确不恢复批量依赖。路由、导航、配置、计费、分组权限和其余 ORM 中未检出批量运行时残留。共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go` 继续排除；混合 DTO 不含共享池展示字段。普通账号池、账号代理池、只读账号快照、QQ 群上下文均不属于共享账号池，保留。普通生图、图片编辑和异步单图任务继续支持。

发布基准中的 322 份历史 SQL 和 `backend/internal/repository/migrations_runner.go` 共 323 个文件逐字节未变，包含历史批量迁移及校验和兼容条目。新增 286–290 的健康通知、自动奖励、排行规则/领取记录和周期进度迁移完整保留；未删除历史表、记录或冻结余额。成长/周期/排行奖励使用用户行锁、唯一来源键、不可变奖励快照和提交后缓存失效，保持并发幂等及事务原子性。

累计窗口仍保留三个具体边界：EasyPay 签名白名单拒绝非标准附加回调字段，需要标准回调或主动查询/对账；图片下载通过环境代理时，本地 DNS 预检无法约束代理最终目的地址，代理需限制私网访问；宿主机更新失败只恢复镜像，不备份或回退已执行的数据库迁移，需要数据库备份和新旧模式兼容确认。已修复的后台探测 panic、重复注入和重复配置不属于剩余风险。

### 本阶段验证

- 后端最终服务专项：913 个测试函数、2410 项测试及子用例通过，包含保护开关、HTML 内容检测、兼容重试、累计 Gemini 缓存/用量计费、支付、定时测试和更新服务边界。QQ 无本地监听依赖的 58 个函数、197 项测试及子用例通过；本轮仓储与管理接口、EasyPay 签名/回调/金额、宿主机更新器及清单策略回归通过。
- 前端完整套件 416 个文件、3404 项测试通过；类型检查、完整 ESLint、i18n 5 项和生产构建通过。同步自动化 101 项（99 项通过、2 项条件跳过）、发布工具 13 项、actionlint 与同步脚本语法检查通过。
- 最终后端生产构建及相关服务/QQ/处理器/仓储/命令包的 unit 标签 `go vet` 通过；QQ、仓储和迁移的 integration 标签编译通过，不代表已执行真实数据库集成测试。
- 后端沿用缓存 Go 1.27.1 与 `/tmp` 隔离 modfile，缺失的五项 golang.org/x 依赖仅在隔离文件映射到本机缓存；仓库的 Go 1.27.3 与依赖声明不变。宽范围 QQ/支付运行遇到禁止监听端口后，对应可运行专项已重跑；没有修改或禁用仓库测试。正式工具链全量测试、真实数据库集成和在线漏洞扫描交由外层验证。
- 后端全范围 golangci-lint 在三分钟限定内未完成包加载，未计为通过；已完成的 go vet、构建和回归结果不受影响。这是验证范围限制，不列入产品风险。
- 缩小至本轮 QQ 包后的 golangci-lint（含 gosec）完成并返回 `0 issues`；全范围 lint 的未完成状态不改写为通过。最终真实冲突标记、累计差异空白、功能排除、323 个历史兼容文件、真实索引和指定完整决策 schema 均通过检查。

## 2026-10-21 最终双上游兼容审查

最终审查开始时，候选为 `58f4b8e3b`，已包含主上游合并 `7b441ae08` 和第二上游合并；`upstream/main=7da09cab4`、`overdraft/sub2api-custom=a08b78173` 均已进入候选祖先。`origin/main=ad0b087c1`，发布累计基准仍为 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）。本次不是仅审查第二上游的 9 个新增提交：累计窗口覆盖主上游 83 个、第二上游独有 59 个非合并提交，候选提交相对发布基准为 741 个文件、68062 行新增及 3438 行删除。

外层已完成前两阶段的暂存和合并；本阶段开始时工作树干净、真实索引没有 unmerged 条目。此状态不同于前两节修复时保留的 stage 1/2/3；本阶段不改写真实索引，通过合并提交的父树及隔离到 `/tmp` 的重建结果复核双方实现。历史兼容性重新核对：发布树中的 322 份 SQL 及 `migrations_runner.go` 共 323 个文件逐字节相同，功能排除检查通过。未执行 fetch、git add、commit、push、打标签、发布或服务重启。

### 最终融合结果

- 在隔离对象目录重建两次合并并逐段核对原冲突，确认 13 个主上游冲突文件和 8 个第二上游冲突文件的处理均保留双方有效行为。主上游此次更新的文件在第二次合并后仅有前端依赖清单和锁文件继续变化，变化是合入 `follow-redirects` 的安全版本，不回退主上游代码。没有以单侧版本覆盖整个冲突文件。
- 主上游累计保留：Gemini 显式缓存、存储计费及额度预留，Vertex 认证/目录/配额，Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离，支付主动确认、订单快照和退款保护，客户端断连后的用量回收、兼容流异常终止、模型限流写时复制与调度快照、普通生图门禁及筛选记忆。上游依赖安全更新继续保留。
- 第二上游累计保留：平台统计和模型同步、质量保护/恢复、每日时段/顺序检测/退避/用量、账号排除/移组撤销/输出上限，QQ 绑定/邀请/转账/活动/群上下文/自主聊天/报价/健康通知、自动成长和周期奖励、每日排行及历史待领补发、指令分页与确认、跨协议 HTML/SVG 检测、GPT 直充、渠道健康卡片和不可变镜像宿主机更新。旧奖励码创建/兑换入口退出遵循第二上游最终流程，历史奖励和履约记录保留。
- 本项目融合点再次确认：`wire_gen.go` 的渠道/计费/报价依赖只初始化一次且保留 TLS 参数；`account_test_service.go` 先添加内容要求再取身份保护快照，后台 nil Header 已初始化；流式余额错误保留脱敏、输出前换号、输出后部分计费及动态超时；转账复核当前绑定、账户状态和邮箱，按稳定顺序锁定双方，最终过期检查失败回滚全部余额修改。自动奖励通过用户行锁、唯一来源键、不可变快照和提交后失效缓存保持幂等。定时检测保留质量暂停恢复，不复活手动禁用账号。
- `269_retire_codex_ticket_harvesting.sql` 在本项目仅清理旧账号专属代理配置，不清理仍使用的票据、cookie、启用开关和全局采票代理；普通账号代理池、Codex 采票/身份保护、429 策略、独立价卡和缓存存储计费继续保留。该迁移不在已发布的 322 份历史 SQL 中。
- 发布累计窗口还保留更新器打包、镜像 digest/提交绑定的更新清单、日期版本校验、前端锁定安装及 QQ 集成数据库接入。版本为 `2026.10.13`，上游标识为 `v0.2.17-custom.25`，本次没有发布新版本。

### 最终排除核对

此前逐项列出的 65 个批量生图专属路径全部不存在。再次明确排除五个混合文件中的退役代码：`backend/cmd/server/wire_gen.go` 的批量 worker/清理依赖，`backend/internal/repository/usage_billing_repo_unit_test.go` 的批量余额冻结/扣取/释放测试，`backend/internal/service/admin_group.go` 的批量开关派生，`backend/internal/service/admin_service_group_test.go` 的批量字段断言，`frontend/src/views/admin/GroupsView.vue` 的批量价格重置。其余普通功能和测试保留。

共享账号池专属的 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go` 继续排除；DTO 中不恢复 `SharedTotalCallCount` / `SharedListingStatus`。普通账号池、代理池、`account_shared_snapshot_test.go` 的只读快照和 QQ 群上下文不是共享账号池，没有误删。普通生图、编辑及异步单图任务继续保留。对两上游存活路径的核对未发现排除范围以外的缺失文件。

### 最终验证和剩余风险

- 本阶段重新无缓存执行：服务层 913 个函数、2410 项测试及子用例，QQ 58 个函数、197 项测试及子用例，前端 7 个文件/81 项兼容与 i18n 回归，全部通过。仓储配额/计费、兼容流 handler、普通生图权限、QQ 管理权限与请求验证、DTO、EasyPay 签名/回调/有限金额、宿主机更新器及清单策略专项通过。
- 同步自动化 101 项（99 项通过、2 项条件跳过）、发布工具 13 项再次通过。QQ/仓储/迁移的 integration 标签编译通过，不冒充真实数据库集成。前阶段同一前后端源码的完整前端 416 个文件/3404 项测试、类型检查、ESLint、生产构建及后端构建/go vet/QQ lint 日志已复核。
- 宽范围仓储/handler 检查遇到 miniredis/WebSocket 的监听限制；随后只在临时执行选择中限定到可运行的上述边界回归并全部通过，仓库测试未删除或禁用。沿用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，仓库正式 Go 1.27.3 与依赖声明未改动；正式工具链全量测试、真实数据库集成、全范围 lint 和在线漏洞检查由外层执行。这些验证限制不列为产品风险。
- 本阶段未发现需要追加修复的代码冲突，实际工作树改动仅为本节审查记录。源码真实冲突标记为空；两类功能排除、历史兼容文件、累计差异空白、真实索引完整性及完整决策 schema 均作最终检查。

合并后仍存在三项具体边界：
1. `backend/internal/payment/provider/easypay.go` 的回调字段白名单拒绝带非标准附加字段的支付回调；此类上游需要标准回调或主动查询/对账恢复到账，不能放宽白名单重新暴露签名拼接重放问题。
2. `backend/internal/service/image_storage.go` 经环境代理下载图片时，本地 DNS 预检不能约束代理实际解析和连接的目的地址；代理须拒绝私网目的地，否则保留代理侧 SSRF 风险。
3. `backend/internal/sourceupdater/updater.go` 更新失败只恢复镜像，不备份或回退已经执行的数据库迁移；更新前需要数据库备份并确认新旧模式兼容，镜像恢复不等于数据库恢复。


## 2026-10-22 第二上游集中冲突修复

本阶段在临时工作树处理 `overdraft/sub2api-custom=5f988ae20d30053093b512f1d2870e0bb7670e87` 的进行中合并。起始 `HEAD=5dff501ed` 已合入 `upstream/main=dba770d56a0ee0ae5bad1b3dc5e11614d323163c`，项目基准为 `origin/main=b43ebea1c`，第二上游共同祖先为 `a08b78173`。遵照本次任务只修改工作树，没有 fetch、git add、commit、push、标签、发布或服务重启；真实索引及八个文件的 stage 1/2/3 保持原样，交由外层使用临时索引检查后暂存。索引中的 UU 不表示工作树仍存在文本冲突。

发布累计基准仍为 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）。累计审查覆盖主上游 85 个和第二上游独有 62 个非合并提交，共 147 个，包含之前合入但尚未发布的变化。本轮主上游新增 Grok 4.7 Fast 独立价卡修正与 Go 构建补丁对齐；第二上游新增 custom.26–28 三个提交、165 个路径，涵盖统一渠道质量、被动保护、能力题库、预算/供应商/回放、鹈鹕总耗时、账号分组标签和 QQ 生图/编辑。初次逐文件三方重算：157 个非冲突路径与合并结果逐字节一致，其余八个文件逐段融合。完整回归后另适配其中一个测试文件，生产功能不回退。

### 文本与语义融合

- `backend/cmd/server/VERSION` 保持实际已发布的 `2026.10.13`；`FORK_VERSION` 保留第二上游 `v0.2.17-custom.28`。`backend/internal/service/update_service.go` 更新两个上游比较基准和已有缓存签名，避免把本次已经融合的提交再次列为待合并；此改动不代表发布新版本。
- `backend/cmd/server/wire.go` 和 `wire_gen.go` 同时保留本项目账号健康服务、Codex 采票器清理及新增被动质量服务的启动/停止，管理 handler 注入同时包含账号健康、被动质量和渠道质量。拒绝恢复冲突另一侧的批量 worker/清理依赖；渠道、计费、报价和 TLS 参数保持既有融合结果。
- `backend/internal/repository/account_proxy_concurrency_test.go` 保留两个 Redis 客户端和断言式类型转换，采用可扩展初始化循环；新增质量总并发上限及普通代理池跨实例行为均保留。
- `backend/internal/repository/scheduled_test_actions_repo.go` 将 SQL 新增的延迟/状态两列与 Scan 对齐，保留本项目显式关闭 rows 的错误处理；人工评审不能覆盖耗时阈值失败，旧轮次和恢复状态栅栏继续生效。
- `backend/internal/service/account.go` 同时保留请求局部的质量优先级/并发字段与本项目仍生效的 429 重试语义，不采用第二上游将该兼容字段描述为退役的注释。
- `backend/internal/service/antigravity_gateway_retry.go` 将后台检测发送统一放入质量预算准入后，禁止合并片段造成先发送一次再执行带预算发送；检测不嵌套客户 429 重试，保留账号保护的有效并发和代理路径。客户请求继续经过本项目 `doAccountHTTPUpstream`，保留按账号的同号 429 重试预算、正文重放和最终错误处理。`antigravity_quality_budget_test.go` 增加预算拒绝时零发送、成功/429 各一次发送和一次用量结算、普通客户重试的回归。
- `frontend/src/views/admin/AccountsView.vue` 同时保留平台常量和新增分组标签导入，平台/状态筛选记忆、选择清理、分页与分组切换兼容；没有以任一侧整个文件覆盖另一侧功能。

完整前端回归发现并修复三份测试夹具：`ChannelMonitorView.duplicate.spec.ts` 与 `ChannelMonitorView.checkModeBadge.spec.ts` 为新增质量总览链接提供 RouterLinkStub，保留复制、错误处理和检测模式断言，并断言质量入口目标；`TestManagementView.spec.ts` 补充总耗时翻译并将完成态断言更新为 130.123 秒，仍验证运行中计时与完成后停表。三文件的 42 项专项重跑全部通过。

### 累计功能、权限与数据库

主上游累计的 Gemini 显式缓存及存储计费/预留、Vertex 认证/模型目录/配额、Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、支付主动确认、Stripe 快照及退款保护、断流用量回收、模型限流写时复制与调度快照、普通生图门禁和账号筛选记忆继续保留。

第二上游累计的平台统计/模型同步、主动质量保护/恢复、时段/顺序/退避/用量、排除账号/移组撤销/输出上限、QQ 绑定/邀请/转账/活动/群上下文/自主聊天/报价/健康通知/成长周期奖励/排行及历史补领、GPT 直充、渠道健康卡片和不可变镜像更新继续保留。新增质量管理 API 位于既有管理员认证、审计与合规组内；被动质量不清除人工停止及其他策略限制，代理池共享总上限，预算账本事务预占、幂等结算并按原始日期归属，缺失用量不当作零消耗，历史回放只读。供应商只是管理员定义的质量检测聚合，不是共享账号池。

QQ 新增工具严格限制每轮一个图片操作，图片编辑只能选择本次输入的已验证原图，群/应用隔离与引用有效期保持；图片另行校验功能开关和额度，生成/编辑均为 `n=1`，不属于退役批量生图。处理进度不进入对话历史且不重复消耗发送序号。本项目 Codex 采票/身份保护、普通账号代理池、429 策略、独立价卡、缓存计费与转账授权/事务不变量均保留。

逐字节复核发布基准的 322 份 SQL 和 `backend/internal/repository/migrations_runner.go`，共 323 个历史兼容文件均未改变，包含批量迁移及校验和兼容条目。新增 291–295 迁移完整保留，分别支持被动保护、QQ 图片引用、质量总览、预算/供应商/频率与账号时间窗口索引；没有删除历史表、记录或冻结余额。此前累计的 269 迁移仍只清理旧账号专属采票代理，保留本项目仍使用的票据、cookie、开关和全局采票代理。

### 本轮明确排除位置

本轮核对两个上游全部存活路径：主上游缺失的 65 个路径均为批量生图专属实现，第二上游额外缺失的两个路径均为共享账号池退役实现；未发现其他第二上游功能文件被排除。以下专属文件继续不存在，混合文件只排除所述退役依赖。普通账号池、代理池、只读账号快照、普通生图、异步单图任务和 QQ 群上下文均保留。

共享账号池：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图：
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`
- `backend/cmd/server/wire_gen.go`：只排除 `batchImageCleanupService` / `batchImageWorkerRuntime` 注入与清理依赖。

路由、导航、配置、分组权限、计费和其余 ORM 未恢复批量运行时实现；共享池 DTO 字段未恢复。历史 SQL 的保留不表示退役功能被重新启用。

### 累计窗口仍存在的边界

1. `backend/internal/payment/provider/easypay.go` 的签名字段白名单拒绝非标准附加回调字段；此类支付渠道需要标准回调或主动查询/对账恢复到账。不能为兼容扩展字段重新放宽签名校验。
2. `backend/internal/service/image_storage.go` 使用环境代理下载时，本地 DNS 预检不能约束代理最终连接的目标；若代理允许私网目标，代理侧 SSRF 风险仍存在。
3. `backend/internal/sourceupdater/updater.go` 失败回滚只恢复镜像，不回退已执行的数据库迁移；需要升级前备份和新旧数据库模式兼容。镜像恢复不是数据库恢复。

### 本阶段验证

- 服务层专项 944 个测试函数、2461 项测试及子用例全部通过，包含新增 Antigravity 预算/重试回归、质量保护/能力/预算、累计缓存计费、支付、定时检测和更新服务边界。另行运行本轮主上游 Grok Fast 的三项计费/价卡回归，全部通过。
- QQ 无监听依赖的 63 个函数、202 项测试及子用例通过；仓储定时保护/SQL 扫描、管理端质量接口与最小清理依赖专项通过。支付回调签名/重放防护/有限金额、宿主机更新器和发布清单测试通过。
- 后端生产构建与服务/仓储/管理 handler/QQ/命令包的 unit 标签 go vet 通过。仓储、QQ 和迁移的 integration 标签编译通过，不代表执行了真实数据库集成测试。
- 上游同步自动化 101 项（99 通过、2 条件跳过）、发布工具 13 项和 actionlint 通过。两类功能排除、源码真实冲突标记、累计空白与 323 个历史兼容文件检查通过。
- 后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，缺失的五项 golang.org/x 模块只映射到缓存版本，仓库 Go 1.27.3 与依赖清单未改。代理池/Redis 快照测试在禁止监听端口时不能执行，随后可运行的 SQL/handler 专项重跑通过；仓库没有禁用或删除这些测试。首次并行服务编译被资源限制终止，降低并发后上述完整专项重跑通过。正式工具链、真实数据库集成及在线漏洞检查仍须由外层运行，这些不是产品风险。
- 全范围 golangci-lint 在隔离模块目录的包加载阶段达到三分钟上限，未计为通过；不将验证资源限制记为代码风险。保留外层正式全范围 lint/gosec 与漏洞扫描职责。QQ 包按仓库默认含测试配置重跑 golangci-lint（含 gosec），结果为 `0 issues`；仅生产代码模式的 unused 提示来自测试辅助入口，已确认实际被测试使用，保持该入口。
- 前端冻结锁文件离线安装、类型检查、完整 ESLint、修复后的三份测试文件 ESLint、i18n 5 项及生产构建通过。首次构建与多个后端编译并行时被资源限制终止；降低并发后的生产构建完成，保留既有分块大小警告，未改动依赖或降低构建要求。
- 前端完整套件最初发现三份测试文件的 9 项夹具兼容失败：两份监控测试缺少新增导航的 RouterLink，检测耗时断言仍使用旧毫秒展示。保留原行为断言并补齐路由桩及秒级总耗时后，42 项专项通过；完整重跑最终为 422 个文件、3438 项测试全部通过。
- 最终重新检查功能排除、真实冲突标记、累计差异空白、真实索引及全部 stage 1/2/3 条目，均通过；决策 JSON 按指定完整 schema 校验通过。真实索引的 UU 状态有意保留，供外层临时索引检查及后续暂存。

## 2026-10-22 最终双上游兼容审查

本阶段开始时，外层已经完成两次合并：候选 `fda9fd7ef` 的第一父提交为主上游合并 `5dff501ed`，`upstream/main=dba770d56` 和 `overdraft/sub2api-custom=5f988ae20` 均为候选祖先，项目基线为 `origin/main=b43ebea1c`。工作树起始干净，真实索引没有未合并条目；上一阶段所述的 8 个文件、10 处冲突及 stage 1/2/3 保留状态属于当时记录，不能当作本阶段仍有冲突。本阶段不抓取、不暂存、不提交、不推送、不打标签、不发布、不重启服务，不修改真实 Git 索引。

累计窗口仍从实际发布基准 `v2026.10.13`（`13e903e4210bc9254a1cf6ba1d1f8d73d1e3a166`）开始，覆盖主上游 85 个、第二上游独有 62 个非合并提交，共 147 个。起始候选相对发布树为 817 个文件、78936 行新增、4009 行删除；本轮主上游只有 4 个实际改动路径，第二上游新增 3 个提交、165 个路径，但最终审查同时覆盖此前已经合入且尚未发布的更新。

重新检查两上游的全部存活路径：主上游只有 65 个批量生图专属路径在候选中缺失，第二上游仅另缺少 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go` 两个共享账号池专属路径，没有其他第二上游功能文件被排除。前节的 65 个批量生图专属路径与 `backend/cmd/server/wire_gen.go` 中退役服务注入/清理依赖继续明确排除；混合用途文件的非退役实现保留。功能排除检查通过。

再次逐字节比对发布树中的 322 份 SQL 与 `backend/internal/repository/migrations_runner.go`，323 个历史兼容文件均未改变，包括批量生图的历史迁移及校验和兼容记录；没有删除历史表、记录或冻结余额。新增 291–295 迁移保留，普通账号/代理池、普通生图、异步单图任务继续存在。

### 三方复核与通用安全修复

按两次合并各自的父提交和共同祖先逐文件重算：主上游的 5 个增量路径均与自动融合内容一致，其中 Go 镜像对齐已在本项目基线中，只有 4 个路径形成实际差异。第二上游 165 个增量路径中，156 个与自动融合内容逐字节一致，8 个原冲突文件逐段复核，另一个检测管理测试文件适配秒级耗时。版本、依赖注入、代理池测试、保护状态 SQL 扫描、账号质量字段、Antigravity 检测预算/客户重试、账号筛选与分组标签均保留双侧语义；没有使用整文件 ours/theirs 代替融合。

最终全树核对发现，第二上游早期 `ec34b96d9` 共享池安全提交中的两项通用安全修改未进入候选。它们同时服务普通代理和渠道监控，不属于共享账号池专属实现，本阶段补回：

- `backend/internal/pkg/proxyurl/parse.go` / `parse_test.go`：恢复 `Redact` 及上游三项回归，去除代理用户名、密码、路径、查询和片段；无效输入返回固定占位。额外修复 `Parse` 的 `%v` 仍回显 `url.Error` 原始地址、以及缺失 host 时仍暴露用户名/查询字段的问题，失败时返回不含输入的错误，并覆盖畸形 host、转义、端口和缺失 host。
- `backend/internal/repository/http_upstream.go`：恢复 TLS 指纹请求、建池和 HTTP/2 回退日志的 URL 脱敏；复用、淘汰和创建日志不再输出内含凭据的 `cache_key`。保留真实连接凭据、连接池行为、本项目账号健康观测、Codex 采票不复用连接及配置重试。新增 `http_upstream_proxy_log_test.go` 从真实日志出口覆盖创建、复用、淘汰、请求及回退，并验证连接配置中的凭据没有被改写。
- `backend/internal/service/gemini_oauth_service.go`：恢复 Gemini OAuth 代理日志脱敏，OAuth、授权状态校验与代理连接保持原样。
- `backend/internal/service/channel_monitor_ssrf.go`：恢复通用 `ip.IsMulticast()` 拒绝，覆盖 IPv4、IPv4 映射 IPv6 及非链路本地 IPv6 组播。新增 `channel_monitor_ssrf_test.go` 验证主机预检和实际拨号入口拒绝这些目标，公网单播地址继续允许。

函数级差异核对中的其余缺项为已退役批量生图 ORM 的 `RoutedGroupID` 方法，以及第二上游 `93f9f788a` 明确删除的未使用 Antigravity 提取辅助函数；不是其他功能遗漏。两上游路由差集仅为十个 `/images/batches` 路由；因此本阶段还明确核对并排除 `backend/internal/server/routes/gateway.go` 的批量入口及 `backend/ent/mutation.go` 的批量模型方法，普通图片路由与普通实体方法保留。

### 累计行为与剩余边界

主上游累计的 Gemini 显式缓存/存储计费及额度预留、Vertex 认证/模型/配额、Argon/Haiku、思考摘要、Responses 时间戳、Web Search、工作区隔离、支付主动确认、退款保护、断流计费回收和调度性能修复完整保留。第二上游的账号平台统计/模型同步、主动与被动质量保护、检测时段/顺序/退避/预算/能力题库/供应商聚合/回放、账号排除/移组撤销/输出上限、QQ 绑定/邀请/转账/群上下文/自主聊天/成长奖励/排行补领、GPT 直充、渠道健康卡片、账号分组标签、普通单图生成/编辑和不可变镜像更新继续保留；本项目 Codex 采票/身份隔离、普通代理池、429 重试、独立价卡及转账事务/权限加固均保留。

质量管理入口位于既有管理员认证、审计与合规组内；被动质量限制独立叠加，不清除人工停用或其他策略，普通代理池共享质量总并发上限。后台检测先预占预算再发送，客户请求不消耗检测预算；账本事务和幂等结算保留原始日期，缺失用量保留预占，历史回放只读。QQ 图片工具每轮只允许一次操作且 `n=1`，编辑只能选择本次已验证原图，引用按应用/群隔离且有有效期，图片另做功能开关和额度校验。供应商聚合和群上下文不是共享账号池。

仍存在前节记录的三项具体边界：EasyPay 签名白名单拒绝非标准附加回调字段（需标准回调或主动查询/对账恢复）；环境代理图片下载的 DNS 预检不能约束代理最终目的地址（代理侧仍需限制私网）；宿主机镜像回滚不恢复已执行迁移（需备份及新旧模式兼容）。本阶段已修复的日志凭据泄露和组播缺口不再列为剩余风险，工具链/沙箱/远程工作流状态不列为合并风险。

### 最终验证

- 修复后的代理 URL 解析/脱敏、真实 HTTP 日志出口、组播主机和拨号阻断回归通过；HTTP 连接池完整 `TestHTTPUpstreamSuite` 通过。服务层 948 个测试函数的专项选择集复验通过，包含既有 944 个函数、Grok Fast 三项计费回归和新增组播回归；Gemini OAuth 状态校验继续通过。QQ 专项本阶段也已通过。
- 修改包的 unit 标签 `go vet`、后端构建和集成标签下仓储/QQ/迁移编译检查通过；`proxyurl` 包 golangci-lint（含 gosec）为 `0 issues`。只把集成编译记录为编译，不冒充真实数据库执行。
- 前端本阶段 9 个文件、100 项专项通过，涵盖账号分组/筛选、质量管理与上一阶段三份修复夹具；前端源文件没有改动。前一阶段的完整 422 文件/3438 项测试、类型检查、ESLint、i18n 和生产构建记录继续适用于未改动的前端。
- 同步自动化 101 项（99 通过、2 条件跳过）、发布工具 13 项、工作流 actionlint、功能排除、真实冲突标记及累计差异空白检查通过。邮件测试夹具中的等号标题是测试数据，不是冲突标记。
- 后端复验沿用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile；仓库 Go 1.27.3、依赖清单和全量验证要求未变。正式工具链全量单元/数据库集成、全范围 lint 和在线漏洞检查由外层执行，本阶段没有把这些未执行的检查记为通过。
- 真实索引校验和、两上游与项目分支引用均保持起始状态；没有执行暂存或其他 Git 写操作。当前只有上述安全修复、两份新增回归文件及本节审查记录待外层检查。服务版本仍为 `2026.10.13`，第二上游标识仍为 `v0.2.17-custom.28`，本阶段没有发布新版本。


## 2026-10-23 主上游冲突审查

本阶段只解决外层已启动的主上游合并：项目基线 `origin/main=e13061f1f`，`MERGE_HEAD=upstream/main=5c185c5c5`，共同祖先 `dba770d56`。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）；该基准到合并前 HEAD 只有 `e13061f1f` 版本同步，没有其他已合并未发布的功能更新。本阶段不抓取、不暂存、不提交、不推送、不打标签、不发布、不重启服务，真实索引的四组 stage 1/2/3 留给外层检查。

主上游新增四个非合并提交，覆盖 23 个路径：Codex 出站默认身份改为 `codex_cli_rs`、旧 TUI 配置配对归一化、空内容 SSE 安全重试、请求级过载 Retry-After/Retry-After-Ms 及响应体退避提示、DOMPurify 3.4.17 安全更新、赞助信息及上游版本同步。所有功能变更保留；DOMPurify 和 follow-redirects 安全约束已在本项目基线中，合并后继续保持，锁文件不降级、不重复键。

第二上游已合入的基线为 `5f988ae20`（`v0.2.17-custom.28`），其主动/被动质量保护、检测预算/供应商/回放、普通账号及代理池、分组标签、QQ 绑定/邀请/转账/奖励/排行、普通单图生成编辑、GPT 直充、渠道监控和镜像更新均未被本阶段删除。已阅读最新 `c2acfcd79`、`c2dfc8a0c` 的提交记录及增量，涉及质量管理与缓存统计、QQ 控制/通知、API key 摘要存储与模型级质量控制；它们尚不属于当前主上游合并的父提交，由外层后续第二上游阶段完整合并，不能把后续增量声称为本阶段已完成。新出现的批量图片面板测试等仍须遵循同一排除规则。

### 文本与语义冲突

- `backend/cmd/server/VERSION` 保留本项目 `2026.10.23`；`frontend/package.json` 保留现有依赖、脚本、安全约束，去除冲突块中重复的 follow-redirects 覆写。
- `backend/internal/service/settings_view.go` 保留本项目完整字段，包括网关错误重试、流超时/首输出超时、粘性逃逸、平台开关、Codex 采票和代理设置，同时合入主上游 CLI 身份说明；双方字段逐项核对，没有整文件替换。
- `backend/internal/service/openai_gateway_upstream_errors.go` 同时保留账户计费失败切号与主上游请求级退避。计费、413 和账户访问状态优先于响应中的过载文字，仍返回对应安全客户端消息，不泄露供应商余额，也不重新启用同账号重试。
- 同文件进一步修复自动融合的语义缺口：本项目 OAuth 429 重试窗口不再覆盖上游从毫秒头或响应体读取的过载退避；合并两个最小等待要求，并限制在原截止时间内。不可同账号重试的账户错误不挂上该等待时间或截止时间。
- `backend/internal/service/openai_capacity_shed_test.go` 新增混合错误优先级与 OAuth 退避/截止时间回归，保留全部主上游新增用例及本项目原有测试。默认 CLI 身份变化不影响入站官方 TUI 识别、账号命名空间隔离、管理员版本覆写及关闭强制统一后的回退语义；有实际输出或工具完成后仍禁止重放。

### 排除与兼容核对

按主上游及已合入第二上游的全部存活路径核对，候选缺少的主上游路径恰为前节逐条列出的 65 个批量生图专属路径，第二上游仅额外缺少 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go`；没有其他第二上游功能路径被排除。这些专属路径继续明确排除，批量路由、导航、配置、计费、分组权限与 ORM 未恢复；普通生图及异步单图任务保留。 另逐项复核 `backend/cmd/server/wire_gen.go` 的批量服务注入/清理依赖、`backend/internal/server/routes/gateway.go` 的 `/images/batches` 路由以及 `backend/ent/mutation.go` 的批量实体方法，三处退役代码继续明确排除，其他实现保留。

发布基准中的 354 份历史 SQL 及 `backend/internal/repository/migrations_runner.go` 共 355 份兼容文件逐字节未变，包含全部批量生图历史迁移及校验和条目。没有新增数据库迁移，也没有删除历史表、记录或冻结余额。功能排除检查通过。


### 本阶段验证

- 服务层 284 个测试函数、591 项测试及子用例全部通过，包含新增账户错误优先级和 OAuth 退避/截止时间回归，以及身份、版本同步、普通图片计费、Messages/Alpha Search、流式输出边界与既有 429 策略。OpenAI 包完整测试、handler 重试专项通过。
- 前端冻结锁文件离线安装、类型检查、完整 ESLint、国际化 5 项和生产构建通过；仅保留原有 Browserslist 与分块大小提示，没有改动依赖或降低构建标准。同步验证回归 12 项通过，决策 JSON 已按指定完整 schema 校验。
- 后端验证使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，五项 golang.org/x 模块映射到已有缓存版本；仓库要求的 Go 1.27.3 与依赖清单未改。首次服务测试在监听端口用例处被沙箱阻止，随后重跑不依赖监听的完整专项选择集；12 个需要监听端口的函数保留在仓库，未计为通过。正式工具链、全量单元及真实数据库集成验证由外层完成，不将环境限制记为合并风险。

- 后端构建及 OpenAI 包/服务包的 unit 标签 go vet 通过。最终复核真实索引校验和、全部 stage 1/2/3 和仓库引用均未改变；真实冲突标记为零，累计空白、排除规则及 355 份历史兼容文件检查通过。本阶段未发现未解决的合并风险，决策为 `resolved`，`risks` 为空数组；当前阶段没有发布新版本。


## 2026-10-23 第二上游文本与语义冲突审查

本阶段基于外层已经启动的第二上游合并：`HEAD=78d84c000`、`MERGE_HEAD=overdraft/sub2api-custom=c2dfc8a0c`，项目基线 `origin/main=e13061f1f`；`upstream/main=5c185c5c5` 已包含在 HEAD 中。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）。累计窗口包括上一阶段已合入但尚未发布的主上游四个非合并提交、本轮第二上游两个非合并提交及本项目版本同步；不能只按本轮第二上游统计。本阶段仅修改工作树，保留真实索引的全部 stage 1/2/3，供外层复核。

### 累计变化与保留功能

- 主上游：Codex 默认 CLI 出站身份、旧 TUI 配对归一化、空内容 SSE 安全重试、Retry-After/Retry-After-Ms 与响应体退避、DOMPurify 3.4.17、安全依赖约束及赞助更新继续保留；上一阶段的计费错误优先级及 OAuth 重试截止时间修复未被覆盖。
- 第二上游 `58959f3c4→c2dfc8a0c` 涉及 283 个增量路径。保留质量管理简化、独立可用性/延迟/缓存证据、模型级状态/试探/恢复、缓存新会话分配与健康池兜底、渠道统计与错误详情、按有效输入计算缓存命中率及旧聚合数据未知值语义。
- 保留 API Key 摘要存储、列表掩码、创建/重置时一次性展示、二次验证、限流、幂等响应去密钥、轮换后凭据校验和缓存失效；普通网关仍要求真实密钥。
- 保留 QQ 管理概览、分区配置/CAS、活动预览、群策略、管理员/成员状态、禁言与审核记录、转账回执、排行奖励事务通知、成员隔离上下文和图片意图复核。普通单图生成/编辑与异步单图任务继续支持；图片工具每轮一次且没有恢复批量队列。
- 本项目既有风险路由、采票指纹/身份隔离、网关健康观测、代理池、计费恢复、权限及账号/分组筛选保持；供应商聚合和正常账号/代理池不属于共享账号池。

### 冲突处理

逐段解决 16 个文件（14 个内容冲突、2 个删除/修改冲突）：服务版本保留 `2026.10.23`，第二上游标识保留 `v0.2.17-custom.30`；依赖安全下限不降级。账号页去除重复平台常量导入，检测耗时测试保留秒级语义并对齐文案，KeysView 同时保留两个请求取消及一次性密钥/二次验证清理，其测试同时保留用量和密钥槽。

`transfer.go` 将回执插入与余额、令牌删除放在同一事务，保留当前绑定/邮箱校验、固定锁顺序、精确小数和锁等待后的实时时效检查；回执写入或令牌有效性失败均回滚。新增回执失败回归，修正新集成测试缺少真实绑定和邮箱的夹具。新迁移 `299_qq_bot_transfer_history.sql` 的 `net_amount` 允许零值，与现有允许 100% 手续费及非负到账金额的契约一致，同时保持金额恒等式和禁止负值；新增集成断言覆盖零到账回执及余额守恒。这是未发布的新迁移，未修改任何已发布 SQL。

请求详情集成测试同时保留摘要密钥和本项目清理回调。Anthropic 转发保留统一 SSE 错误/部分用量处理，再合入入口缓存采集；Gemini 在首次写出前记录完成标识，保留写入失败/断流计费，并仅发送一次 message_start。OpenAI 转发及 WebSocket 同时保留本项目诊断、指纹租约、下游缓冲和第二上游最终线路缓存证据；调度文件同时保留风险定向选路和缓存降权后的可用账号兜底。

### 本轮排除位置

- 删除重新出现的 `frontend/src/api/batchImage.ts`、`frontend/src/views/user/BatchImageGuideView.vue` 和新增 `frontend/src/api/__tests__/batchImage.panel.spec.ts`。
- 移除 `backend/internal/server/routes/user.go` 的 `/key-image-batches/:key_id` 路由及专属参数，恢复 `backend/internal/server/router.go` 的普通用户路由注入。
- 删除仅服务该批量面板的 `backend/internal/server/middleware/panel_api_key_auth.go`、`panel_api_key_auth_test.go`；移除 `api_key_auth.go` 中该路径专属的状态码和跳过计费条件，保留普通网关鉴权及普通异步单图查询。
- 从混合用途文件 `backend/internal/service/api_key_secret.go`、`backend/internal/repository/api_key_repo.go` 和 `api_key_secret_test.go` 移除仅用于批量面板的按 ID 认证入口/断言；摘要、轮换、所属用户校验、真实密钥鉴权及通用最小字段查询保留。
- 扩展 `.github/upstream-exclusions.json` 和回归测试，禁止通过 `/key-image-batches` 别名恢复该功能。
- 对两上游完整路径清单复核：主上游缺失的 65 个文件均为既有批量生图专属实现；第二上游额外缺失三个本轮新增的批量专属文件及两个共享账号池专属迁移/测试，无其他功能文件被排除。前文 65 个批量专属路径继续明确排除；`wire_gen.go` 的批量服务清理、`routes/gateway.go` 的批量路由、`ent/mutation.go` 的批量方法亦继续排除，其他实现保留。
- 共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go` 继续排除。没有恢复共享账号池 UI、调度、权限、计费或 ORM，也没有删除历史表或冻结余额。

发布基准中的 354 份 SQL 与 `backend/internal/repository/migrations_runner.go` 共 355 份历史兼容文件逐字节一致；新增 296–304 共九份非退役功能迁移保留。

### 剩余部署兼容边界

`300_api_key_digest_only.sql` 不可逆地将明文凭据转换成摘要，旧二进制不能继续以明文查询；`303_passive_quality_model_states.sql` 改变质量状态/租约的唯一键，旧进程的冲突目标不再兼容。升级必须协调所有应用实例，不能仅回退镜像来恢复旧版本；需要升级前备份和明确的数据回退方案。已有客户端原始密钥在新版本中继续有效，配额、计费与历史用量保留。该边界是本轮实际数据库兼容风险；工具链、监听权限和远程工作流状态不作为产品风险。

### 验证结果

- 前端首次全套发现自动融合删除了 `EditAccountModal.spec.ts` 中本项目采票测试仍使用的 `flushPromises` 导入，导致六项失败；恢复导入后该文件 90 项通过，完整重跑 433 个文件、3511 项全部通过。类型检查、完整 ESLint 和国际化五项检查通过。
- 后端服务层最终 17876 项测试及子用例通过；QQ、仓储、handler、admin handler、server、中间件和路由专项合计 4232 项通过。上述八组共 22108 项通过、53 项按原条件跳过，无代码失败。转账回执失败/到期回滚新增回归通过，摘要存储/轮换/原密钥失效、模型隔离/缓存兜底、SSE/Gemini/重试/采票兼容均包含在验证中。
- 后端构建、受影响包 unit 标签 go vet、全仓 integration 标签编译通过；集成检查使用 `-exec=true` 只验证编译，不声称已运行真实数据库测试。同步自动化 112 项（110 通过、2 条件跳过）通过，含新增批量面板别名排除回归。
- 后端采用缓存 Go 1.27.1 与 `/tmp` 隔离 modfile，五项 golang.org/x 模块映射到已有缓存版本；仓库 Go 1.27.3 与依赖未修改。先尝试全仓单元检查，收集监听/DNS/子进程工具链限制后，再完整运行不依赖这些条件的受影响包测试；这些条件用例仍保留，未计为通过。服务层额外排除一个依赖真实 DNS 的参数矩阵和三个 Unix socket 插件用例后重跑通过。
- 全范围 golangci-lint（含 gosec）在隔离副本的包加载阶段三分钟超时；govulncheck 和 pnpm audit 因无法访问漏洞服务未完成，不计为通过。没有降低安全规则或添加漏洞例外，正式工具链完整验证及在线审计仍由外层负责；这些验证环境限制不列入 risks。

- 前端生产构建及修复后完整 ESLint 最终通过；仅保留既有 Browserslist 和大分块提示。最终复核真实索引、16 组冲突阶段、所有引用、355 份历史兼容文件均保持起始状态；真实冲突标记、累计空白与功能排除检查通过。当前未提交、未推送、未发布。

## 2026-10-23 最终双上游兼容复核

本阶段从外层已生成的候选 `5a0ee9470` 开始；其父级主上游合并为 `78d84c000`，两上游分别为 `5c185c5c5` 和 `c2dfc8a0c`，项目基线为 `origin/main=e13061f1f`。进入本阶段时工作树干净、没有未合并索引项；上一阶段的 16 组冲突已由外层处理，本阶段不重新构造或改写索引。未执行 fetch、git add、commit、push、tag、发布或服务重启。

累计审查从已发布的 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）到整个候选，进入阶段时共 302 个文件、15501 行增加和 1476 行删除，包含主上游四个及第二上游两个非合并提交，以及本项目版本同步和融合修复。对主上游的 23 个增量路径、第二上游的 283 个增量路径逐文件比对；第二上游有 45 个保留了本地差异的文件，新增代码中未原样保留的部分均对应退役批量面板、项目版本、转账验证夹具或零到账兼容修复，没有按功能规模筛除其他更新。

复核密钥摘要与缓存失效、TOTP 二次验证、普通网关权限与计费、转账余额/回执/令牌事务、模型独立试探租约、缓存降权兜底及风险选路、SSE 中断用量和 WebSocket 指纹租约。保留主上游 Codex CLI 身份、空内容流重试和退避、安全依赖更新；保留第二上游质量管理、缓存统计与证据、密钥一次性展示与轮换、QQ 分区配置和活动预览、群管理、奖励事务通知、转账回执、成员上下文及单图意图复核。

### 最终明确排除位置

- 两上游完整文件清单复核：主上游缺失的 65 个文件均为本文件前文列明的批量专属实现；第二上游缺失 68 个批量专属文件和两个共享账号池专属文件，没有其他缺失路径。
- 继续明确排除 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go`；普通账号池、代理池、QQ 群上下文及通用并发池均保留。
- 批量前端 API、页面和专属测试，以及 `backend/internal/server/middleware/panel_api_key_auth.go`、`panel_api_key_auth_test.go` 继续不存在；`routes/user.go` 不注册 `/key-image-batches`，`router.go` 不注入面板密钥鉴权，`api_key_auth.go` 不保留其权限或计费例外。
- `api_key_secret.go`、`api_key_repo.go` 及仓储密钥测试不含批量面板专用的按 ID 认证入口；`wire_gen.go`、`routes/gateway.go`、`ent/mutation.go` 不含批量依赖、路由或 ORM 方法，混合用途文件的其他实现保留。完整专属路径见本文件前文和本阶段决策的两个排除数组。
- 发布基准的 354 份 SQL 和 `migrations_runner.go` 共 355 份文件逐字节一致；新增 296–304 九份迁移保留，未连接数据库或改动历史数据、冻结余额。普通生图与异步单图任务入口及测试保留。

### 剩余数据库兼容风险

`300_api_key_digest_only.sql` 将明文密钥不可逆地转为摘要，旧应用无法继续按明文认证或写入；`303_passive_quality_model_states.sql` 改变状态和租约唯一键，旧应用的冲突目标不再兼容。这两项来自保留的第二上游功能，需要协调所有实例升级，准备升级前备份及数据回退方案，不能仅回退镜像。历史迁移校验兼容和现有客户端原始密钥有效性不受破坏。

### 最终语义补修和验证

- 发现主上游新增的秒/毫秒退避解析在转成 `time.Duration` 后才限幅，超大合法浮点数会溢出并被忽略，造成退避从预期的八秒变成零。`openai_gateway_upstream_errors.go` 的两个解析函数现于转换前限幅；`openai_capacity_shed_test.go` 新增四个请求头/响应体回归，修复前四项均失败，修复后全部通过。已有八秒上限、正常退避、计费错误优先级和 OAuth 截止时间继续保留。
- 前端全套 433 个测试文件、3511 项测试通过；类型检查、完整 ESLint、国际化五项检查和生产构建通过。
- 后端八组可执行用例合计 22112 项测试及子用例通过，53 项按原条件跳过，其中服务层 17880 项通过；转账、密钥、权限、调度和普通/异步单图回归均保留。沿用上一阶段记录的监听、DNS 和插件进程环境排除清单，没有删除或弱化测试。
- 后端构建、受影响包的 unit 标签 go vet、部署安全检查通过；修复后重新运行同步自动化，112 项检查中 110 项通过、两项条件跳过。全量线上漏洞审计和真实数据库集成不计为本阶段已通过，留给外层完整验证。
- 全仓 integration 标签编译最终通过（`go test -tags=integration -run='^$' -exec=/bin/true ./...`）；该检查仅编译，不声称执行了真实数据库集成测试。
- 后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile（沿用五项缓存模块映射），仓库 Go 1.27.3 与依赖清单保持原样。没有修改安全规则或漏洞例外。
- 再次确认 355 份历史兼容文件逐字节不变、真实冲突标记为零、累计空白及排除规则检查通过；真实索引、冲突阶段和仓库引用与进入本阶段时一致。本阶段只留下两个 Go 文件和本审查文档的未暂存修改。

## 2026-10-24 主上游文本与语义冲突审查

本阶段处理外层已启动的主上游合并：`HEAD=origin/main=82080b8e2`，`MERGE_HEAD=upstream/main=37067e8e3`。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）；审查同时覆盖该基准至 HEAD 的全部未发布变化和本轮 92 个主上游增量路径。累计窗口包含五个主上游、两个已合入第二上游的不同非合并提交，以及本项目融合修复。仅修改工作树；没有 fetch、暂存、提交、推送、打标、发布或重启服务，真实索引 stage 1/2/3 留给外层检查。

第二上游已合入点仍为 `c2dfc8a0c`。已阅读其后至当前 `overdraft/sub2api-custom=af7a290ec` 的提交记录和差异，包括分组质量目标/预算/兜底、断连失败证据、QQ 垃圾信息举报与成员选择。这些新增变化属于外层后续第二上游合并阶段，不是本阶段的产品排除，也不宣称已经合入本候选。

### 累计变化与保留功能

- 本轮主上游加入按操作、账号类型、平台和分组授权的管理员 API 密钥，含摘要存储、到期/IP 限制、轮换、二次验证、路由权限目录、资源投影与脱敏、受限账号写入事务、Codex 导入、代理选择、分组模型接口、设置 UI、双语文案及 `247_scoped_admin_api_keys.sql`。
- 累计主上游 Codex CLI 出站身份、旧身份归一化、空流安全重试、秒/毫秒退避及八秒限幅、DOMPurify 安全更新、赞助和版本同步继续保留；此前本项目计费错误优先级、重试截止时间与溢出修复未被覆盖。
- 已合入第二上游的用户 API Key 摘要存储、一次性展示、轮换与缓存失效、模型级质量状态/试探/恢复、缓存证据/命中率/新会话分配及健康池兜底、渠道统计保留。
- 已合入第二上游 QQ 分区配置/CAS、活动预览、群管理与审核、奖励事务通知、转账回执、成员隔离上下文和单图意图复核保留；本项目 GPT Direct、采票/指纹隔离、风险选路、代理池、权限与计费继续保留。

### 文本融合与语义补修

逐段融合八个冲突文件：`.gitignore` 保留全部项目文档例外、插件/缓存/部署忽略规则并加入新文档；`wire_gen.go` 保留本项目带配置的代理仓储及所有原有服务并注入密钥仓储；`account_handler.go` 在上游公共模型响应 helper 中保留 Antigravity 默认目录和显式模型映射语义；`account_repo.go` 同时保留上游事务范围、本项目多代理原子写入、关联保护、退役字段过滤和指纹种子保护；两个服务结构保留全部本地字段并加入新依赖/代理模式；两个语言入口同时保留 GPT Direct、QQ 和管理员密钥文案。最终复查补回 `.gitignore` 遗漏的原有尾部规则。

- 受限账号事务在账号行锁之前调用 `ScopedAdminLockAccount`，沿用 membership gate → plan → account 锁顺序，避免与现有质量保护写入倒置。不存在账号仍返回既有 NotFound。受限分组变更保留未变关联的优先级和创建时间，并在同一事务内清理离组保护、撤销旧测试轮次和写 outbox；不删除历史事件。
- `custom_account_proxy_selection.go` 的 least_used 统计补入 `account_proxies`，按账号/代理去重并排除软删除账号，保留本项目多代理池占用语义。
- 补充仓储/服务锁顺序与回滚、受限分组保护生命周期、模型目录和凭据不泄露回归；真实数据库夹具补齐账号代理表及现有保护迁移，并覆盖离组被动保护清理和历史留存。usage 响应测试与累计密钥摘要/掩码契约一致，完整管理员也不回显明文用户 API Key。设置页独立测试 stub 新组件，其组件测试仍独立运行。
- 完整服务回归发现既有串行化测试复用了生命周期夹具的 40 毫秒超时：繁忙时第一个执行提前超时重试，被误判为另一个定义并发执行。`scheduled_test_execution_serial_test.go` 单独采用与外层测试上下文一致的三秒预算，仍明确阻塞第一个执行并保留全部串行/跨账号并行断言；生产执行超时与重试行为不变。

### 本轮明确排除与数据库兼容

- 对主上游及已合入第二上游的全部路径复核：主上游缺少 65 个批量生图专属文件，第二上游缺少 68 个批量生图专属文件及两个共享账号池专属文件，均属于本文件已记录的排除范围，没有其他已合入第二上游功能路径被排除。完整路径亦记录在本阶段决策的排除数组。
- 明确排除共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go`；普通多分组账号、代理池、通用并发池及文档中的普通“共享账户”均保留。
- 批量生图 UI/API/worker/queue/计费/配置/测试/文档/生成 ORM 继续排除。混合文件 `backend/cmd/server/wire_gen.go`、`backend/internal/server/routes/gateway.go`、`backend/internal/server/routes/user.go`、`backend/internal/server/router.go`、`backend/internal/server/middleware/api_key_auth.go`、`backend/internal/service/api_key_secret.go`、`backend/internal/repository/api_key_repo.go`、`backend/ent/mutation.go` 中的批量依赖、路由、面板鉴权例外和 ORM 方法均未恢复，其余实现保留。
- 发布基准中的 354 份历史 SQL 和 `backend/internal/repository/migrations_runner.go` 共 355 份兼容文件逐字节一致；累计新增 296–304 九份迁移与本轮管理员密钥迁移保留。没有操作历史数据库、表、记录或冻结余额；普通生图和异步单图任务保留。
- 累计窗口仍有两个明确部署兼容风险：`300_api_key_digest_only.sql` 不可逆转换明文凭据，旧二进制无法继续按明文认证/写入；`303_passive_quality_model_states.sql` 改变状态/租约主键及冲突目标，旧进程不兼容。须协调所有实例升级、提前备份并准备数据回退方案，不能只回退镜像。这是保留功能的升级边界，不是未解决的文本或语义冲突。

### 验证记录

- 前端冻结锁文件离线安装、类型检查、完整 ESLint、国际化五项及生产构建通过；组件隔离修复后再次完整运行 434 个文件、3523 项测试，全部通过。仅保留已有 Browserslist 和大分块提示。
- 最终九个受影响后端包的可执行用例共 22344 项测试及子用例通过、70 项按原条件跳过，无失败；其中服务层 17916 项通过。串行化夹具修复后完整重跑通过，并另外连续重复该测试 50 次通过。权限/脱敏、锁顺序/回滚、代理池计数、质量调度、QQ、普通及异步单图相关用例均保留；需要真实数据库的新增用例只计为编译、未计为执行通过。
- 后端构建、受影响包 unit 标签 go vet、全仓 integration 标签编译通过；集成标签使用 `-run='^$' -exec=/bin/true`，只计为编译验证，不计为真实数据库测试。部署安全检查通过；同步自动化 112 项中 110 通过、两项按原条件跳过。
- 后端使用缓存 Go 1.27.1 和 `/tmp` 隔离 modfile，五项 golang.org/x 模块映射到已有缓存版本；仓库 Go 1.27.3 和依赖清单未修改。先收集全量运行的失败，再以 `/tmp` 清单排除确证依赖端口、真实 DNS 和 Unix socket 的用例，仓库测试未删除或弱化，这些用例不计为通过。
- 全仓 golangci-lint（含 gosec）首先因工具包加载不接受隔离 modfile 失败；转入完整隔离源码副本后，包加载长时间占用资源，已中止，未计为通过。重复编译也出现编译进程被终止，收尾改用低并发；未降低静态检查规则。govulncheck 与 pnpm audit 因无法访问漏洞服务未完成，未计为通过。上述环境限制不列入合并风险，正式工具链完整检查及在线审计由外层执行。
- 真实索引、全部冲突阶段和仓库引用保持不变；355 份历史兼容文件逐字节一致，真实冲突标记为零，累计差异空白与功能排除检查通过；决策已按指定 schema 验证。当前阶段没有发布新版本。

## 2026-10-24 第二上游文本与语义冲突审查

本阶段基于 `origin/main=82080b8e2`、已包含主上游 `37067e8e3` 的 `HEAD=43503820a`，处理 `overdraft/sub2api-custom=af7a290ec` 的进行中合并。第二上游合并基点为 `c2dfc8a0c`，本轮增量涉及 223 个路径，65 个路径与本项目相对共同基点均有变化。发布累计基准仍为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）；审查覆盖两上游累计 13 个不同非合并提交及本项目融合修复，不仅限于本轮六个第二上游提交。此前决策文件属于主上游阶段，本节补全后续第二上游审查。

### 上游变化与保留功能

- 主上游累计管理员 API 密钥按操作/资源授权、摘要存储、IP/到期约束、轮换与二次验证、受限事务写入及其 UI/迁移全部保留。Codex CLI 身份归一化、空流安全重试、退避解析、DOMPurify 安全更新继续保留。
- 第二上游本轮保留分组质量目标、跨公开模型共享的质量流量预算、验证状态、评分兜底、健康供给恢复及预算耗尽后的 API Key 分组回退；“共享缓存预算”是普通分组质量管理，不是退役共享账号池，不予排除。
- 保留客户端断连后已确认的上游失败证据、统一上游错误提示（HTTP/SSE/WS 与异步单图）、原始 Ops 错误和健康分类、QQ 谨慎垃圾信息举报/双重审核/撤回与禁言流程、成员选择和隐私隔离；保留新增 305–307 迁移。
- 累计此前已合入的密钥摘要/一次性展示/轮换、模型级被动质量与缓存证据、渠道统计、QQ 奖励/转账通知、群管理与成员上下文均保留；本项目风险选路、利润控制、采票代理脱敏、平台开关、流式超时、多代理池和普通/异步单图不被替换。

### 冲突融合与语义修复

- 逐段处理 14 个文件中的 19 个文本冲突块，并移除三个 modify/delete 冲突留下的退役运行时文件；真实索引的全部 17 组未合并项保持原样，供外层核对 stage 1/2/3。没有执行 fetch、git add、commit、push、打标签、发布或重启服务。
- 设置读取、写入、DTO、审计、前端与契约测试同时保留 `upstream_error_message` 和本项目网关策略、导航权限、采票开关/代理掩码；部分更新不覆盖省略字段。服务版本保持 `2026.10.23`，`FORK_VERSION=v0.2.18-custom.2`。
- 根别名及 Codex 直连中间件保留原有 retry → endpoint normalization 顺序，在 Ops 之前加入错误展示策略；原始错误仍用于重试、计费和日志。普通生图与异步单图端点继续保留，用户批量路由不恢复。
- 调度同时保留风险指定账号/平台和新质量评分兜底。回归复现风险指定账号的质量预算拒绝被旧抢槽路径吞掉并转为等待计划，现明确返回预算错误；不能绕到其他账号。上游退避仅在最终分类确认为请求级容量不足后设置，避免覆盖计费/载荷过大/权限失败优先级；八秒限幅、浮点溢出保护与 OAuth 重试截止时间均保留。
- WS 透传同时保留输出互斥、已提交回合的用量收集、容量重试包装和第二上游错误展示；Ops 使用原始 provider payload，客户端只收到展示副本。HTTP bridge 保留已发送的两帧协议顺序，并采用新上游错误提示。
- 新增回归复现本项目 WS 客户端会话上下文脱离取消后会继续处理质量兜底请求，且可能丢失租约失效原因。现按实时租约状态响应取消，覆盖首轮和后续轮次进入兜底、租约失效及客户端断连；普通请求保留原有有界用量收集。回归不依赖真实网络监听。
- 全量服务回归发现本项目旧 `The service is busy` 客户端文案断言与第二上游统一错误展示冲突；更新断言，继续验证已输出后不切换账号、唯一失败终态、可重试代码及内部错误保留原文，未放宽这些不变量。

### 排除与数据库兼容

本轮实际删除 `backend/internal/handler/batch_image_handler.go`、`backend/internal/service/batch_image_download.go`、`backend/internal/service/batch_image_public.go`，以及自动合入的 `backend/internal/handler/batch_image_error_message_test.go`、`backend/internal/service/batch_image_error_message_test.go`。`backend/internal/server/routes/user.go` 不恢复 `/key-image-batches/:key_id`。其余批量 UI、配置、分组计费、worker/queue、ORM 及面板批量鉴权仍排除；混合用途文件保留其他功能。本轮完整决策记录 78 个明确排除的批量路径（70 个专属文件和八个混合用途文件）。

共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go` 继续明确排除，不执行其中的资源退役 SQL。逐一比对两上游树，候选缺失路径均属于上述两类排除，没有其他第二上游功能路径被删除。

发布基准的 354 份历史 SQL 和 `backend/internal/repository/migrations_runner.go` 共 355 份文件逐字节一致；保留累计新增的 247、296–307 迁移。未连接或修改历史数据库、记录、表及冻结余额。

累计窗口仍有两个数据库部署兼容风险：`300_api_key_digest_only.sql` 将明文密钥不可逆转为摘要，旧二进制不再兼容其认证/写入；`303_passive_quality_model_states.sql` 改变状态与租约主键及冲突目标，旧进程不兼容。须协调实例升级、提前备份并准备数据回退方案，不能仅回退镜像。这些是保留功能的升级边界，不是未解决的合并冲突。

### 验证记录

- 最终完整重跑 14 个累计受影响后端包的可执行用例，23746 项测试及子用例通过、70 项按原条件跳过，无失败；其中服务层 18304 项通过。网络监听、真实 DNS 和插件进程依赖用例按 `/tmp` 清单排除，原仓库测试保留，未计为通过。
- 首轮回归确证风险指定账号预算拒绝被吞掉，以及 WS 兜底上下文取消失效/原因丢失；修复后 31 项针对性回归全部通过，覆盖首轮/后续轮次、租约失效/断连、普通用量收集、容量退避限幅/优先级和客户端展示/内部原文分离。随后再次全量重跑上述 14 个包通过。
- 前端完整 439 个文件、3598 项测试通过；类型检查、完整 ESLint、国际化五项检查及生产构建通过。后端最终修改后再次执行类型检查与 lint，通过。保留既有 Browserslist 和大分块提示。
- 同步自动化 112 项中 110 通过、两项按原条件跳过；部署安全检查通过。自动化测试内使用隔离临时仓库/模拟工作流，不操作工作仓库远端或真实发布。
- Go 使用缓存 1.27.1 和 `/tmp` 隔离 modfile，沿用已验证的五项缓存模块映射；仓库声明的 1.27.3 和依赖清单未修改。真实数据库集成、完整 golangci-lint 及在线漏洞审计不计为本阶段已通过；这些验证边界不列作产品合并风险。
- 最后一次完整重验的后端构建、全仓 unit 标签编译、全仓 integration 标签编译及受影响包 go vet 均通过。`-run='^$' -exec=/bin/true` 的标签检查仅计为编译验证，不计为真实数据库集成执行。
- 最终确认真实索引、17 组冲突阶段和引用均与进入本阶段时一致；355 份历史兼容文件逐字节不变，真实冲突标记为零，累计空白与排除检查通过；决策按指定 schema 校验。当前阶段没有发布新版本。


## 2026-10-24 最终双上游兼容审查

最终审查入口为 `HEAD=2c2621c90`，项目基准为 `origin/main=82080b8e2`，两个上游为 `upstream/main=37067e8e3` 和 `overdraft/sub2api-custom=af7a290ec`；两上游均已是候选的祖先。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`），累计覆盖 13 个不同非合并上游提交及本项目融合修复，包含前一轮已合并但未发布的更新；入口候选涉及 527 个变更路径，加上本次回归补修后为 528 个。本阶段开始时工作树干净、索引无未合并项；前两阶段的冲突已由外层完成合并，本阶段没有操作真实索引、仓库引用或远端。

### 保留范围与最终语义补修

- 保留主上游管理员密钥操作/资源授权、摘要、IP/到期限制、二次验证、事务锁顺序与关联保护，Codex 身份与空流重试、容量退避、DOMPurify 安全更新；保留本项目风险选路、利润/计费保护、采票指纹、代理池、网关策略与原始 Ops 错误。
- 保留第二上游整个累计窗口中的密钥一次性展示/轮换/摘要存储、模型级质量与缓存证据、渠道统计、分组质量目标/预算/评分兜底/健康恢复、预算耗尽后的普通 API Key 分组回退、断连确证失败、统一错误展示，以及 QQ 配置分区、活动预览、群管理、双重垃圾举报审核、成员选择/上下文隔离、奖励与转账通知。共享缓存预算不属于退役共享账号池。
- 最终回归复现风险指定账号/分组请求在主组质量预算耗尽后，被通用调度器切换到 API Key 备用组和另一个账号，同时将计费组与倍率改为备用组；旧调度、非批量负载调度和高级调度的六个用例修复前全部失败。根因是 `openai_account_scheduler.go` 的实际主入口未应用风险目标，而 `api_key_group_fallback.go` 没有区分显式风控路由。
- 现于统一调度入口应用风险分组和平台，指定账号走现有能力、利润、隐私、额度及实时准入校验，明确绕过可更换账号的通用质量恢复；同时禁止显式风险路由进入 API Key 分组回退。指定账号的增量响应必须属于相同凭据，未知或不同持有者拒绝；正常可回放请求的原有调度不变。新增回归覆盖三种调度模式、正常风险选路、质量拒绝、其他账号恢复诱因、原计费组/倍率与响应所有权。普通请求仍保留第二上游新增的预算耗尽分组回退。

### 排除与累计数据库兼容

- 比对两上游完整文件树：主上游缺少 65 个批量专属文件，第二上游缺少 70 个批量专属文件和两个共享账号池专属文件；没有其他上游文件缺失。所有排除均沿用前两阶段决策，本阶段没有再次删除文件。
- 共享账号池继续明确排除 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go`。普通账号池、多代理池、通用并发池及质量预算保持支持。
- 批量专属 UI/API/队列/worker/计费/配置/测试/文档/生成 ORM 继续排除；明确核对八个混合文件 `backend/cmd/server/wire_gen.go`、`backend/internal/server/routes/gateway.go`、`backend/internal/server/routes/user.go`、`backend/internal/server/router.go`、`backend/internal/server/middleware/api_key_auth.go`、`backend/internal/service/api_key_secret.go`、`backend/internal/repository/api_key_repo.go`、`backend/ent/mutation.go`，只排除批量依赖、路由、面板鉴权例外及 ORM 方法，其他功能保留。普通生图和异步单图的路由、鉴权、计费和测试继续保留。
- 发布基准 354 份历史 SQL 与 `migrations_runner.go` 共 355 份兼容文件逐字节一致。累计新增 247、296–307 共 13 份迁移保留，未修改历史表、记录或冻结余额。
- 累计风险仍为两项数据库升级边界：`300_api_key_digest_only.sql` 不可逆转换明文密钥并拒绝旧格式写入；`303_passive_quality_model_states.sql` 更换状态/租约主键及冲突目标，旧进程不兼容。需协调所有实例升级并预备备份及数据回退方案，不能仅回退镜像。这些不是未解决的合并冲突。


### 最终验证记录

- 修复后完整重跑 14 个受影响后端包，23768 项测试及子用例通过、70 项按原条件跳过，零失败；服务层 18326 项通过。新增风险路由回归 22 项测试及子用例全部通过。沿用前阶段已确认的环境依赖排除清单，未删除或弱化仓库用例，排除项不计为通过。
- 前端默认并发首轮出现 13 个失败测试文件及 19 个 worker 错误，均为测试/RPC 超时；保留完整日志后只将并发设为两个 worker，439 个文件、3598 项测试完整重跑全部通过，无未处理错误。类型检查、完整 ESLint、国际化五项检查和生产构建通过，未修改前端代码、断言或超时。
- 后端构建、全仓 unit 标签编译、全仓 integration 标签编译及受影响包 go vet 通过。标签编译使用 `-run='^$' -exec=/bin/true`，不计为真实数据库集成执行；Go 使用缓存 1.27.1 与 `/tmp` 隔离 modfile，仓库声明的 1.27.3 和依赖清单未修改。
- 同步自动化 112 项中 110 通过、两项条件跳过；同步脚本语法、部署安全、网关环境、资源、简易模式和 Caddy 缓存检查通过。macOS 容器脚本检查因缺少 `plutil` 未完成；真实数据库集成、完整 golangci-lint 与在线漏洞审计不计为本阶段通过，正式验证留给外层。这些环境边界不列作产品合并风险。
- 355 份历史兼容文件逐字节一致，源码无真实冲突标记，累计差异空白、功能排除和指定决策 schema 检查通过。真实索引与仓库引用保持进入本阶段时的状态，只留下两个生产文件、两个测试文件和本审查文档的未暂存修改。本阶段未发布新版本。


## 2026-10-25 主上游文本与语义冲突审查

本阶段处理外层已启动的 `upstream/main` 合并：`HEAD=origin/main=09c24e0c3`，`MERGE_HEAD=upstream/main=ba81c7fd7`。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`），累计审查覆盖两个上游的 19 个不同非合并提交、版本同步和本项目融合修复，包含此前合并但尚未发布的全部变化。本轮主上游增量有六个非合并提交、50 个路径。仅修改工作树；未对工作仓库 fetch、git add、commit、push、打标、发布或重启服务。真实索引中的 25 组冲突阶段保留给外层核对。

已合入第二上游的最新提交为 `af7a290ec`。同时阅读其后至 `overdraft/sub2api-custom=8c54f4005` 的两个提交及差异：QQ 排行配置版本通过专用响应头穿过压缩，以及隔离 vm2api 账号路由、桥接管理、代理、WebSocket、配套迁移与管理界面。它们属于外层后续第二上游合并阶段，不是产品排除，本阶段不宣称已经合入。

### 累计变化与保留功能

- 本轮保留普通生图的分组级 `image_count_billing` 开关、按实际返回图片数量计费、无法取得数量时的兼容回退、专用图片端点标记、渠道 token/按次/图片定价优先级、分组管理与可用分组 API 契约，以及默认关闭的 `248_group_image_count_billing.sql`。一次普通请求返回多张图片不等于退役的异步批量作业。
- 累计主上游管理员 API 密钥的操作与资源范围授权、摘要/IP/到期约束、轮换、二次验证、关联保护、事务锁顺序、Codex 身份及安全重试、容量退避、DOMPurify 安全更新和赞助资料全部保留。
- 累计第二上游用户密钥一次性展示/摘要/轮换、模型级质量状态和缓存证据、渠道统计、分组质量目标/预算/评分兜底/健康恢复、普通 API Key 预算耗尽分组回退、断连确证失败和统一错误展示继续保留。共享缓存预算是普通质量管理，不是共享账号池。
- 第二上游 QQ 配置分区、活动预览、群管理、双重举报审核、成员选择及隔离上下文、奖励与转账通知继续保留。本项目风险指定账号/平台/分组、原计费组与倍率、利润控制、采票指纹隔离、多代理池和网关策略未被覆盖。

### 文本融合与语义补修

- 逐段处理 25 个文件中的 70 个文本冲突块：服务、请求、DTO、仓储、缓存、前端类型/表单/双语文案和生成 ORM 增加普通按张计费字段，同时排除夹带的批量开关、折扣和冻结倍率。没有整文件简单选择 ours/theirs。
- 认证缓存版本在本项目摘要凭据 v27 基础上升级为 v28，防止复用没有新计费开关的缓存。保留摘要认证、并发限制、利润保护及所有本地字段；新增缓存序列化往返和旧版本拒绝回归。
- ORM 的 group runtime 描述符按实际 schema 重新对应索引；保留本项目 `user_concurrency_limit`，排序索引指向 `sort_order`。分组复制、创建、更新和数据库实体到服务的映射保留新字段。回归覆盖平台限制、部分更新、关闭图片权限、缓存失效及深复制。
- 前端将计费开关清理从退役批量配置中分离为 `resetDisabledImageCountBilling`；创建/编辑提交和平台/生图权限变化时都按 OpenAI/Composite 范围归一化，避免隐藏开关残留。新增两种支持平台的编辑保存及关闭生图权限回归。
- 新增 `image_count_context_tier_test.go` 复现 16 个上下文定价组合错误：Gateway 分组定价没有传 token，用渠道定价时漏掉缓存 token；OpenAI 图片分支的分组和渠道定价同样漏传上下文。修复 `gateway_usage_billing.go:calculateImageCost` 和 `openai_gateway_usage.go:calculateOpenAIImageCost`，将实际上下文用于按次/图片价格档位选择；Claude 加上缓存读写，OpenAI 的总输入已包含缓存，不重复相加。保留实际图片数量、图片倍率、显式图片单价及混合请求 token 计费。

- 补齐本项目持久化失效不变量：新增 `308_group_image_count_auth_cache_invalidation.sql`，在迁移 300 的分组触发器基础上加入 `image_count_billing` 变化检测，完整保留摘要 cache key、主/备用分组、软删除和权限/利润条件。避免直接 SQL 更新或提交后进程中断留下旧计费快照；新增迁移兼容断言与集成场景，历史迁移及校验和不变。

- 完整服务测试复现 `sync.Map.Store` 后台 panic：Grok 测试重置了已有异步刷新使用中的全局 `sync.Map`，而被测高级调度器实际使用独立实例缓存。删除五处无关且不安全的全局重置，并等待真正缓存写入而不是仅等待仓储调用；不改生产调度逻辑、不删用例、不放宽断言。

### 排除和数据库兼容

- 比对主上游与已合入第二上游的完整文件树，缺失分别为 65 个批量专属文件、70 个批量专属文件及两个共享账号池文件，没有其他已合入上游功能文件被删除。普通账号池、普通生图与异步单图任务保持支持。
- 明确排除共享账号池专属 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go`，不执行其退役 SQL。
- 本轮冲突文件只排除批量字段及其支持代码，保留各文件中的普通计费、权限、调度和其他功能。完整明确排除路径如下（专属文件保持不存在，混合用途文件仅排除对应实现）：
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
- `backend/ent/group.go`
- `backend/ent/group/group.go`
- `backend/ent/group/where.go`
- `backend/ent/group_create.go`
- `backend/ent/group_update.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/ent/schema/group.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/repository/group_repo.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
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
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/user/BatchImageGuideView.vue`

发布基准的 354 份历史 SQL 与 `backend/internal/repository/migrations_runner.go` 共 355 份兼容文件逐字节不变。保留累计新增的 247、248、296–307，并新增 `308_group_image_count_auth_cache_invalidation.sql`，共 15 份迁移。没有删除批量生图历史表、记录或冻结余额。

累计仍有两个具体部署兼容风险：`300_api_key_digest_only.sql` 将明文凭据不可逆转换为摘要并拒绝旧格式写入；`303_passive_quality_model_states.sql` 更换状态/租约主键与冲突目标，旧进程不兼容。升级需协调所有实例、提前备份并准备数据回退方案，不能只回退镜像。这些是累计保留功能的部署边界，不是未解决的合并冲突。

### 本轮最终验证记录

- 修复后完整重跑 14 个累计受影响后端包的可执行用例：23817 项测试及子用例通过，70 项按原条件跳过，零失败；服务层 18374 项通过。沿用已确认的环境依赖排除清单，相关用例保留且不计为通过。16 个上下文计费组合修复前均失败，最终全部通过；分组创建/更新、认证缓存往返/旧版本拒绝、迁移兼容断言和 Grok 并发修复回归均通过。
- 前端 439 个文件、3601 项测试全部通过；类型检查、完整 ESLint、五项国际化检查和生产构建通过。使用两个测试 worker，构建时关闭测试缓存，未修改断言或超时。临时 `frontend/node_modules` 符号链接已移除。
- 后端生产构建、全仓 unit 标签编译、全仓 integration 标签编译和受影响包 go vet 在最终复验中全部通过。标签编译使用 `-run='^$' -exec=/bin/true`，不计为真实数据库集成执行。Go 使用缓存 1.27.1 与 `/tmp` 隔离 modfile；仓库声明的 1.27.3 和依赖清单保持不变。
- 迁移包另有 110 项测试及子用例通过；新增数据库触发器集成场景完成编译。独立重新生成的完整 ORM 与工作树逐字节一致；发布基准的 354 份历史 SQL 与迁移运行器共 355 份兼容文件逐字节一致。
- 同步自动化 113 项中 111 通过、两项条件跳过；同步脚本语法及部署安全、网关环境、资源和 Caddy 检查通过。真实数据库集成、完整 golangci-lint 和在线漏洞审计未计为通过；这些验证边界不列入产品合并风险。
- 工作树无真实冲突标记，累计空白、功能排除及指定决策 schema 检查通过。真实索引、25 组冲突阶段及仓库引用保持原样，供外层检查后暂存。本轮仅修复工作树，未对工作仓库 fetch、git add、commit、push、打标签、发布或重启服务。


## 2026-10-25 第二上游文本与语义冲突审查

本阶段处理外层已经启动的第二上游合并：`origin/main=09c24e0c3`，`HEAD=ab065d24b` 已包含 `upstream/main=ba81c7fd7`，`MERGE_HEAD=overdraft/sub2api-custom=8c54f4005`。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）。审查覆盖两个上游累计 21 个不同的非合并提交、此前已合并但未发布的融合修复，以及本轮 91 个第二上游增量路径。

### 累计变化与保留功能

- 本轮完整保留 QQ 排名配置独立版本头、压缩 ETag 的旧客户端兼容及 CORS 暴露；完整保留可选 vm2api 专属账号路由、Docker/KVM 槽位生命周期、受管 CONNECT、管理员二次验证界面/API、桥接服务、部署脚本和文档。VM 槽位按母账号及其影子账号隔离，不是被排除的共享账号池。
- 累计主上游管理员 API 密钥按操作/资源授权、摘要/IP/到期/轮换和二次验证、事务关联保护、Codex 身份与空流安全重试、DOMPurify 更新全部保留。普通生图按实际图片数计费、端点/混合请求区分、渠道上下文档位、分组计费开关及认证缓存失效迁移保留。
- 累计第二上游用户密钥一次性展示/摘要/轮换、模型级质量/缓存证据、渠道统计、分组质量目标与共享缓存预算、评分兜底、健康恢复和预算耗尽回退全部保留；共享缓存预算与普通多代理池不属于共享账号池。
- 累计 QQ 配置分区、活动预览、群管理、成员隔离、双重举报审核、奖励与转账通知、断连后的确证失败和统一错误展示保留。本项目工具诊断、账号健康、风险选路、利润控制、采票/指纹隔离、网关策略及普通/异步单图继续保留。

### 冲突处理与语义融合

- 逐段融合七个文件、17 个文本冲突块。服务版本保持本项目 `2026.10.23`，第二上游标识为 `v0.2.18-custom.4`。依赖注入同时保留账号健康管理与 OpenAI VM 服务；管理界面同时保留工具诊断和专属槽位事件、弹窗及刷新逻辑。
- `openai_plugin_transport.go` 与 `openai_codex_models_service.go` 在现有同账号 429 重试内优先执行 VM 传输，保留非 VM 的插件、保护 TLS、代理校验和测试观测；VM 不可用不能静默转为直连；同一次请求一旦经过 VM，重试期间即使绑定消失且调度快照尚无受管标记，也必须返回错误，禁止回落到插件或原生直连。
- 回归复现并修复新 VM HTTP 主链绕过本项目保护配置、请求完整性和 429 重试，以及模型同步提前返回绕过重试的问题。保留续传内容、模型/费用策略、用量解析、响应绑定和失败时禁止直连的语义。
- WS 保留首帧/后续帧完整性校验、原生票据刷新、保护 TLS、429 握手重试、客户端输出互斥和断连用量收集；VM 使用独立 HTTP client 与凭据，不注入原生票据/TLS。首个上游输出前仍读取客户端控制帧，保留中断确认、后续轮次队列、逐轮授权和图片用量。
- VM 队列的中断和排队请求绕过原生策略读取器，故在接收时立即禁止首轮重放，并在最终重试判定前取消、等待额外读取协程退出，避免丢失已接收的下一轮或中断后重复执行。回归使用内存帧连接，不需要监听端口。
- 管理员受限 API Key 对新 VM 管理路径仍默认拒绝；新增明确断言。完整管理员操作继续沿用二次验证，不扩张已有权限目录。

### 排除与数据库兼容

逐一比对两上游完整树：候选缺少主上游 65 个批量生图专属文件；第二上游缺少 70 个批量生图专属文件（含面板批量鉴权及测试）、两个共享账号池专属文件，没有其他第二上游文件被遗漏。以下为本轮明确核对的排除路径；存在的混合用途文件仅排除退役功能代码、文案或专属文档白名单，保留其他实现。

共享账号池：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图：
- `.gitignore`
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `backend/internal/service/wire.go`
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/admin/accounts.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`

发布基准的 354 份历史 SQL 与 `backend/internal/repository/migrations_runner.go` 共 355 份文件逐字节一致；没有修改历史表、数据或冻结余额。保留累计新增迁移 247、248、296–307，以及本项目 `308_group_image_count_auth_cache_invalidation.sql` 和第二上游 `308_openai_vm_bindings.sql`。迁移运行器使用完整文件名排序及记账，两份 308 前缀文件互不覆盖，旧迁移与校验和兼容表均未改动。

累计部署兼容风险仍包括：迁移 300 不可逆地转换用户密钥存储并拒绝旧格式写入，迁移 303 改变模型级质量状态/租约的主键及冲突目标，旧进程不兼容；须协调实例升级、备份并准备数据库回退方案，不能只回退镜像。新增可选 VM 路由启用后，回退旧版本前必须先按 `docs/openai-vm.md` 清理全部绑定并恢复母账号/影子账号原代理，否则旧版本不能执行槽位生命周期和路由恢复。固定的 vm2api v1.3.225 不支持 compact，绑定账号的 compact 请求保留 404，不绕过隔离槽位直连；依赖该端点的客户端启用前须确认兼容性。这些是累计保留功能的部署边界，不是未解决的合并冲突。

本阶段仅修改临时工作树；未对工作仓库 fetch、git add、commit、push、打标签、发布或重启服务。真实索引及 :2/:3 保留，供外层临时索引验证后暂存。

### 本阶段验证结果

- 最终采用 19 个累计受影响后端包的可执行测试结果；最后的传输补修完成后重新运行整个 service 包。合计 24050 项测试及子用例通过、70 项按原条件跳过、零失败；新增五组融合回归（含子用例共 15 项）全部通过。另有需要本地监听、外部进程或真实数据库的原有用例保持原样，未在沙箱中计为通过。
- 前端完整重跑 442 个文件、3627 项测试通过，进程正常退出；类型检查、完整 ESLint、国际化五项检查、生产类型构建和 Vite 构建通过。首轮并发验证期间的 worker/hook 超时经完整重跑消除，未放宽断言或测试超时。临时依赖符号链接已经移除。
- 同步自动化 113 项中 111 通过、两项条件跳过，发布工具 13 项全部通过；部署安全、网关环境、资源、Caddy、同步及 VM 脚本语法检查通过。新 VM 部署测试中权限初始化需要映射到非 root UID，当前沙箱仅映射 UID 0，相关完整部署用例未计为通过；没有为此放宽生产目录权限或修改测试断言。
- Go 使用缓存 1.27.1 与 `/tmp` 临时 modfile，构建额外映射缓存的 x/term 版本；仓库声明的 1.27.3 与依赖清单保持不变。真实数据库集成、完整 golangci-lint 和在线漏洞审计未计为通过；这些验证边界不作为产品合并风险。
- 最终传输修改后的后端构建、全仓 unit/integration 标签编译及累计受影响包 go vet 全部通过；`-run=^$ -exec=/bin/true` 仅计为编译检查，不计为真实数据库集成执行。
- 最终真实冲突标记、累计空白、上游功能排除及指定决策 schema 检查通过。全部 21 条冲突索引阶段（七个文件）、真实索引和仓库引用保持原样；最终候选相对发布基准涉及 650 个路径。仅新增融合回归测试文件未进入索引，供外层统一检查和暂存。


## 2026-10-26 最终双上游兼容复核

复核对象为外层已生成的候选 `a4a8be136`，本项目基线 `origin/main=09c24e0c3`，主上游 `ba81c7fd7`、第二上游 `8c54f4005` 均为候选祖先。本次没有重新合并或改写索引：进入本阶段时真实索引已无未合并阶段，前述七文件的 :2/:3 保留说明仅描述上一阶段；本阶段通过合并父提交及原冲突对象记录核对双方实现。

从发布基准 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）到候选累计覆盖两上游 21 个不同的非合并提交、650 个变更路径。逐项复核七个原冲突文件及 HTTP/WS VM 传输、按张计费、权限与迁移交叉逻辑，保留上一阶段的融合修复，未发现需要追加的代码修复。保留完整第二上游 VM 槽位、QQ 配置版本/压缩兼容、模型质量目标、共享缓存预算、评分与预算耗尽回退、用户密钥摘要/轮换等功能；普通账号池及普通/异步单图不受排除影响。

独立比对两上游完整文件树：主上游缺少的 65 个路径全部为批量生图专属文件，第二上游缺少的 72 个路径为 70 个批量生图文件及两个共享账号池专属迁移/测试，没有遗漏其他上游文件。本轮明确继续排除的路径沿用上一节清单；混合用途文件只排除对应退役实现。发布基准 354 份历史 SQL 与迁移运行器共 355 份兼容文件逐字节一致；16 份新增迁移保留，两份 308 按完整文件名独立执行。未删除历史数据、表或冻结余额。

本阶段重新运行 VM 融合回归和跨网关图片上下文档位回归通过；真实冲突标记扫描、累计空白检查和功能排除检查通过。此前阶段记录的 24050 项后端测试/子用例及 3627 项前端测试通过结果已经核对日志，不冒充本阶段重新全量执行。此次扩大 VM 测试因本地监听受限而停止，前端重跑因工作树未安装依赖而无法启动；均未计为通过，也没有修改用例断言。此前集成标签检查仅为编译，真实数据库集成、在线漏洞审计等未完成项目仍由外层验证。

剩余具体兼容风险保持不变：迁移 300 的密钥摘要转换、迁移 303 的质量主键变更均要求协调实例升级并准备数据库回退；已启用 VM 时须先解绑再停用桥接或回退；固定 vm2api v1.3.225 不支持 compact，绑定账号返回 404 且不回落直连。当前无未解决文本或语义冲突，产品排除规则满足。仅补充本审查文档，未 fetch、git add、commit、push、打标签、发布或重启服务。


## 2026-10-27 主上游文本与语义冲突审查

当前阶段为 `origin/main=HEAD=726ee3254` 合并 `upstream/main=MERGE_HEAD=f178e5ae3`。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）。候选累计包含主上游及已经合入的第二上游 `8c54f4005` 的 54 个不同非合并提交，本次主上游增量为 33 个非合并提交。

已经阅读第二上游最新 `136706a53` 及前序 `787154384`：新增质量恢复预算/证据、遥测范围、分组质量汇总、独立评分、定时测试通过率保护与 VM 设置界面。这两个提交尚不在当前主上游合并阶段的候选中，须由外层后续第二上游阶段完整合并；本阶段没有把它们当作功能排除，也不宣称已完成最终双上游同步。

### 冲突与累计功能

- 七个文件的 25 处文本冲突逐段融合。账号行锁 SELECT/Scan 同时保留 OpenCode/Ollama 用量、上游计费限流、Codex 票据与质量/手动调度状态，以及新增 Grok 目录与能力快照；同步更新所有关联 SQL mock，包括未产生文本冲突的票据、质量保护、限流及新 Grok 测试。回归另发现上游 Grok CAS 快照测试漏掉本项目普通多代理池查询，补齐 `account_proxies` 预期并保留单账号快照刷新断言。
- DeepSeek Pro 保持官方 Pro 价格，保留命名空间/版本化名称、默认价请求时点峰谷倍率和自定义价不覆盖规则；同时合入上游固定计费时点接口及回归。缓存计数保留 DeepSeek `prompt_cache_hit_tokens`、Gemini `cachedContentTokenCount` 和显式数值零优先，避免折损计费与质量证据。
- Grok 统一目录解析、CAS 观察快照、响应模型审计、诊断界面和编辑草稿/模式隔离保留；本项目显式替换白名单仍要求实时列表，404/405 不可用配置兜底覆盖白名单，但记录失败并保留旧成功快照。新增融合回归覆盖该边界。前端上游新增 API 测试同时适配本项目已有 180 秒超时和显式替换参数，新增携带替换参数时保留诊断响应的断言；未删除现有 API 行为。
- 本次 GPT-6.1 Terra、UTF-16 Claude 指纹、OpenCode/DeepSeek developer 角色兼容、分页溢出保护、畸形工具调用拒绝、超限图片上传拒绝、Gemini fileData 图片计数、模型广场分组价格优先及国产频率型 429 短冷却完整保留。
- 累计管理员 API 密钥操作/资源授权、二次验证、用户密钥摘要/轮换、普通图片按实际张数与上下文档位计费、模型级质量目标/共享缓存预算/评分兜底、QQ 配置版本/压缩兼容与群管理、VM 独占槽位/HTTP/WS 路由和失败禁止直连均保留。本项目账号健康、利润控制、指纹/票据隔离和网关策略保持。

### 排除与兼容性

独立比对完整树：主上游缺少的 65 个路径全部为批量生图专属实现；已经合入的第二上游缺少的 72 个路径为 70 个批量生图文件及两个共享账号池迁移/测试，没有遗漏其他已合入第二上游文件。最新第二上游另有 62 个新增路径等待后续阶段，均不属于本次排除。

本轮明确核对并继续排除的共享账号池路径：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

本轮明确核对并继续排除的批量生图路径：
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`

现存混合用途文件继续保留普通账号、生图和异步单图代码；功能排除检查覆盖路由、依赖注入、分组权限/计费、配置和前端入口。本轮无需新增删除，未恢复任何退役运行时实现。发布基准的 354 份历史 SQL 与 `backend/internal/repository/migrations_runner.go` 共 355 份文件逐字节一致；累计 16 份新增迁移保留，两份 308 按完整文件名独立记账。未删除历史表、记录或冻结余额。

累计部署兼容边界仍包括：迁移 300 的密钥摘要转换不可仅回退镜像；迁移 303 的模型级质量主键不兼容旧进程。必须协调实例升级并准备数据库备份/回退。启用 VM 后须先解绑、恢复原代理再停用桥接或回退；固定 vm2api v1.3.225 的 compact 请求保留 404。新频率型 429 判断依赖额度快照，默认十分钟探测间隔内真实耗尽可能暂时按短冷却重试，刷新快照后恢复额度窗口冷却。历史批量任务如仍未终结，升级前仍须用旧版本完成或取消并核对冻结余额，当前版本不恢复旧队列处理。

本轮只修改工作树，真实索引的七组 :2/:3 保留供外层验证后暂存；未 fetch、git add、commit、push、打标签、发布或重启服务。

### 本阶段验证结果

- 融合回归 311 项测试及子用例通过；本轮 Terra、图片上传/计数、定价、角色、UTF-16 指纹、频控及畸形响应等功能回归 158 项通过；管理权限/诊断回归 283 项通过、1 项按既有条件跳过；另完整覆盖本轮变更的仓储单元测试 104 项通过。这些集合有交集，不相加为独立用例总数。分页、响应、协议兼容、OpenAI 包测试及指定 Antigravity/审计测试通过。
- 前端全量执行 444 个文件、3749 项测试：首轮 3747 项通过，新 Grok API 文件两项因未适配既有超时/替换参数失败；修正并新增回归后，该文件四项通过，全部六个本轮受影响前端测试文件重跑 212 项通过。没有再次执行未受修改影响的其余前端测试；不把首轮描述为全绿。类型检查、完整 ESLint、修正文件 ESLint、国际化测试和生产构建通过。
- 后端构建、全仓 unit/integration 标签编译和十个受影响包的 go vet 通过。使用缓存 Go 1.27.1 与 `/tmp` modfile，构建将缺失的 x/term 映射到缓存 v0.45.0；仓库 Go 1.27.3 和依赖声明未改。标签编译使用 `-run=^$ -exec=/bin/true`，不计作真实数据库集成执行。
- 同步自动化 113 项中 111 通过、2 项条件跳过；部署安全、网关环境、运行资源、Caddy 和脚本语法检查通过。需要本地监听的 Grok OAuth、Antigravity 列表、代理质量、审计外呼与 miniredis 权限用例未计为通过，其他独立检查完成；真实数据库集成、完整 golangci-lint 和在线漏洞审计未执行。本阶段不是外层发布验证完成声明。
- 最终候选相对发布基准涉及 732 个路径。真实冲突标记扫描、累计空白检查、功能排除检查、指定决策 schema 校验通过。21 条冲突索引阶段保持原样，测试依赖临时符号链接已移除。


## 2026-10-27 第二上游文本与语义冲突审查

本阶段 `HEAD=8bd1f81ae` 已包含主上游 `f178e5ae3`，正在合并第二上游 `136706a53`；本项目基线为 `origin/main=726ee3254`。累计审查从正式发布基准 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）开始，覆盖两上游 56 个不同的非合并提交，包括此前已经合入但未发布的更新。

### 冲突处理与功能保留

- 逐段解决四个文件、五处文本冲突。服务 `VERSION` 维持本项目已发布日期版本，`FORK_VERSION` 保留第二上游版本标识；没有生成新的发布版本。缓存恢复 SQL mock 同时保留本项目显式关闭错误处理和新增模型参数。质量集成测试提取共享建表夹具，同时保留排除账号字段与原分页/独立保护断言。设置页测试同时保留管理员访问密钥、导航排除与 Codex 打票代理断言，以及新增 VM 独立表单、URL 切换测试。
- 语义复核修复新增通过率保护与本项目账号写入/缓存的衔接：`backend/internal/repository/account_repo.go` 在行锁内保留最新 `active_pass_rate_holds`，旧 OAuth 刷新、表单快照或显式账号状态编辑均不能清除、复活或注入其他保护所有者的限制；`backend/internal/repository/scheduler_cache.go` 将该字段纳入调度投影。新增 `account_pass_rate_merge_test.go` 覆盖五种状态交错，并经过真实缓存投影与 JSON 往返验证模型隔离，数据库最终准入保持不变。
- 第二上游两个新增提交完整保留：独立真实流量评分及配置版本冲突检查、定时测试通过率保护/持久证据/删除投影修复、恢复预算预留及不可变证据、遥测缺口范围与运行诊断、分组质量报告与分钟预算、供应商隔离、评分/会话亲和回退、QQ 1–10 次连抽的事务及幂等、VM 全局设置与二次验证。QQ 连抽不属于批量生图。
- 累计主上游 Terra、Grok CAS 目录/诊断/响应审计、DeepSeek 定价/角色/缓存、UTF-16 指纹、图片限额与实际张数计费、Gemini fileData 计数、分页/畸形响应/429 修复保留。既有管理员密钥范围权限、用户密钥摘要/轮换、账号健康与利润控制、票据/指纹隔离、普通多代理池、模型级质量目标与缓存预算、QQ 配置压缩/群管理、VM HTTP/WS 槽位与禁止直连回退均保留。新质量接口仍受管理员鉴权；受限管理密钥没有因接口新增而获得目录通配权限。

### 排除与数据库兼容

独立比对完整上游树：主上游缺少的 65 个路径均为批量生图专属；第二上游缺少的 72 个路径仅为 70 个批量生图专属文件和两个共享账号池专属迁移/测试，没有遗漏其他第二上游文件。混合用途的 `group_repo.go` 保留普通生图及按张计费，同时继续排除批量生图权限/折扣/冻结配置。

本轮明确核对并继续排除的共享账号池路径：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

本轮明确核对并继续排除的批量生图路径（最后一项仅排除对应代码）：
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`
- `backend/internal/repository/group_repo.go`

发布基准的 354 份 SQL 与迁移运行器共 355 个兼容文件逐字节不变；累计新增 21 份 SQL，包括此前两份不同文件名的 308 及本轮 309–313。没有删除历史表、记录或冻结余额。普通生图和异步单图任务继续可用。历史批量任务如果仍未终结，升级前应使用旧版本完成或取消并核对冻结余额，当前版本不恢复退役队列。

### 累计剩余兼容风险

- 迁移 300 的密钥摘要转换与迁移 303 的模型级质量主键不兼容旧进程，必须协调实例升级并准备数据库备份/回退，不能仅回退镜像。启用新增通过率保护前也需完成所有服务实例升级，旧进程不识别该独立模型保护。
- 已启用 VM 时，停用桥接或回退前必须解绑并恢复母账号/影子账号原代理。固定 vm2api v1.3.225 不支持 compact，绑定账号保留 404；CONNECT 白名单外的图片 CDN 下载也会失败，不自动绕过槽位。
- 国产频率型 429 判断依赖额度快照，默认十分钟探测间隔内真实额度耗尽可能暂按秒级冷却重试，刷新快照后才恢复额度窗口冷却。

本阶段只修改工作树，四组冲突的 12 条真实索引阶段保持原样；未 fetch、git add、commit、push、打标签、发布或重启服务。最终验证结果补记如下。

### 本阶段最终验证

- 后端融合及受影响用例去重后共 315 项测试/子用例通过，包括新增五种账号保护交错场景；前端关闭测试缓存后全量运行正常退出，454 个文件、3845 项测试全部通过。前端类型检查与完整 ESLint 通过。
- 后端服务构建、前端生产构建通过；后端全仓 unit/integration 标签编译及五个修改包的 `go vet` 通过。编译检查不等同于数据库集成测试执行。
- 同步自动化测试 113 项（111 通过、2 跳过）及发布工具测试 13 项通过；部署安全、环境配置、资源配置、Caddy 配置及 shell 语法检查通过。测试中的仓库和发布动作仅发生在临时测试夹具中。
- 完整后端 unit 执行因沙箱禁止本地监听而在 miniredis/httptest 用例中中断；完整 golangci 检查遇到包加载器的 modfile 错误。真实数据库集成及在线漏洞审计未执行，均不计为通过。Go 检查使用缓存的 1.27.1 工具链和临时 modfile，未改仓库要求的工具链或依赖声明。
- 全工作树文本冲突标记扫描、相对发布基准的 `git diff --check`、上游功能排除检查及决策 JSON schema 校验通过。再次核对 355 个已发布数据库兼容文件未变。临时前端依赖链接已移除，仅留下预期源码、测试和审查文档修改。

## 2026-10-27 最终双上游兼容复核

本阶段开始时，外层已将前两阶段修复纳入候选 `5a2e46947`，其父提交分别包含主上游 `f178e5ae3` 与第二上游 `136706a53`；真实索引没有未合并阶段，工作树干净。本阶段没有重建或暂存索引，也没有 fetch、commit、push、打标签、发布或重启服务。

再次审阅 `origin/main=726ee3254`、两上游提交记录和合并差异，累计范围仍从 `v2026.10.23=23fd587350` 开始，包含 56 个不同的上游非合并提交及此前三轮已合并、未发布的更新。主上游 Terra、Grok 目录 CAS/诊断/响应审计、DeepSeek 定价/缓存/角色兼容、UTF-16 指纹、分页与畸形响应保护、普通图片按实际张数及上下文档位计费均保留；第二上游的密钥摘要与轮换、模型级质量目标/共享缓存预算、独立评分/通过率保护、恢复证据与遥测、会话亲和回退、QQ 配置/群管理/事务连抽、VM HTTP/WS 隔离槽位及独立设置均保留。共享缓存预算和 QQ 连抽不属于被排除的两个产品功能。本项目管理员范围权限、二次验证、账号健康、利润控制、票据/指纹隔离和普通多代理池保持。

复核发现并修复一个此前未覆盖的语义缺口：完整账号保存已在行锁内保护 `active_pass_rate_holds` 与 `passive_quality`，但通用 `UpdateExtra` 和 `BulkUpdate` 仍会接收这些字段，允许旧表单清空限制或注入陈旧限制。`backend/internal/repository/account_protection.go` 新增专属投影过滤，`account_repo.go` 的通用增量写入仅合并普通字段，从而原子保留数据库当前限制；创建/导入/复制账号也不会复制没有对应持久所有者的限制。专用质量流程继续直接在账号锁下更新投影，恢复、计划删除和缓存事件语义不变。新增单元回归先确认四种更新场景失败，再确认修复后通过；新增数据库集成回归覆盖创建、清空、覆盖及恢复后旧快照重放。

独立完整树比对再次确认：主上游缺少的 65 个路径全为批量生图专属；第二上游缺少的 72 个路径全为上一节列出的 70 个批量生图文件及两个共享账号池迁移/测试。继续明确排除上一节全部路径，`group_repo.go` 仅排除批量生图权限、折扣和冻结字段；没有删除任何其他第二上游功能。`panel_api_key_auth.go` 的上游调用链仅服务已退役的面板批量图片路由，普通密钥鉴权保留。354 份已发布 SQL 与迁移运行器共 355 个文件逐字节不变，累计 21 份新增 SQL 保留；历史批量生图 SQL、校验和、表、记录及冻结余额不变，普通生图与异步单图入口保留。

最终复核的剩余兼容边界沿用上一节：迁移 300/303 与旧进程不兼容，启用通过率保护需所有实例支持；VM 停用/回退须先解绑并恢复原代理，固定 vm2api 版本不支持 compact 且 CDN 仍受 CONNECT 白名单约束；频率型 429 在额度快照刷新前可能误用短冷却。这些是累计代码/数据库兼容边界，不是工具或验证环境问题。

本阶段重新执行仓储受影响测试与通用写入回归，去重后 133 项测试及子用例通过；后端构建与仓储 unit/integration 标签 `go vet` 通过。前端没有修改，沿用前阶段全量 454 文件/3845 项测试、类型检查、ESLint 和构建的已记录结果，不声称本阶段重跑。新增数据库集成回归的实际执行由外层完整验证负责；编译不替代数据库集成执行。

最终全仓编译分别使用 `-tags=unit` 与 `-tags=integration`，两者均通过；最初同时启用两种标签触发现有 TLS 测试重名，改为项目规定的独立编译后消除。构建/编译沿用前阶段缓存工具链与临时 modfile，没有修改仓库依赖。全工作树冲突标记扫描、累计空白检查、新增文件空白检查、功能排除检查和指定决策 schema 校验通过。真实索引仍未修改，保留五个预期源码、测试与文档变更供外层检查暂存。


## 2026-10-28 主上游文本与语义冲突审查

本阶段基于 `origin/main=HEAD=dfb9c0ee0`，处理已经开始的 `upstream/main=a3298b46e` 合并；主上游共同基点为 `f178e5ae3`。发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`），审查覆盖候选中两上游 70 个不同的非合并提交（主上游 58 个、第二上游独有 12 个），包括前四轮已合入、尚未发布的更新。本轮主上游新增 14 个非合并提交。候选相对发布基准涉及 878 个文件；另有本项目日期版本同步，不计入上游提交数。

第二上游已合入基点为 `136706a53`；已审阅最新 `overdraft/sub2api-custom=136ab14d9` 的六个增量提交和路径差异，涉及模型级保护、自动复核与渲染、输出 TPS、质量治理简化、可用回退容量、用户账号亲和及恢复稳定性。其 67 个新增路径属于外层后续第二上游合并阶段，不是产品排除，本阶段不将尚未合入的功能报告为已完成。第二上游既有功能完整保留。

### 文本冲突与语义融合

- 逐段解决四个文件、七个文本冲突块，未选择整文件 ours/theirs。`backend/cmd/server/VERSION` 保持本项目已发布的 `2026.10.23`，不以主上游 `0.2.19` 覆盖，也不提前生成发布版本。
- `backend/go.mod` 保留主上游 `x/net v0.62.0`、`x/crypto v0.58.0` 及配套依赖更新，同时保留本项目较新的 `x/image v0.45.0`；其校验和仍在 `go.sum`。仓库 Go 版本保持 `1.27.3`。
- Gemini 原生、Claude Messages 和 Chat Completions 三条路径使用新的可配置同账号重试预算；签名降级独立计数，退避按扣除降级后的实际重试次数计算。保留本项目 `sleepRetryBackoff` 注入接口、透明 429 预算和耗尽标记、上游计费故障分类与脱敏、质量调度及完整流结束判定。
- `gemini_messages_compat_service.go` 同时保留 `newGeminiUpstreamBillingFailoverError` 与新增 `boundCacheUpstreamPassthroughError`。绑定显式缓存仍只访问原账号，绕过本项目透明 429 重试，由 handler 按普通池模式的状态码/次数重试；失败不污染账号健康，耗尽后保留绑定缓存的透传语义。
- 复核发现旧 `gemini_429_retry_failover_test.go` 仍引用已删除的常量，现改为三条路径 × 0/1/4 次配置的回归矩阵，同时检查透明预算只消耗一次和退避序号；签名降级测试补充两次降级后退避仍从 1 开始的融合断言。

### 本轮及累计功能保留

本轮保留 OAuth 非流式 Responses 的 SSE 聚合、缺失 Content-Type 的帧识别、incomplete/cancelled 终态 JSON 与用量；模型列表 singleflight 内二次缓存检查；Gemini 声明映射列表/详情与分组白名单、默认目录更新、Google One 保守测试模型及普通图片模型识别；渠道网页搜索设置的跨平台维度保留与显式清空；管理员保护错误使用 400 状态。

累计保留主上游 Terra、Grok CAS 目录/诊断/响应审计、DeepSeek 价格/缓存/角色兼容、UTF-16 指纹、分页溢出保护、畸形响应拒绝、上传大小保护及按实际图片数量计费。已合入第二上游的用户密钥摘要与轮换、模型级质量目标及共享缓存预算、独立评分和定时测试通过率保护、恢复预算/证据/遥测、评分和会话亲和回退、QQ 配置压缩/群管理/事务连抽、VM HTTP/WS 独占槽位及独立设置继续保留。共享缓存预算、QQ 连抽和普通账号代理池均不属于退役功能。本项目管理员范围权限、二次验证、账号健康、利润控制、指纹/票据隔离和普通多代理池保持。

### 排除路径与数据库兼容

完整树比对确认主上游缺少的 65 个路径均为批量生图专属；已合入第二上游缺少的 72 个路径仅为 70 个批量生图文件及两个共享账号池迁移/测试。`panel_api_key_auth.go` 仅服务退役的面板批量图片路由，普通 API 密钥鉴权仍保留。混合用途的 `group_repo.go` 和 `wire_gen.go` 只继续排除批量字段和后台依赖，其他行为不变。本阶段无需新删除退役文件，下列路径已逐项核对并明确继续排除。

共享账号池：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图（末两项仅排除相应代码）：
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`
- `backend/internal/repository/group_repo.go`
- `backend/cmd/server/wire_gen.go`

发布基准中的 354 份 SQL 与 `migrations_runner.go` 共 355 个兼容文件逐字节保持不变；累计新增 21 份 SQL 保留，两份不同文件名的 308 继续按完整名称记账。历史批量生图迁移及校验和兼容记录不变；不操作历史表、记录或冻结余额。普通生图和异步单图入口保持。

### 累计剩余兼容边界

- 迁移 300 的密钥摘要转换和迁移 303 的模型级质量主键不兼容旧进程；不能仅回退镜像，需要协调实例升级及数据库备份/回退。通过率保护也必须在全部服务实例支持后启用。
- 有 VM 绑定时，停用桥接或回退旧版本前必须解绑并恢复母账号和影子账号原代理。固定 vm2api v1.3.225 不支持 compact；CONNECT 白名单外的图片 CDN 访问失败，不绕开独占槽位或改走直连。
- 国产频率型 429 判断依赖额度快照，默认十分钟探测窗口内真实额度耗尽可能暂按秒级冷却重试，刷新快照后恢复额度窗口冷却。

本阶段仅修改工作树，不执行 fetch、git add、commit、push、打标签、发布或服务重启；四组冲突的真实索引阶段保留供外层核对。

### 本阶段验证

- 前端全量 455 个文件、3850 项测试通过；类型检查、完整 ESLint、五项 i18n 检查及生产构建通过。临时依赖符号链接已移除，构建产物仅放在 `/tmp`。
- 后端六个受影响包的 220 个顶层测试函数、535 项测试及子用例全部通过，覆盖配置、处理器、模型目录、仓储保护投影、三路径 429 预算与签名降级、绑定缓存重试、OAuth SSE 聚合和累计兼容边界。Ent schema 测试在隔离源码副本通过；服务生产构建、全仓 integration 标签编译与受影响配置/处理器/仓储/服务包的 `go vet -tags=unit` 通过。integration 检查使用 `-run='^$' -exec=/bin/true`，仅确认测试可编译，不执行数据库测试。
- 同步自动化 113 项（111 通过、2 跳过）、发布辅助 13 项通过；部署安全、网关/简化模式环境、资源、Caddy 及 shell 语法检查通过。
- 后端使用精确 Go 1.27.3 工具链和 `/tmp` 隔离 modfile，将六个本地未缓存的新依赖临时映射至已有缓存版本；仓库依赖及安全升级未降级。因此这些结果是融合行为的兼容验证，不代表正式依赖版本的完整验证。
- 已执行全量后端 unit；18 个包因本地监听/miniredis 受限或 Ent 子进程的 modfile 参数问题未通过，后者已在隔离副本复验通过。没有将全量 unit 报为通过；真实数据库集成及在线漏洞审计未执行。
- 全仓 `golangci-lint` 在 8 分钟限制后超时，没有返回 lint 结果；Ent schema 独立包检查通过且报告 0 issues。受影响包 `go vet` 已通过。
- 历史 SQL/兼容文件、排除检查、源码真实冲突标记、累计差异空白与 31 个改动 Go 文件格式检查通过，决策 JSON 按目标仓库完整 schema 验证通过。真实索引 SHA 和全部冲突 stage 逐字节未变。

## 2026-10-28 最终双上游兼容审查

候选基准为 `origin/main=dfb9c0ee0`，主上游为 `upstream/main=a3298b46e`，第二上游为 `overdraft/sub2api-custom=136ab14d9`；发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）。本候选相对发布基准累计包含 76 个不同的上游非合并提交（主上游 58 个、第二上游独有 18 个）和本项目版本同步；净差异为 973 个文件、增加 87,594 行、删除 3,765 行及 3 个二进制路径。候选已包含两个上游指针，审查范围涵盖本轮合并及此前全部未发布更新。

### 冲突融合

- `backend/cmd/server/VERSION` 保持 `2026.10.23`；`FORK_VERSION` 保留第二上游的 `v0.2.18-custom.12`。
- `backend/internal/repository/scheduled_test_actions_repo.go` 按模型读取和初始化保护状态，保留模型级组合/全通过决策、统计检测账号级保护与账号分组一致性约束。
- `backend/internal/repository/scheduled_test_repo.go` 保留多模型 `model_ids` 的读写、结果展示证据补水和关闭错误处理。
- `backend/internal/service/account_test_service.go` 同时保留 Agent Identity 智能测试保护与第二上游视觉输入。
- `backend/internal/service/group_quality_fallback_test.go`、`openai_account_scheduler.go` 和 `openai_gateway_scheduling.go` 同时覆盖精确风险账号/compact 门控、账号亲和迁移、预算拒绝和健康容量回退。
- 七处文本冲突均已融合，未发现待解决冲突标记。当前索引没有未合并条目；双方实现通过合并提交父树逐文件比对。

### 本轮排除和保留

本轮第二上游新增的 `backend/migrations/255_retire_shared_account_pool.sql` 及 `backend/migrations/retire_shared_account_pool_integration_test.go` 不进入候选。批量生图专属 UI、API、队列/worker、计费、供应商适配、测试、文档及 ORM 路径按本文件上一节的逐项清单继续排除；混合文件只排除对应实现：`backend/internal/repository/group_repo.go`、`backend/cmd/server/wire_gen.go`、`backend/internal/server/routes/gateway.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/usage_billing.go`、`backend/internal/repository/usage_billing_repo.go`、`backend/ent/mutation.go`，不移除普通账号池、普通生图、异步单图或普通 API 密钥鉴权。

发布基准中的批量生图历史迁移 159–169、187、234、`backend/internal/repository/migrations_runner.go` 及 checksum 兼容记录逐字节不变；没有删除历史表、记录或冻结余额。候选新增迁移 247、248、296–318 均保留，包含第二上游的被动规则治理和多模型质量更新。

### 累计剩余风险与验证

- 迁移 300 将 API 密钥改存为摘要并清除含凭据的历史幂等响应；迁移 303、315 改变模型级保护和定时测试状态键，迁移 318 归档配置后退休自定义被动规则及其状态/租约。已运行这些迁移后，单独回退旧二进制不能恢复旧数据库语义；需协调所有实例升级并保留可恢复备份。
- VM 账号绑定需使用支持 compact 的桥接版本，并按需配置图片 CDN 的 CONNECT 白名单；不满足时相应请求会失败。
- 国产频率型 429 判断依赖额度快照，额度耗尽至快照刷新期间可能暂用较短冷却。
- `backend/internal/payment/provider/easypay.go` 继续拒绝非标准回调字段；使用附加字段的支付渠道需改用标准回调或主动查询/对账，不能放宽签名校验。
- `backend/internal/service/image_storage.go` 通过环境代理下载时，只能预检本地 DNS 解析；若代理实际解析到私网目标，代理侧仍可能访问非公网地址。
- `backend/internal/sourceupdater/updater.go` 自动回滚只恢复应用镜像，不回滚已执行的数据库迁移；升级前仍需备份并确认数据库模式兼容。
- `python3 .github/check-upstream-exclusions.py .`、决策 schema、累计 `git diff --check`、历史迁移字节比对和源码冲突标记检查通过。定向 Go 测试未运行：本地工具链需要下载 Go 1.27.3，沙箱网络禁止该下载；此项是验证限制，不是产品风险。


## 2026-10-29 第二上游文本与语义冲突审查

本轮工作树基准为 `origin/main=HEAD=b8816ae2c`，主上游 `upstream/main=a3298b46e` 已在基准内；第二上游推进到 `overdraft/sub2api-custom=3604ba2fc`。按要求只读本地上游引用，不 fetch、暂存、提交、推送、打标签、发布或重启。原索引的 53 个冲突路径及双方 stage 原样保留，工作树中的 70 个文本冲突块逐段融合；七个修改/删除冲突按新的主动保护架构处理，不以整文件 ours/theirs 代替语义审查。

发布累计基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）。累计审阅 82 个不同的上游非合并提交（主上游 58 个、第二上游独有 24 个），包括本轮六个第二上游增量及此前已合并但尚未发布的更新。

### 冲突融合与不变量

- 服务版本仍为 `2026.10.23`，第二上游 `FORK_VERSION` 更新保留。依赖注入与清理移除已退役被动质量服务，保留本项目账号健康管理和 Codex ticket harvester 清理。
- 严格账号分层、实际上游故障证据、响应归属锁定与风险账号/分组路由共同生效：风险指定账号不得被通用回退替换，也不得绕过当前层级；本项目利润、隐私、模型能力、compact 冷却、代理和计费授权限制继续保留。最终准入错误直接返回，不转成等待计划。
- HTTP/WS 的真实上游状态与本项目透明 429 重试耗尽标记同时保留；保留主上游 Gemini 配置重试预算、签名降级独立计数、绑定显式缓存同账号重试及不污染账号健康的语义。
- 保留 OpenAI 工具诊断、请求完整性暂存、WS 首输出缓冲与取消/用量排空；只移除已不存在的被动质量租约引用。原普通 WS 排空测试迁移到独立测试文件。
- 账号全量写入继续在行锁下保留当前 `active_pass_rate_holds`，同时剔除退役 `passive_quality`、`group_quality_targets` 和 trial 元数据。旧管理表单和慢刷新不能解除新保护或恢复旧保护。统计规则仓储的扫描列与新查询一致，并保留关闭错误处理。
- 双方都使用缓存版本 28、内容却不同，融合后提升 API 密钥缓存版本至 29，包含按图片数量计费、密钥摘要与严格分层配置。Ent 保留新 `strict_failover` 字段，重新核对排除批量字段后的默认值下标、排序列下标与 mutation 字段容量。
- 新增小时统计 API 精确登记到全站 `ops.read` 权限；账号范围读取权限不能获取全站统计，未登记接口仍默认拒绝。管理员账号页同时保留工具诊断和新的完整滚动小时统计；渠道监控测试保留 RouterLink 的目标校验，并对齐到新的测试计划页。

### 上游变化及累计行为

本轮第二上游的全部非排除更新保留：主动定时测试与恢复、完整小时保护、模型级通过率、测试时间/TPS 和 HTML 展示、独立账号小时统计、渠道监控历史聚合边界修复、严格故障转移、QQ 名称唤醒与嵌套消息识别。知识截止检查随后由迁移 320 退役；评分、被动规则、统计型测试和客户流量恢复已由最新上游明确退役，迁移 322–324 归档配置并移除旧状态，与主动测试新架构一致，这些不是产品排除项。

累计保留主上游 Terra、Grok 目录 CAS/诊断/响应模型审计、DeepSeek 价格与缓存/角色兼容、管理员范围密钥、UTF-16 指纹、分页与上传边界、真实图片数量计费、OAuth 非流式 SSE 聚合、Gemini 模型声明/目录/重试、模型列表 singleflight 二次缓存和跨平台搜索设置。此前第二上游的密钥摘要/轮换、多模型测试与自动复核、账号亲和、QQ 配置版本/群管理/事务连抽、VM HTTP/WS 独占槽位和独立设置均保留；质量治理遵循本轮主动架构，不恢复已被后续上游替代的评分与被动流量逻辑。本项目权限、二次验证、健康管理、利润控制、票据/指纹隔离及普通代理池继续保留。

### 本轮明确排除的路径

完整路径对照：候选缺少主上游 65 个路径，均为批量生图专属；缺少第二上游 72 个路径，仅为以下 70 个批量生图路径和两个共享账号池路径。`panel_api_key_auth.go` 只服务面板批量图片接口，普通 API 密钥鉴权保留。混合文件仅排除批量依赖或字段，新增严格分层代码保留。

共享账号池：
- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图专属路径：
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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`

混合路径：`backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen.go`、`backend/ent/migrate/schema.go`、`backend/ent/mutation.go`、`backend/ent/runtime/runtime.go`，仅排除批量任务依赖和 ORM 字段。

发布基准全部历史 SQL 迁移及 `backend/internal/repository/migrations_runner.go` 逐字节不变；本轮新增迁移 319–325 保留。批量生图历史表、记录、冻结余额及 checksum 兼容记录不动，普通生图与异步单图支持继续保留。

### 累计剩余风险

- 迁移 300 的密钥摘要、303/315 的模型级状态键，以及 318/322–324 的质量配置归档和状态表退役不兼容旧进程；需协调全部实例升级并保留数据库备份。自动更新器只回滚镜像，不回滚这些数据库迁移。
- VM 账号的 compact 需要桥接版本支持，图片 CDN 仍受 CONNECT 白名单约束；停用桥接或回退前需解绑并恢复原代理，否则相关请求失败。
- 国产频率型 429 分类依赖额度快照，真实额度耗尽到刷新期间可能短冷却重试。
- EasyPay 仍拒绝非标准回调字段，使用附加字段的渠道须走标准回调或主动查询/对账。
- 图片下载经环境代理时，本机 DNS 预检不能约束代理端的实际解析，代理可能访问本机预检未发现的私网地址。


### 本阶段验证结果

- 前端全量 444 个文件、3,784 项测试通过；类型检查、完整 ESLint 和生产构建通过。首轮发现的旧导航目标断言已修复并全量重跑；依赖链接仅临时用于验证，构建产物位于 `/tmp`。
- 后端三组干净回归去重后 302 个顶层测试、1,029 项测试及子用例通过，覆盖严格分层与风险路由、准入失败、响应所有权、计费组、行锁保护投影、HTTP/WS 429 预算、Gemini 配置重试/签名降级/绑定缓存、主动测试和小时统计，以及权限目录。
- 服务生产构建、全仓 integration 标签编译、受影响服务/仓储/处理器/路由/中间件/启动包的 `go vet -tags=unit` 通过。integration 使用 `-run='^$' -exec=/bin/true`，只证明编译通过，不替代实际数据库测试。
- 两次较宽测试筛选受 httptest/miniredis 本地监听限制而失败，随后对相关无需监听的回归按测试文件或精确名称重跑通过；没有把完整后端 unit 或数据库集成标为通过。
- 验证使用 Go 1.27.3 与隔离临时 modfile，将六个本地未缓存的依赖映射到已有缓存版本；仓库 `go.mod`、`go.sum` 和安全升级未修改。因此后端结果用于融合兼容验证，正式依赖的完整验证与在线漏洞检查仍由外层执行。
- 历史 SQL 和 checksum 兼容记录字节核对、功能排除、真实冲突标记、累计空白和改动 Go 格式检查通过。真实索引的全部 stage 与开始时逐字节一致；只解决工作树，不清除外层要求保留的冲突索引。


## 2026-10-29 最终双上游兼容复核

本阶段开始时，外层已生成候选合并提交 `98ddfc242`，父提交为 `origin/main=b8816ae2c` 与 `overdraft/sub2api-custom=3604ba2fc`；主上游 `upstream/main=a3298b46e` 已包含。工作树和真实索引没有未合并条目，前阶段 53 个冲突路径通过合并父树继续核对。本阶段没有 fetch、git add、commit、push、打标签、发布或重启，真实索引保持原样。

累计复核仍覆盖 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）以来 82 个不同的上游非合并提交（主上游 58 个、第二上游独有 24 个），包括此前已经合并但尚未发布的更新。按第二上游最终树保留严格账号分层、主动定时测试、模型级通过率、完整小时统计、监控历史聚合、QQ 交互、VM 桥接、密钥摘要/轮换和账号亲和；评分、被动保护和统计型测试按第二上游后续提交退役，不将这些架构替换误判为产品排除。

### 本阶段补齐的语义兼容

- `backend/internal/service/gateway_scheduling.go`：通用网关的非批量调度入口及风险指定账号入口，实时准入或代理槽位获取错误直接返回；不再把主动保护拒绝或数据库错误转换成成功的并发等待计划。普通槽位繁忙仍可等待；风险账号不得被通用回退替换，严格层级和原计费分组约束保持。新增回归先复现了返回 nil 错误的失败，修复后两种调度配置均通过。
- `backend/internal/repository/account_repo.go`、`account_protection.go`：全量写入在既有行锁下保留当前 `active_test_model_holds` 与 `active_test_all_models_paused`，与 `active_pass_rate_holds` 一致；创建/复制、单条及批量 JSONB 补丁不得复制、注入或清除这些受管投影。旧表单和慢刷新不能恢复过期模型保护或清掉新保护，专属主动测试写入与恢复保持。全量覆盖和补丁回归均先复现失败，再验证修复；数据库集成用例也补齐了这些字段。
- 复核 API 密钥缓存版本 29 同时携带图片数量计费、严格分层及摘要凭据，Ent 分组默认字段下标全部对应；小时统计仍精确要求全站 `ops.read`，普通账号读取权限不能读取全站统计。

### 排除与迁移兼容

候选缺少主上游 65 个路径，均为批量生图专属；缺少第二上游 72 个路径，仅为上一节逐项列出的 70 个批量生图专属路径及两个共享账号池路径。共享账号池继续明确排除 `backend/migrations/255_retire_shared_account_pool.sql`、`backend/migrations/retire_shared_account_pool_integration_test.go`。面板 API 密钥中间件只服务批量图片接口，普通 API 密钥鉴权保留。

除上一节 70 个专属路径外，本阶段还逐项扫描并确认以下混合文件中的批量生图依赖、字段、入口、计费、配置或专属测试片段已排除，文件的其他功能保留：

- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/cmd/server/wire_gen_test.go`
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
- `backend/ent/schema/group.go`
- `backend/ent/tx.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/admin/group_handler_simple_mode_test.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/gateway_model_allowlist_listing_test.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/security_audit_media_submit_test.go`
- `backend/internal/handler/security_audit_order_test.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/group_repo.go`
- `backend/internal/repository/usage_billing_repo.go`
- `backend/internal/repository/usage_billing_repo_unit_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/middleware/api_key_auth.go`
- `backend/internal/server/middleware/group_model_allowlist_test.go`
- `backend/internal/server/router.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/prompt_audit_route_coverage_test.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_group_duplicate.go`
- `backend/internal/service/admin_group_duplicate_test.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/admin_service_group_test.go`
- `backend/internal/service/api_key_auth_cache.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/group.go`
- `backend/internal/service/usage_billing.go`
- `backend/internal/service/wire.go`
- `deploy/docker-compose.dev.yml`
- `frontend/src/api/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/user/dashboard/UserDashboardQuickActions.vue`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/en/common.ts`
- `frontend/src/i18n/locales/en/dashboard.ts`
- `frontend/src/i18n/locales/en/index.ts`
- `frontend/src/i18n/locales/en/landing.ts`
- `frontend/src/i18n/locales/zh/admin/accounts.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/i18n/locales/zh/common.ts`
- `frontend/src/i18n/locales/zh/dashboard.ts`
- `frontend/src/i18n/locales/zh/index.ts`
- `frontend/src/i18n/locales/zh/landing.ts`
- `frontend/src/router/index.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/__tests__/GroupsView.codexManifest.spec.ts`
- `frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts`
- `frontend/src/views/admin/__tests__/GroupsView.strictFailover.spec.ts`

发布基准的 354 份 SQL 与 `backend/internal/repository/migrations_runner.go` 共 355 个兼容文件逐字节不变。累计新增 33 份 SQL 均保留；两份 308 迁移以完整文件名独立记账。历史批量生图迁移及 checksum 兼容记录保持，不删除历史表、记录或冻结余额；普通生图、异步单图任务和普通账号/代理池继续保留。

累计剩余风险仍为上一节列出的数据库模式回退边界、VM 桥接版本和 CONNECT 白名单、国产 429 额度快照延迟、EasyPay 非标准回调限制及代理端 DNS 的图片下载边界；本阶段修复的准入与模型保护问题不再列为剩余风险。


### 最终复核验证

- 新增失败回归先复现网关吞掉准入拒绝、旧快照恢复模型保护和补丁注入受管字段；修复后，网关/严格分层/风险路由/429/WS、仓储保护投影、权限与通用调度回归共 121 个顶层测试、358 项测试及子用例全部通过。
- 后端生产构建、全仓 integration 标签编译、受影响 service/repository 包的 `go vet -tags=unit` 通过。integration 使用 `-run=^$ -exec=/bin/true`，仅确认编译，不代表实际数据库集成执行。
- 同步自动化 113 项检查中 111 通过、2 跳过。前阶段前端 444 个文件、3784 项测试及类型检查、lint、构建通过；本阶段未修改前端，未重复执行这些检查。
- 本阶段使用 Go 1.27.3 与前阶段隔离临时 modfile，六个未缓存依赖仍映射到已有缓存版本；仓库依赖与安全升级未修改。正式依赖的完整验证、实际数据库集成及在线漏洞检查仍由外层执行，不将其记为已通过。
- 全仓真实冲突标记、累计空白、改动 Go 格式、功能排除、355 个历史兼容文件字节比对及指定决策 schema 检查通过。真实索引 SHA256 保持 `848694b1ce3e0d6b11e5f084fb855b97cb999a8c242cde38a64a1f016619788c`，不存在未合并索引条目；留下 8 个未暂存文件供外层审查。


## 2026-10-29 集中修复（第 3/3 轮）

本轮完整失败清单只有前端依赖安全审计，独立日志列出 GHSA-83rm-6c8r-g728 的三个 Vue I18n 依赖报告及 GHSA-qr28-fpjm-wq7f 的 URI 解码报告。Release 工作流、打包和发布元数据在外层上一轮已通过，本轮补充补丁输入回归后 14 项测试通过。

- 删除未使用的 `@lobehub/icons` 依赖及其 533 个不可达锁文件快照，消除唯一引入 `decode-uri-component` 的链路。ModelIcon/ProviderIcon 使用仓库内 SVG 路径，图标功能保留；所有保留包的版本、integrity 和依赖边保持原样，仅 shared 引用增加补丁标识。
- npm registry 在本轮沙箱内返回 DNS `EAI_AGAIN`，修复版本未缓存。对 `@intlify/shared@9.14.5` 的全部五种 JS 实现回补 srcdoc 属性名中和，使用 pnpm 哈希锁定补丁；项目既有 ESM runtime 入口使用此实现。三个精确公告例外到期日为 2026-11-28，不使用 `origin: upstream` 的免到期策略。后续迁移到 >=11.4.14 时应一起删除补丁和例外；独立 global/browser 内嵌包不在补丁范围，不能切换到这些入口。
- 两个 Dockerfile 在锁定安装前复制补丁；Release 所执行的 `pnpm build` 包含攻击样例回归。六个攻击样例先全部失败，补丁后通过；普通富文本/插值、56 项 i18n 测试、类型检查、审计校验器 9 项测试、干净临时目录的离线锁定安装及五种 shared 入口攻击复查通过。
- 原始审计报告与当前锁文件的离线核对通过；实际 `pnpm audit --prod --json` 因 `getaddrinfo EAI_AGAIN registry.npmjs.org` 无法完成，不能视为在线审计成功。完整前后端及在线安全验证由外层重跑。

本轮没有新增功能排除或未解决冲突。功能排除检查通过，后端、全部历史 SQL 和校验和兼容记录未修改。没有 fetch、提交、推送、打标签、发布、服务重启或密钥访问。补丁范围与退出计划见 `frontend/patches/README.md`。

本轮最终补充验证：完整 ESLint 和 `pnpm build`（i18n 安全回归、类型构建及 Vite 生产构建）通过；实际生成的 vendor-i18n chunk 包含 srcdoc 中和补丁。最终 diff 空白检查通过，未改动后端及历史数据库兼容文件。


## 2026-10-31 主上游文本与语义冲突审查（第 1/3 轮）

本阶段基于 `origin/main=HEAD=d3901d54b`，处理已经开始的 `upstream/main=d851a3847` 合并；第二上游指针为 `overdraft/sub2api-custom=e4f28e7d6`。累计发布基准是 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`），覆盖候选中的 90 个不同上游非合并提交（主上游 66 个、此前已合入第二上游独有 24 个），并审阅本项目未发布修复。本轮主上游增量为 8 个非合并提交。

本任务仅处理主上游阶段。另已阅读第二上游自 `3604ba2fc` 起的 11 个独有非合并提交及两个合并提交的差异：QQ 站内聊天/知识/查询执行、定时复核与全通过进度、严格层级耗尽重试、模型字段歧义拒绝、删除账号结果过滤、访问控制/分组兜底，以及账号亲和/Codex 身份扩展的后续退役等，留给外层第二上游合并阶段处理，不把尚未合入的增量误报为产品排除或已保留功能。

### 冲突和升级衔接

- 逐段解决 `.gitignore`、`backend/cmd/server/wire_gen.go`、`backend/internal/handler/wire.go`、CORS 实现及测试、前端 package/lockfile 和支付套餐测试共 8 个文件的冲突，未整体选择任一侧。
- 依赖注入保留带配置的代理仓储、GPT Direct、QQ 和定时测试服务，同时接入 MCP 仓储/服务/处理器及全会话撤销；`ProvideUserHandler` 仍接收定时测试服务。批量生图依赖没有恢复。
- CORS 同时保留 `If-Match`、`X-QQBot-Section`、所有 QQ 版本响应头及 `MCP-Protocol-Version`、`MCP-Session-Id`、`WWW-Authenticate`；OAuth 公共端点例外仍只匹配精确路径。已有回归补充断言两类请求头共同存在。
- MCP 默认关闭，配置固定 URL 后启用；仍通过私有路由复用管理 API 的所有权、二次验证、模式门控和审计，OAuth 绑定资源、PKCE、授权范围、只读、撤销及重放检测保持。普通/受限管理密钥认证没有被 MCP 替代；返回结果继续删除密钥和凭证，创建密钥只在成功结果的指定位置返回一次。
- Vue I18n 与消息编译器完整升级到 11.4.16，保留固定 runtime 入口与现有 srcdoc 攻击回归；删除 9.14.5 临时补丁、补丁说明和三个对应审计例外。两份 Dockerfile 同步删除失效补丁 COPY；发布辅助测试支持没有补丁的配置，并继续检查声明与锁文件对应关系。删除 Vitest 中过时的 JIT define；支付套餐继续按真实消息编译断言。
- 锁文件只保留可达依赖，未恢复本项目已删除的未使用 `@lobehub/icons` 依赖；上游 decode-uri-component 安全 override 保留。CI/安全/发布工作流使用 Node 24，两份构建镜像也使用 Node 24。

累计保留 Terra、Grok 目录 CAS/诊断/响应模型审计、DeepSeek 定价/缓存/角色、UTF-16 指纹计费、分页与上传边界、真实图片数量计费、Gemini 目录/声明模型/重试、模型列表二次缓存和 OAuth SSE 聚合。已合入第二上游的严格账号分层、主动定时测试、模型级通过率保护、完整小时统计、监控历史、用户密钥摘要/轮换、账号亲和、QQ 配置/群管理/事务连抽、VM HTTP/WS 独占槽位及上游计费限流保持。本项目权限、二次验证、账号健康、利润控制、票据/指纹隔离及普通代理池保持；主动架构已替代的评分与被动保护不恢复。准入错误不转为等待计划、风险指定账号和原计费分组约束、受管保护投影防旧快照覆盖的回归仍通过。

### 本轮明确核对的排除位置

主上游缺少的 65 个路径全部属于批量生图；已合入第二上游缺少 72 个路径，其中 70 个属于批量生图，两个属于共享账号池。以下专属文件继续缺席；两个混合 Wire 文件只排除批量生图注入，其他服务均保留。

共享账号池：

- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图：

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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/wire.go`

发布基准的 354 份 SQL 与 `backend/internal/repository/migrations_runner.go` 共 355 个兼容文件逐字节不变；累计新增 34 份 SQL 保留，包含本轮 `249_mcp_oauth.sql`，两份 308 仍按完整文件名独立记账。批量生图历史 SQL 与 checksum、历史表、记录和冻结余额均不删除或改写。普通生图和异步单图入口继续保留。

### 累计剩余风险

- 累计迁移 300 的密钥摘要、303/315 的模型级状态键及 318/322–324 的质量配置归档和旧状态表退役不兼容旧进程；自动更新器仅回滚镜像，升级需协调所有实例并保留数据库备份。
- VM 账号的 compact 依赖桥接版本支持，图片 CDN 受 CONNECT 白名单约束；停用或回退桥接前未解绑并恢复原代理会导致相关请求失败。
- 国产频率型 429 分类依赖额度快照，实际额度耗尽至快照刷新期间仍可能按短冷却重试。
- EasyPay 拒绝非标准回调附加字段，使用该类字段的渠道仍需标准回调或主动查询对账。
- 图片下载经环境代理时，本机 DNS 预检不能约束代理端实际解析，仍存在代理解析到私网地址的访问边界。

### 验证和 Git 边界

- 定向配置、MCP 服务/仓储、CORS、机器凭证与 step-up、严格分层、风险准入、受管投影和管理员设置回归：91 个顶层测试、320 项测试/子用例通过。使用 Go 1.27.3 和隔离临时 modfile，将六个未缓存依赖映射到已有缓存版本；仓库 go.mod/go.sum 安全升级未降级。
- 配置、服务、仓储、中间件和管理员处理器五个包的 integration 标签编译通过，使用相同隔离临时 modfile；`-run=^$ -exec=/bin/true` 仅验证编译，不代表实际数据库集成执行。
- 完整前端 ESLint 通过。实际新版本依赖安装、前端测试及完整后端 MCP/构建检查已尝试，Vue I18n 11.4.16 与 MCP SDK 1.4.1 未缓存且 registry DNS 不可达；没有使用旧依赖测试冒充新依赖通过。较宽后端用例中的端口监听失败也已收集。正式依赖的完整验证与真实数据库集成仍须由外层执行。
- 同步自动化 113 项中 111 通过、2 跳过，发布辅助 14 项通过；部署安全、网关环境、运行时资源、简易模式环境与 Caddy 缓存测试通过。Apple container 测试因缺少 macOS `plutil` 未完成。
- 排除检查、355 份历史兼容文件字节比对、改动 Go 语法/格式、完整冲突标记和累计空白检查通过；用临时索引和临时对象目录构建候选检查，真实索引及所有冲突 stage 保持不变，供外层暂存。
- 本轮没有 fetch、git add、commit、push、标签、发布或服务重启。工作树文本及语义冲突已处理，`conflicts` 为空；索引的 U 状态按要求保留。


## 2026-10-31 第二上游集中冲突修复

本轮继续审查 `origin/main=d3901d54b`、已合入主上游 `upstream/main=d851a3847` 和 `overdraft/sub2api-custom=e4f28e7d6`，累计发布基准为 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）。两个上游自发布基准以来共有 101 个去重非合并提交；累计评估包含前次同步后尚未发布的更新，不以本轮新增提交数代替发布范围。`VERSION` 保持本项目的 `2026.10.23`，第二上游的 `FORK_VERSION=v0.2.19-custom.8` 保留。本轮没有 fetch、暂存真实索引、commit、push、标签、发布或重启服务。

### 合并内容和冲突处理

- 逐段核对 56 个冲突路径及 97 个文本冲突块。批量生图的两个 modify/delete 文件按产品规则删除；QQ 举报功能随第二上游退役，旧排队处罚取消，保留历史记录、普通群管理和只读原文查询。其余业务更新完整保留。
- 第二上游的严格分层在同一请求内耗尽明确失败的账号后进入下一层，增加未分层账号末层、真实流读取故障证据和利润门提前准入；基础设施错误、未知状态、并发繁忙不构成跳层依据。保留本项目风险指定账号/分组、准入错误不得转等待计划、请求完整性、健康/并发、Codex 打票和 VM 专属传输边界。
- 保留管理员配置的账号不可用分组回退：先原分组、再分组回退、再个人 API key 回退。分组回退按原分组计费与订阅，个人回退按目标分组计费；利润门采用相同的计费身份。新增风险路由回归，风险指定请求禁止任一通用回退。WebSocket 逐轮核对原分组恢复及 owner-bound 状态。
- 第二上游退役其自定义身份/亲和与计费限流时，和本项目活跃功能发生语义冲突。恢复自动合并删除的账号亲和 CAS、单机多窗口、Mac TLS/会话缓存、HTTPS 代理、票据/Cookie 连接池隔离、智能测试同源传输、计费上限与探测联动；兼容官方 off/device/session/full 模式。新迁移 `329_retire_custom_routing_identity_and_rate_limit.sql` 保留序号但不清除本项目仍生效的策略、种子、探测及上限，回归验证其数据保留和幂等性。
- 保留 Grok 目录与 metadata 的最新行锁列，避免旧管理表单覆盖受管快照；保留投影和保护策略写入限制。受限管理员的成员关系写入按计划锁先于账号锁，继承外层事务；兼容新成员锁接口和本项目旧保护锁适配器，保留完整的受限管理操作场景并补齐真实迁移测试依赖，避免重复建表。
- 设置解析、读取、更新和前端同时保留本项目网关超时、粘性逃逸、平台开关、掩码打票代理与第二上游大陆网页访问开关。大陆限制默认关闭，仅限 `/`、`/login`、`/home`，使用受信代理解析和 no-store 响应，不改变 API 鉴权。
- 完整保留 QQ 站点知识、用户绑定身份的价格/渠道/活动只读查询、管理员试问、工具能力提示、有限重试和执行诊断；保留连续完整成功轮次恢复、模型独立进度、删号结果过滤、上游认证失败测试重试、渲染就绪/安全错误与账号质量手动恢复。
- 新增分组回退 ORM 字段时按排除批量生图后的字段顺序修正 runtime descriptor、迁移索引和 mutation 容量，继续保留普通生图及异步单图任务。移除混合回退测试中重新带入的批量生图结算测试和前端测试中的专属字段；IP 数据版权说明仅去除行尾空格，声明原文保留。

### 累计未发布范围

保留上轮全部 MCP OAuth 管理能力、PKCE/资源/范围/只读/撤销/重放防护、受限管理权限/二次验证/审计、SDK 安全升级和 Vue I18n 11/依赖安全修复；保留 Terra、Grok 目录 CAS/诊断/响应模型审计、DeepSeek 定价/缓存/角色、UTF-16 指纹计费、分页与上传边界、按真实图片数量计费、Gemini 目录/声明模型/重试、模型列表二次缓存、OAuth SSE 聚合。第二上游累计严格分层、主动定时测试、模型级通过率、完整小时统计、监控历史、用户密钥摘要与轮换、普通代理池、QQ 配置/群管理/事务连抽和 VM HTTP/WS 独占槽位继续有效。已由主动架构替代的评分和被动保护不恢复。

发布基准的 354 份 SQL 与 `backend/internal/repository/migrations_runner.go` 逐字节不变；累计新增 39 份 SQL。本轮新增 326–330，329 为本项目策略保留兼容槽位，其他迁移保留第二上游行为。批量生图历史迁移、checksum、历史表、记录及冻结余额均不删除或改写。旧部署未结批量任务仍需用旧版本完成或取消并核对余额；新版本不运行旧队列或自动退款。

### 本轮明确排除路径

与两个上游逐路径核对：主上游缺失的 65 个文件全部属于批量生图；第二上游缺失的 72 个文件中，70 个属于批量生图、2 个属于共享账号池。其他第二上游文件均保留。共享账号池专属路径：

- `backend/migrations/255_retire_shared_account_pool.sql`
- `backend/migrations/retire_shared_account_pool_integration_test.go`

批量生图专属文件及混合文件中的专属代码（混合文件其余实现保留）：

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
- `backend/ent/schema/batch_image_event.go`
- `backend/ent/schema/batch_image_item.go`
- `backend/ent/schema/batch_image_job.go`
- `backend/internal/handler/batch_image_error_message_test.go`
- `backend/internal/handler/batch_image_handler.go`
- `backend/internal/repository/batch_image_download_limiter.go`
- `backend/internal/repository/batch_image_download_limiter_test.go`
- `backend/internal/repository/batch_image_queue.go`
- `backend/internal/repository/batch_image_queue_test.go`
- `backend/internal/repository/batch_image_repo.go`
- `backend/internal/repository/batch_image_repo_integration_test.go`
- `backend/internal/server/middleware/panel_api_key_auth.go`
- `backend/internal/server/middleware/panel_api_key_auth_test.go`
- `backend/internal/service/batch_image.go`
- `backend/internal/service/batch_image_billing_hold.go`
- `backend/internal/service/batch_image_billing_recovery.go`
- `backend/internal/service/batch_image_billing_recovery_test.go`
- `backend/internal/service/batch_image_cleanup.go`
- `backend/internal/service/batch_image_cleanup_test.go`
- `backend/internal/service/batch_image_download.go`
- `backend/internal/service/batch_image_download_test.go`
- `backend/internal/service/batch_image_error_message_test.go`
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
- `docs/BATCH_IMAGE_MVP.md`
- `frontend/src/api/__tests__/batchImage.panel.spec.ts`
- `frontend/src/api/batchImage.ts`
- `frontend/src/composables/__tests__/useBatchImageAccess.retry.spec.ts`
- `frontend/src/composables/useBatchImageAccess.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/views/user/BatchImageGuideView.vue`
- `backend/cmd/server/wire_gen.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/group_unavailable_fallback_test.go`
- `frontend/src/views/admin/__tests__/GroupsView.strictFailover.spec.ts`

### 累计剩余产品风险

- 累计迁移 300 的密钥摘要、303/315 的模型级状态键及 318/322–324 的质量配置归档与旧状态表退役不兼容旧进程；仅回滚镜像不能回退数据库，升级应协调实例并保留备份。
- VM 账号 compact 需要兼容桥接版本，图片 CDN 受 CONNECT 白名单限制；停用桥接前未解绑并恢复原代理会导致相关请求失败。
- 国产 Coding Plan 频率型 429 分类依赖额度快照，真正额度耗尽到快照更新前仍可能按短冷却重试。
- EasyPay 对非标准回调附加字段执行严格拒绝，相关渠道需要标准回调或主动查询对账。
- 图片经环境代理下载时，本机 DNS 预检不能约束代理端最终解析，代理解析到私网仍存在访问边界。

### 验证记录

后端分组回退/原分组计费、严格分层、风险路由、计费限流、Grok 锁定快照与 Codex 指纹定向回归通过；服务/仓储/QQ/迁移包 integration 标签编译通过，未宣称执行真实数据库集成。完整前端 ESLint 通过；QQ 知识/只读查询/能力提示与输入验证的纯逻辑回归通过。后端定向测试使用 Go 1.27.3 及隔离临时 modfile 的六个缓存依赖映射；仅服务/仓储等不依赖 MCP SDK 的测试中去掉未缓存 SDK/jsonschema 的临时根依赖，仓库 go.mod/go.sum 没有降级或删依赖。

修复新旧锁接口兼容后，已重跑分组回退、严格分层、受限管理事务、保护/亲和、自动重试、计费上限、Grok 与指纹的合并回归，服务及仓储包全部通过。渲染服务与测试文件的 Node 语法检查通过。

正式依赖构建、完整前端类型/测试/构建、真实 PostgreSQL 集成及需监听的 QQ 协议用例仍由外层验证；实际尝试记录了未缓存依赖和沙箱监听失败，未使用旧依赖测试冒充正式版本通过。本地 golangci-lint 的解析器不能读取 Go 1.27 导出格式，完整后端 lint 交由外层执行。这些执行限制不列作产品风险。排除检查、真实冲突标记、累计空白检查以及临时索引候选验证均须通过，真实索引和全部 :2/:3 保持不动。

## 2026-10-31 最终双上游兼容审查

最终候选为 `1bd047475`，已经包含主上游 `d851a3847` 和第二上游 `e4f28e7d6`；项目基准为 `origin/main=d3901d54b`。本阶段开始时工作树干净、真实索引没有未合并条目：前面两阶段的合并提交由外层完成，本阶段没有可保留或清除的 :2/:3，也没有修改真实索引。只补充本审查记录，未 fetch、git add、commit、push、打标签、发布或重启服务。

累计范围仍从正式发布 `v2026.10.23`（`23fd587350eb02645d5cac41796f837b2fb957a5`）起算：两个上游共有 101 个去重非合并提交，候选相对发布树涉及 1131 个路径。覆盖此前尚未发布的 MCP OAuth、管理员范围密钥与二次验证、安全依赖升级、Terra/Grok/Gemini/DeepSeek、按实际图片数量计费、UTF-16 计费指纹、分页上传边界、用户密钥摘要/轮换、主动质量管理、QQ 和 VM 更新，以及本项目同步验证、远程失败修复、发布累计窗口和通知兼容修复。

### 最终语义核对

- 再次比较两上游与当前候选的路径、提交记录及关键实现。前一阶段的 56 个冲突路径已经融合；本阶段没有新增文本冲突，也没有发现需要再次修改的业务语义冲突，没有用整文件 ours/theirs 覆盖已有修复。
- 分组回退保持原分组订阅和计费身份，个人密钥回退才切换计费分组；OpenAI 与通用网关利润门均读取同一计费身份。风险指定账号/分组不能触发通用回退，未知故障、owner-bound 状态和严格分层未确认状态不能授权跨组重试。
- 保留本项目活跃账号亲和、Codex 身份/票据隔离、单机多窗口、Mac TLS/HTTPS 代理、计费上限与探测；329 兼容槽位不会清除这些数据。受限管理员成员修改仍在外层事务中按计划、账号的顺序加锁，自动测试保护的撤销与缓存规则保留。
- 第二上游其余功能继续保留：严格分层及同请求耗尽、账号不可用分组回退、歧义 model 字段拒绝、连续完整成功轮次恢复、模型级通过率、自动复核/重试、删号结果过滤、QQ 知识和绑定身份只读查询/试问/诊断、大陆网页限制、普通代理池和 VM 独占槽位。评分与被动保护遵循第二上游后续主动架构替代结果。

### 排除与数据库兼容确认

重新逐路径确认：候选缺少主上游的 65 个文件全部属于批量生图；缺少第二上游的 72 个文件中，70 个属于批量生图，两个为共享账号池专属的 `backend/migrations/255_retire_shared_account_pool.sql` 和 `backend/migrations/retire_shared_account_pool_integration_test.go`。具体批量专属路径及混合文件代码位置沿用紧邻上一节的完整清单，本阶段明确继续排除；没有遗漏其他第二上游文件。普通账号池、普通图片生成和异步单图任务继续保留。

再次逐字节核对发布基准全部 354 份 SQL 和 `backend/internal/repository/migrations_runner.go`，355 个兼容文件全部一致；累计新增 SQL 为 39 份。批量生图历史迁移与 checksum 兼容条目不改写，不删除历史表、记录或冻结余额。已经运行过旧批量队列的部署仍须在升级前用旧版本结束或取消未终结任务并核对余额，新版本不会代为结算。

### 本阶段验证及剩余风险

重新执行的核心服务/仓储合并回归、连续成功轮次/渲染错误分类/model 字段/大陆网页限制回归、QQ 知识/只读查询纯逻辑回归均通过。同步自动化 113 项中 111 项通过、2 项条件跳过；发布工具 14 项通过；部署安全、网关环境、运行资源和 Caddy 检查通过。后端定向回归使用 Go 1.27.3 和既有 `/tmp/sub2-service.mod` 的隔离缓存映射，不代表正式依赖完整验证，仓库依赖清单没有改写。

正式依赖完整构建、完整前端类型/测试/构建、安全审计和真实数据库集成仍由外层完整验证集执行。本阶段已实际尝试正式依赖检查，未缓存模块的只读缓存错误没有被记作通过；扩大 Grok 回归时需监听的用例和缺少 Playwright 的渲染测试也没有被记作通过。这些验证边界不进入产品风险。

累计剩余产品风险与上一阶段一致：密钥摘要和质量状态迁移不支持仅回滚旧镜像；固定 VM 桥接不支持 compact 时保留 404，解绑前停桥接会中断请求；Coding Plan 429 分类存在额度快照窗口；EasyPay 非标准回调字段会被拒绝；环境代理解析目标到私网时，本地 DNS 预检不能约束代理的实际访问。旧批量队列升级前的结束/取消与余额核对要求仍适用。

收尾验证：完整前端 ESLint 通过；定向 Go vet 通过；服务、仓储、QQ、迁移四个包的 integration 标签编译通过（不代表真实数据库集成执行）。渲染服务及两份测试文件的 Node 语法检查通过。临时候选索引的累计空白检查、全树冲突标记扫描、未合并条目检查与功能排除检查通过；真实索引摘要未变。决策 JSON 已按仓库完整 schema 校验通过。
