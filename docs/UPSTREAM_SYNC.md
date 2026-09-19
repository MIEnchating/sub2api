# 双上游定时同步

仓库使用本机定时任务同步两个上游：

- `upstream/main`：全部合并。
- `overdraft/sub2api-custom`：全部合并，但排除共享账号池及其专属支持代码。

同步脚本是 [scripts/upstream-sync.sh](../scripts/upstream-sync.sh)。它在独立 Git worktree 中完成合并，由 Codex 做完整语义审查，然后执行后端测试、`go vet`、前端 lint、类型检查、i18n 检查和生产构建。所有检查通过后只创建一个合并提交并推送一次；失败时不推送。

脚本使用 `flock` 防止任务重叠，并要求主工作区干净。任务不会重启本地服务，也不会自动发布版本。

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
