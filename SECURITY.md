# 安全策略

## 报告漏洞

本项目重视安全。若你发现安全漏洞，请**不要**在公开 Issue 中披露。

请通过 GitHub 的私有渠道报告：

- 在仓库页面的 **Security → Report a vulnerability** 提交（推荐）
- 或通过维护者的私有联系方式

请在报告中包含：

- 受影响组件与版本
- 漏洞描述与潜在影响
- 复现步骤或 PoC（如适用）

## 支持范围

| 版本 | 支持状态 |
|------|----------|
| 最新 release（`latest`） | 支持 |
| 更早版本 | 不支持 |

## 安全边界

本项目通过 Bearer key 做访问控制。请确保：

- `MCP_API_KEY` / `PUBLIC_MCP_KEY` / `FUND_EDGE_KEY` 不提交到仓库或公开环境
- 生产部署使用强随机密钥并妥善轮换
- 不要将数据库文件（`*.db`）或 `.env` 暴露到公开可访问路径
- Web 登录密码：首次访问 `/setup` 设置（≥12 位且含字母+数字），或通过 `FUND_AUTH_PASSWORD_HASH` 注入 argon2id PHC
- 会话 cookie：HttpOnly + SameSite；HTTPS 部署必须设 `FUND_AUTH_SECURE_COOKIE=true`；`FUND_AUTH_SESSION_TTL` / `FUND_AUTH_SESSION_MAX_AGE` 控制有效期
- 服务端默认下发 HSTS/CSP 等安全响应头；反向代理不应剥离 `Strict-Transport-Security` 与 `Content-Security-Policy`
- 登录失败限流与锁定窗口内置于服务端，无需外部配置

## 响应流程

- 我们会在确认漏洞后尽快评估影响
- 修复会以新 release 发布，并在 `CHANGELOG.md` 记录（避免过度披露可利用细节）
- 根据影响程度，可能发布安全公告
