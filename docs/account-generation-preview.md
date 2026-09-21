# 账号生成预览

`POST /api/v1/admin/accounts/:id/generate-preview` 位于现有管理员鉴权路由中，用于 Console 的 OAuth SVG 动画和前置问题生成。普通账号测试接口不变。

请求体：

```json
{
  "model_id": "gpt-6-astra",
  "prompt": "生成一个安全的 SVG 动画",
  "reasoning_effort": "low",
  "request_id": "animation-example-1",
  "timeout_seconds": 120
}
```

`model_id` 和 `request_id` 必填，最多 256 字节且不得含控制字符；`prompt` 必填且最多 64 KiB；`reasoning_effort` 为 `none|low|medium|high`；超时为 1–120 秒。请求体最多 128 KiB。

后端按稳定账号 ID 读取 OpenAI OAuth 账号、当前凭据、模型映射和出站代理，固定调用官方 Codex Responses 端点，拒绝重定向。当前不支持凭据影子、Agent Identity 和图片模型；配置了代理但代理缺失时失败，不绕过代理。

上游使用流式返回，服务端收到明确的完成事件后返回标准 JSON：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "account_id": 1223,
    "request_id": "animation-example-1",
    "model": "gpt-6-astra",
    "text": "<svg>...</svg>"
  }
}
```

`model` 仅来自上游完成事件，可为空。空结果、不完整事件流、超出 2 MiB 的响应、敏感凭据回显均失败；部分生成内容不对外返回。错误使用稳定 reason（如 `ACCOUNT_PREVIEW_FAILED`），对可见上游错误脱敏；响应设置 `Cache-Control: no-store`。

该入口不调用普通测试流程，不自动刷新授权，不写账号错误、恢复状态或额度数据，且排除健康与自适应并发的结果统计。不自动重放生成请求或 429；调用方取消会传递到上游。请求仍会消耗上游正常生成用量。Console 负责创建后台任务、限制并发及校验 SVG，不能直接执行模型返回的 HTML/SVG。

部署时先更新 Sub2API，再更新 Console。旧版返回 404/405 时，Console 提示升级，不回退到有账号状态副作用的 `/test` 接口。
