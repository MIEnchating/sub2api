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
