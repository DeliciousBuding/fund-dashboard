package httpapi

import (
	"html/template"
	"log/slog"
	"net/http"
	"sync"

	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
)

// scopeView is one granted scope rendered on the consent screen.
type scopeView struct {
	Name        string
	Title       string
	Description string
}

// consentView is the consent screen payload.
type consentView struct {
	ClientName   string
	Issuer       string
	Scopes       []scopeView
	ConsentToken string
	State        string
	AutoApproved bool
}

// errorView is the local error page payload. It is rendered instead of
// redirecting whenever the redirect target has not been verified.
type errorView struct {
	Issuer      string
	Code        string
	Description string
	ClientName  string
}

// The consent pages are the only server-rendered HTML in this service; every
// other surface is the SPA or JSON. Templates are parsed once because they are
// constant, and the CSS is a separate same-origin file because the app CSP is
// style-src 'self' (inline <style> and style="" attributes are both blocked).
var (
	oauthTemplateOnce sync.Once
	oauthTemplates    *template.Template
)

const oauthConsentTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>授权访问 · Fund Dashboard</title>
<link rel="stylesheet" href="/oauth/assets/consent.css">
</head>
<body>
<main class="card">
  <div class="brand" aria-hidden="true">&#9672;</div>
  <h1>授权访问持仓中枢</h1>
  <p class="lead"><strong>{{ .ClientName }}</strong> 请求以你的身份访问 Fund Dashboard 的 MCP 接口。</p>
  <section class="scopes">
    <h2>它将获得以下权限</h2>
    <ul>
      {{- range .Scopes }}
      <li>
        <span class="scope-title">{{ .Title }}</span>
        <code>{{ .Name }}</code>
        {{- if .Description }}
        <span class="scope-desc">{{ .Description }}</span>
        {{- end }}
      </li>
      {{- end }}
    </ul>
  </section>
  <p class="note">授权后对方会得到一个短期访问令牌，可随时在设置页撤销。你的登录密码不会共享给对方。</p>
  <form method="post" action="/oauth/consent" class="actions">
    <input type="hidden" name="consent_token" value="{{ .ConsentToken }}">
    {{- if .State }}
    <input type="hidden" name="state" value="{{ .State }}">
    {{- end }}
    <button type="submit" name="decision" value="approve" class="primary">授权</button>
    <button type="submit" name="decision" value="deny" class="ghost">拒绝</button>
  </form>
</main>
</body>
</html>
`

const oauthErrorTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>授权失败 · Fund Dashboard</title>
<link rel="stylesheet" href="/oauth/assets/consent.css">
</head>
<body>
<main class="card">
  <div class="brand danger" aria-hidden="true">!</div>
  <h1>无法完成授权</h1>
  <p class="lead">{{ .Description }}</p>
  <dl class="detail">
    <dt>错误码</dt><dd><code>{{ .Code }}</code></dd>
    <dt>服务</dt><dd><code>{{ .Issuer }}</code></dd>
    {{- if .ClientName }}
    <dt>客户端</dt><dd>{{ .ClientName }}</dd>
    {{- end }}
  </dl>
  <p class="note">这条错误没有回跳到客户端，因为客户端身份或回调地址未能通过校验。请回到发起授权的应用重新发起，或检查其 MCP 连接器配置。</p>
  <p class="actions"><a class="primary" href="/">返回持仓中枢</a></p>
</main>
</body>
</html>
`

// oauthConsentCSS is served at /oauth/assets/consent.css.
const oauthConsentCSS = `:root {
  color-scheme: dark;
  --bg: #0b0d10;
  --surface: #14181d;
  --border: #232a32;
  --fg: #e6e9ec;
  --fg-2: #a4adb8;
  --fg-3: #7c8794;
  --accent: #5eead4;
  --accent-fg: #06231f;
  --danger: #f87171;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px 16px;
  background: var(--bg);
  color: var(--fg);
  font: 15px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
        "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
}
.card {
  width: 100%;
  max-width: 460px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px 28px;
}
.brand {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: #1b2229;
  color: var(--accent);
  font-size: 20px;
  margin-bottom: 18px;
}
.brand.danger { color: var(--danger); }
h1 { font-size: 20px; font-weight: 600; margin: 0 0 8px; }
h2 { font-size: 13px; font-weight: 600; margin: 0 0 10px; color: var(--fg-2); }
.lead { margin: 0 0 22px; color: var(--fg-2); font-size: 14px; }
.scopes { border-top: 1px solid var(--border); padding-top: 18px; }
.scopes ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 14px; }
.scopes li { display: grid; gap: 4px; }
.scope-title { font-weight: 500; }
.scope-desc { color: var(--fg-3); font-size: 13px; }
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--fg-3);
  background: #1b2229;
  border-radius: 5px;
  padding: 1px 6px;
  justify-self: start;
}
.note { margin: 20px 0 0; color: var(--fg-3); font-size: 12.5px; }
.detail { margin: 18px 0 0; display: grid; grid-template-columns: 72px 1fr; gap: 8px 12px; font-size: 13px; }
.detail dt { color: var(--fg-3); }
.detail dd { margin: 0; }
.actions { display: flex; gap: 10px; margin-top: 24px; }
button, a.primary {
  appearance: none;
  border: 0;
  border-radius: 10px;
  padding: 11px 18px;
  font: inherit;
  font-weight: 500;
  cursor: pointer;
  text-decoration: none;
  text-align: center;
}
button.primary, a.primary { background: var(--accent); color: var(--accent-fg); }
button.ghost { background: transparent; color: var(--fg-2); border: 1px solid var(--border); }
button.primary:hover, a.primary:hover { opacity: .9; }
`

func oauthTemplateSet() *template.Template {
	oauthTemplateOnce.Do(func() {
		oauthTemplates = template.Must(template.New("consent").Parse(oauthConsentTemplate))
		template.Must(oauthTemplates.New("error").Parse(oauthErrorTemplate))
	})
	return oauthTemplates
}

func renderOAuthConsent(w http.ResponseWriter, r *http.Request, view consentView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := oauthTemplateSet().ExecuteTemplate(w, "consent", view); err != nil {
		slog.Warn("oauth consent render failed", "request_id", RequestIDFromContext(r.Context()), "error", err.Error())
	}
}

func renderOAuthError(w http.ResponseWriter, r *http.Request, issuer string, failure *oauth.Failure) {
	view := errorView{Issuer: issuer, Code: "server_error", Description: "授权请求无法处理。"}
	if failure != nil {
		view.Code = failure.Code.Error()
		if failure.Description != "" {
			view.Description = failure.Description
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusBadRequest
	if failure != nil && failure.Status != 0 {
		status = failure.Status
	}
	w.WriteHeader(status)
	if err := oauthTemplateSet().ExecuteTemplate(w, "error", view); err != nil {
		slog.Warn("oauth error page render failed", "request_id", RequestIDFromContext(r.Context()), "error", err.Error())
	}
}

func handleOAuthConsentCSS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(oauthConsentCSS))
	}
}
