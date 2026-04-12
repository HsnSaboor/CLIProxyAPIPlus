# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This is the Plus version of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; CLIProxyAPI does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

The Plus release stays in lockstep with the mainline features.

## Supported Providers

| Provider | Flag | Notes |
|---|---|---|
| Cline | `--cline-login` | OAuth device flow via Cline extension |
| CodeBuddy (CN) | `--codebuddy-login` | OAuth via `copilot.tencent.com` (codebuddy.cn) |
| CodeBuddy International | `--codebuddy-intl-login` | OAuth via `www.codebuddy.ai` |

> For the full list of built-in providers (Claude, Codex, Gemini, Cursor, etc.), see the [mainline README](https://github.com/router-for-me/CLIProxyAPI).

### CodeBuddy International

The `--codebuddy-intl-login` flag authenticates against `www.codebuddy.ai` instead of the default `copilot.tencent.com` endpoint. The international variant uses identical API endpoints and response formats — only the base URL and default domain differ. Tokens are stored with `type: "codebuddy-intl"` and `base_url` metadata so the executor routes requests to the correct backend.

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
