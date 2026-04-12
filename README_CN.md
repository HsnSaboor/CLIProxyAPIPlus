# CLIProxyAPI Plus

[English](README.md) | 中文

这是 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 的 Plus 版本，在原有基础上增加了第三方供应商的支持。

所有的第三方供应商支持都由第三方社区维护者提供，CLIProxyAPI 不提供技术支持。如需取得支持，请与对应的社区维护者联系。

## 支持的供应商

| 供应商 | 标志 | 说明 |
|---|---|---|
| Cline | `--cline-login` | 通过 Cline 扩展的 OAuth 设备流程 |
| CodeBuddy (CN) | `--codebuddy-login` | 通过 `copilot.tencent.com` 的 OAuth (codebuddy.cn) |
| CodeBuddy 国际版 | `--codebuddy-intl-login` | 通过 `www.codebuddy.ai` 的 OAuth |

> 完整的内置供应商列表（Claude、Codex、Gemini、Cursor 等），请参阅[主线 README](https://github.com/router-for-me/CLIProxyAPI)。

### CodeBuddy 国际版

`--codebuddy-intl-login` 标志针对 `www.codebuddy.ai` 进行身份验证，而非默认的 `copilot.tencent.com` 端点。国际版使用相同的 API 端点和响应格式，仅基础 URL 和默认域名不同。令牌以 `type: "codebuddy-intl"` 和 `base_url` 元数据存储，以便执行器将请求路由到正确的后端。

## 贡献

该项目仅接受第三方供应商支持的 Pull Request。任何非第三方供应商支持的 Pull Request 都将被拒绝。

如果需要提交任何非第三方供应商支持的 Pull Request，请提交到[主线](https://github.com/router-for-me/CLIProxyAPI)版本。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。
