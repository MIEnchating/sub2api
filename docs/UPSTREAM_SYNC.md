# 双上游定时同步

仓库使用本机定时任务同步两个上游：

- `upstream/main`：全部合并。
- `overdraft/sub2api-custom`：全部合并，但排除共享账号池及其专属支持代码。

同步脚本是 [scripts/upstream-sync.sh](../scripts/upstream-sync.sh)。它在独立 Git worktree 中完成合并，由 Codex 做完整语义审查，然后一次运行完整验证集：后端普通/单元/集成测试、`go vet`、golangci-lint、漏洞检查和生产构建，以及前端依赖锁定安装、ESLint、类型检查、完整 Vitest、依赖审计、i18n/生产构建和部署脚本测试。

每轮验证会继续执行并收集所有失败，不会在第一个错误处停止。存在失败时，脚本把整批失败日志一次性交给 Codex 集中修复，再从头运行完整验证；默认最多修复三轮。仍失败时不推送，并发送包含具体测试名、错误、直接原因和修复轮次的邮件。成功合并并一次性推送后发送成功报告。

最终提交保留 `origin/main`、`upstream/main` 和 `overdraft/sub2api-custom` 三个父提交。这样既能从最终文件树中排除共享账号池，又能记录两个上游版本已经处理，避免后续任务重复合并同一批提交。

脚本使用 `flock` 防止任务重叠，并要求主工作区干净。任务不会重启本地服务。合并通过完整验证后，脚本按 new-api 的发布规则评估更新规模：默认至少包含 3 个上游提交、8 个变更文件和 150 行差异，且 Codex 结构化审查确认没有剩余风险时，才生成版本说明并推送下一个可用的日期标签以触发 GitHub Release；未达到门槛时只推送代码。SMTP 默认复用本机 new-api PostgreSQL 中已有的邮件配置，凭据不会写入仓库。

发布标签以北京时间当天的 `vYYYY.M.D` 为起点。若该标签已经存在，脚本向后查找下一个未占用的有效日期标签，不会移动或覆盖已有标签。发布开关和门槛可通过 `SUB2API_RELEASE_*` 配置调整。

## 本机安装

当前机器使用 UTC 时区，已将 sub2api 任务安排在每天 `17:30 UTC`（北京时间次日 `01:30`），避开 new-api 的 `16:30 UTC` 任务：

```cron
30 17 * * * /bin/bash /root/workspace/sub2api/scripts/upstream-sync.sh >> /root/workspace/automation/sub2api-upstream-sync/cron-runner.log 2>&1
```

运行目录和日志目录位于仓库之外，不会污染 Git 工作区。首次安装前创建：

```bash
mkdir -p /root/workspace/automation/sub2api-upstream-sync
cp scripts/upstream-sync.env.example /root/workspace/automation/sub2api-upstream-sync/config.env
chmod 600 /root/workspace/automation/sub2api-upstream-sync/config.env
```

环境变量可通过 cron 的 `config.env` 注入；脚本内置当前仓库和三个 remote 的默认值。

## 手动试运行

不会提交或推送：

```bash
SUB2API_DRY_RUN=true /bin/bash scripts/upstream-sync.sh
```

只验证 SMTP 投递和中文报告格式：

```bash
/bin/bash scripts/upstream-sync.sh --test-email
```
