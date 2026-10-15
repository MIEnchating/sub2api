# 双上游定时同步

仓库使用本机定时任务同步两个上游：

- `upstream/main`：全部合并，已退役的批量生图功能不得重新引入。
- `overdraft/sub2api-custom`：全部合并，但排除共享账号池、批量生图及其专属支持代码。

同步脚本是 [scripts/upstream-sync.sh](../scripts/upstream-sync.sh)。它在独立 Git worktree 中完成合并，由 Codex 做完整语义审查，然后一次运行完整验证集：后端普通/单元/集成测试、`go vet`、golangci-lint、漏洞检查和生产构建，以及前端依赖锁定安装、ESLint、类型检查、完整 Vitest、依赖审计、i18n/生产构建和部署脚本测试。

每轮验证会继续执行并收集所有失败，不会在第一个错误处停止。存在失败时，脚本把整批失败日志一次性交给 Codex 集中修复，再从头运行完整验证；默认最多修复三轮。仍失败时不推送，并发送包含具体测试名、错误、直接原因和修复轮次的邮件。成功合并并一次性推送后发送成功报告。

冲突审查默认最多自动修复三轮。每轮必须同时通过审查输出结构、真实冲突标记、空白错误和功能排除检查，不能仅凭 Codex 返回 `resolved` 就接受。检查使用临时 Git 索引，失败后保留原索引中的双方版本供下一轮继续修复；通过后才正式暂存。没有文本冲突但排除规则或内容检查失败的合并，也会进入修复流程。

每次候选或修复提交成功推送到远端主分支后，立即快进干净且没有进行中 Git 操作的本地主分支。任务退出时，无论发布成功、发布失败或没有新更新，都重新获取远端主分支并执行安全收尾：只清理本任务工作树目录内、使用 `automation/sub2api-upstream-sync-` 前缀、提交已被远端主分支包含的干净工作树，以及不再挂载工作树的已合入同前缀分支。工作树内被 Git 忽略的构建产物随之清理。未推送的修复、未提交文件、冲突现场、锁定工作树和其他分支保留；远端获取失败时不删除。试运行不执行收尾。发布失败仍保留独立的待发布状态，不因 Git 收尾成功而改报发布成功。收尾详情写入本轮日志。

最终提交保留 `origin/main`、`upstream/main` 和 `overdraft/sub2api-custom` 三个父提交。这样既能从最终文件树中排除共享账号池和批量生图，又能记录两个上游版本已经处理，避免后续任务重复合并同一批提交。

脚本使用 `flock` 防止任务重叠，并要求主工作区干净。任务不会重启本地服务。合并通过完整验证后，脚本按 new-api 的发布规则评估更新规模：默认至少包含 3 个上游提交、8 个变更文件和 150 行差异，且 Codex 结构化审查确认没有剩余风险时，才生成版本说明并推送下一个可用的日期标签以触发 GitHub Release；未达到门槛时只推送代码。SMTP 默认复用本机 new-api PostgreSQL 中已有的邮件配置，凭据不会写入仓库。

发布标签以北京时间当天的 `vYYYY.M.D` 为起点。若该标签已经存在，脚本向后查找下一个未占用的有效日期标签，不会移动或覆盖已有标签。发布开关和门槛可通过 `SUB2API_RELEASE_*` 配置调整。

排除范围详见 [上游合并排除列表](UPSTREAM_EXCLUSIONS.md)。每轮完整验证和最终合并审查后都会运行排除规则检查，修复时也不得重新引入已排除功能。

## 本机安装

远程工作流连续三次查询不到目标提交的运行记录时，脚本会校验远端分支/标签仍指向候选提交，并补触发一次 `workflow_dispatch`；CI 和 Security Scan 均支持此入口。因长期不活动而停用的工作流会尝试重新启用。补触发成功后最多等待五分钟出现运行记录，已有运行仍使用正常的构建超时。成功提交触发请求不等于检查通过，最终仍要求目标提交的工作流成功。

若 GitHub 明确返回 `Actions has been disabled for this repository`，立即停止本轮远程等待，保留待发布状态并报告平台禁用，不再依次空等两项检查各 90 分钟，也不让 Codex 修改代码来“修复”平台限制。仓库设置接口的 `enabled=true` 不能替代实际触发结果。仓库所有者须检查 Actions 页的禁用提示；若重新启用无效，需由 GitHub Support 解除平台限制，之后定时任务会重新尝试发布恢复。

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

`SUB2API_GO_IMAGE` 留空时，每个后端检查从候选 `backend/go.mod` 读取完整 Go 版本，使用对应的 `golang:<版本>` 镜像；修复改变 Go 版本后会重新选择。自定义镜像必须满足候选的最低版本要求。容器设置 `GOTOOLCHAIN=local`，不会隐式下载其他工具链。lint 和漏洞检查工具按 Go 版本及平台隔离缓存，lint 版本与 CI 对齐。

从旧同步脚本升级时，应清除配置中固定的 `SUB2API_GO_IMAGE=golang:1.27.0`（或改用满足候选要求的镜像）。已经运行的旧脚本不会因候选工作树被修改而重载函数或镜像变量；应保留候选，在使用新版脚本及配置的验证环境中重新执行完整验证，不能降低 `go.mod` 的安全版本要求。

## 手动试运行

不会提交或推送：

```bash
SUB2API_DRY_RUN=true /bin/bash scripts/upstream-sync.sh
```

只验证 SMTP 投递和中文报告格式：

```bash
/bin/bash scripts/upstream-sync.sh --test-email
```
