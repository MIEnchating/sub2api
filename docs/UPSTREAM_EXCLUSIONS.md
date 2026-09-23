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
