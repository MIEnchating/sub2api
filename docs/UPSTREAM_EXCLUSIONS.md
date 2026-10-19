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
