#!/usr/bin/env bash
# OAuth 2.1 / MCP connector smoke test.
#
# Exercises the whole remote-connector path against a running instance exactly as
# ChatGPT would: discovery -> dynamic registration -> authorize with an existing
# dashboard session (consent screen on first use, silent afterwards) -> PKCE token
# exchange -> authenticated MCP call -> refresh rotation, and confirms the legacy
# static-key path is untouched.
#
# Usage: scripts/smoke-oauth.sh <base-url> <password>
#   base-url  e.g. http://localhost:8080  (must match FUND_PUBLIC_BASE_URL)
#   password  the dashboard login password (first-run setup if uninitialized)
set -euo pipefail

BASE="${1:-http://localhost:8080}"
PASSWORD="${2:-ci-smoke-password-1}"
JAR="$(mktemp)"
WORK="$(mktemp -d)"
trap 'rm -rf "$JAR" "$WORK"' EXIT

pass=0
fail=0

ok()   { printf '  \033[32mPASS\033[0m %s\n' "$1"; pass=$((pass+1)); }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=$((fail+1)); }
check(){ if [ "$2" = "$3" ]; then ok "$1 ($3)"; else bad "$1: got '$2', want '$3'"; fi; }
# -i: HTTP field names are case-insensitive, and Go canonicalizes the header to
# "Www-Authenticate" rather than the all-caps spelling used in the RFC examples.
has()  { if printf '%s' "$2" | grep -qi -- "$3"; then ok "$1"; else bad "$1: '$3' not in output"; fi; }
hasnt(){ if printf '%s' "$2" | grep -q -- "$3"; then bad "$1: unexpected '$3'"; else ok "$1"; fi; }

# jsonget <json> <python-expression-on-obj>
jsonget() { printf '%s' "$1" | python3 -c "import sys,json; o=json.load(sys.stdin); print($2)"; }

status() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

echo "== fund-dashboard OAuth/MCP smoke: $BASE"

# ── 1. RFC 9728 protected-resource metadata (both bare and path-suffixed) ──
echo "[1] protected-resource metadata"
for p in /.well-known/oauth-protected-resource /.well-known/oauth-protected-resource/mcp; do
  body="$(curl -fsS "$BASE$p")"
  ctype="$(curl -s -o /dev/null -w '%{content_type}' "$BASE$p")"
  has "$p is JSON" "$ctype" "application/json"
  res="$(jsonget "$body" "o['resource']")"
  check "$p resource" "$res" "$BASE/mcp"
  as="$(jsonget "$body" "o['authorization_servers'][0]")"
  check "$p authorization_servers[0]" "$as" "$BASE"
done

# ── 2. RFC 8414 authorization-server metadata (every alias) ──
echo "[2] authorization-server metadata"
for p in /.well-known/oauth-authorization-server /.well-known/oauth-authorization-server/mcp \
         /.well-known/openid-configuration /.well-known/openid-configuration/mcp; do
  body="$(curl -fsS "$BASE$p")"
  check "$p issuer" "$(jsonget "$body" "o['issuer']")" "$BASE"
  has "$p advertises S256" "$(jsonget "$body" "o['code_challenge_methods_supported']")" "S256"
  check "$p is public-client only" "$(jsonget "$body" "o['token_endpoint_auth_methods_supported'][0]")" "none"
  check "$p advertises CIMD" "$(jsonget "$body" "o['client_id_metadata_document_supported']")" "True"
  hasnt "$p does not offer implicit" "$(jsonget "$body" "o['response_types_supported']")" "token"
done

AUTH_EP="$(jsonget "$(curl -fsS "$BASE/.well-known/oauth-authorization-server")" "o['authorization_endpoint']")"
TOKEN_EP="$(jsonget "$(curl -fsS "$BASE/.well-known/oauth-authorization-server")" "o['token_endpoint']")"
REG_EP="$(jsonget "$(curl -fsS "$BASE/.well-known/oauth-authorization-server")" "o['registration_endpoint']")"

# ── 3. Unknown well-known paths must be JSON 404, never the SPA shell ──
echo "[3] unmounted discovery paths fail loudly"
code="$(status "$BASE/.well-known/oauth-protected-resource/nope")"
check "unknown well-known path is 404" "$code" "404"
body="$(curl -s "$BASE/.well-known/oauth-protected-resource/nope")"
hasnt "unknown well-known path is not the SPA" "$body" "<!doctype html>"

# ── 4. Unauthenticated /mcp must advertise discovery ──
echo "[4] resource server advertises OAuth on 401"
hdrs="$(curl -s -D - -o /dev/null -X POST "$BASE/mcp" -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"
check "unauthenticated /mcp is 401" "$(printf '%s' "$hdrs" | head -1 | awk '{print $2}')" "401"
has "401 carries WWW-Authenticate" "$hdrs" "WWW-Authenticate"
has "challenge points at resource metadata" "$hdrs" "resource_metadata="

# ── 5. Dashboard session (setup on first run, otherwise login) ──
echo "[5] dashboard session"
auth_status="$(curl -fsS "$BASE/api/auth/status")"
init="$(jsonget "$auth_status" "o['initialized']")"
if [ "$init" = "True" ]; then
  code="$(status -c "$JAR" -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
          -H 'X-Fund-Request: fetch' -d "{\"password\":\"$PASSWORD\"}")"
  check "login" "$code" "200"
else
  code="$(status -c "$JAR" -X POST "$BASE/api/auth/setup" -H 'Content-Type: application/json' \
          -H 'X-Fund-Request: fetch' -d "{\"password\":\"$PASSWORD\"}")"
  check "first-run setup" "$code" "201"
fi

# ── 6. Dynamic client registration (RFC 7591) ──
echo "[6] dynamic client registration"
reg="$(curl -fsS -X POST "$REG_EP" -H 'Content-Type: application/json' \
       -d '{"client_name":"smoke connector","redirect_uris":["https://chatgpt.com/oauth/mcp/callback"],"grant_types":["authorization_code","refresh_token"],"token_endpoint_auth_method":"none"}')"
CLIENT_ID="$(jsonget "$reg" "o['client_id']")"
has "client_id issued" "$CLIENT_ID" "."
hasnt "no client secret issued" "$reg" "client_secret"

badreg="$(status -X POST "$REG_EP" -H 'Content-Type: application/json' \
          -d '{"redirect_uris":["http://evil.example/cb"]}')"
check "insecure redirect_uri rejected" "$badreg" "400"

# ── 7. Authorize with PKCE (first-use consent, then silence) ──
echo "[7] authorize (PKCE S256)"
VERIFIER="smoke-verifier-0123456789abcdefghijklmnopqrstuvwxyz-ABC"
CHALLENGE="$(printf '%s' "$VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')"
REDIRECT="https://chatgpt.com/oauth/mcp/callback"
urlencode() { python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1],safe=''))" "$1"; }

# redirect_uri is appended separately so the negative case below can substitute a
# different value: Go's query.Get returns the FIRST occurrence, so duplicating the
# parameter would silently keep testing the legitimate URI.
Q="client_id=$CLIENT_ID&response_type=code&scope=fund.read&state=smoke-state&code_challenge=$CHALLENGE&code_challenge_method=S256"

AUTH_URL="$AUTH_EP?$Q&redirect_uri=$(urlencode "$REDIRECT")"
codefrom() { python3 -c "import urllib.parse,sys;print(urllib.parse.parse_qs(urllib.parse.urlparse(sys.argv[1]).query).get('code',[''])[0])" "$1"; }

# Registration is open (RFC 7591), so an unknown client_id proves nothing: the
# first authorization must put the connector's name in front of the owner.
screen="$(curl -s -b "$JAR" "$AUTH_URL")"
has "first use renders the consent screen" "$screen" 'name="consent_token"'
hasnt "consent screen carries no script" "$screen" "<script"
CONSENT_TOKEN="$(python3 -c "import re,sys;m=re.search(r'name=\"consent_token\" value=\"([^\"]+)\"',sys.argv[1]);print(m.group(1) if m else '')" "$screen")"
has "one-time consent token issued" "$CONSENT_TOKEN" "."

loc="$(curl -s -o /dev/null -w '%{redirect_url}' -b "$JAR" -X POST "$BASE/oauth/consent" \
       --data-urlencode "consent_token=$CONSENT_TOKEN" --data-urlencode "decision=approve")"
has "approval redirected to the client" "$loc" "$REDIRECT"
CODE="$(codefrom "$loc")"
has "authorization code issued" "$CODE" "."
has "state echoed" "$loc" "state=smoke-state"
hasnt "no token in the front channel" "$loc" "access_token"

# The consent token is single-use: replaying it must not mint a second code.
replayconsent="$(status -b "$JAR" -X POST "$BASE/oauth/consent" \
                 --data-urlencode "consent_token=$CONSENT_TOKEN" --data-urlencode "decision=approve")"
check "consent token is single-use" "$replayconsent" "400"

# Steady state, i.e. what the owner actually experiences from now on: an approved
# connector needs no click, so logging in is the whole interaction.
loc2="$(curl -s -o /dev/null -w '%{redirect_url}' -b "$JAR" "$AUTH_URL")"
has "second authorization is silent" "$loc2" "$REDIRECT"
hasnt "second authorization shows no consent screen" "$loc2" "consent"
has "second authorization still issues a code" "$(codefrom "$loc2")" "."

# An unregistered redirect target must never be redirected to.
evil="$(curl -s -o /dev/null -w '%{redirect_url}' -b "$JAR" "$AUTH_EP?$Q&redirect_uri=$(urlencode "https://evil.example/cb")")"
check "unregistered redirect_uri does not redirect" "$evil" ""

# ── 8. Token exchange (no client_id, as OpenAI's connector sends it) ──
echo "[8] token exchange"
tok="$(curl -fsS -X POST "$TOKEN_EP" -H 'Content-Type: application/x-www-form-urlencoded' \
       --data-urlencode "grant_type=authorization_code" --data-urlencode "code=$CODE" \
       --data-urlencode "redirect_uri=$REDIRECT" --data-urlencode "code_verifier=$VERIFIER" \
       --data-urlencode "resource=$BASE/mcp")"
ACCESS="$(jsonget "$tok" "o['access_token']")"
REFRESH="$(jsonget "$tok" "o['refresh_token']")"
check "token_type" "$(jsonget "$tok" "o['token_type']")" "Bearer"
check "granted scope" "$(jsonget "$tok" "o['scope']")" "fund.read"
has "access token is a JWS" "$ACCESS" '\.'

replay="$(status -X POST "$TOKEN_EP" -d "grant_type=authorization_code&code=$CODE&redirect_uri=$REDIRECT&code_verifier=$VERIFIER")"
check "authorization code is single-use" "$replay" "400"

badverifier="$(status -X POST "$TOKEN_EP" -d "grant_type=authorization_code&code=x&code_verifier=y")"
check "bogus code rejected" "$badverifier" "400"

# ── 9. Authenticated MCP call ──
echo "[9] MCP with the OAuth access token"
tools="$(curl -fsS -X POST "$BASE/mcp" -H "Authorization: Bearer $ACCESS" \
         -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"
has "tools/list works" "$tools" "get_portfolio_summary"
hasnt "read scope hides write tools" "$tools" "add_transaction"
hasnt "read scope hides maintenance tools" "$tools" "crawl_nav"

call="$(curl -fsS -X POST "$BASE/mcp" -H "Authorization: Bearer $ACCESS" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_portfolio_summary","arguments":{"portfolio_id":1}}}')"
has "tools/call returns a content array" "$call" '"content"'
has "tools/call returns structuredContent" "$call" '"structuredContent"'
has "tools/call returns isError" "$call" '"isError"'

bogus="$(status -X POST "$BASE/mcp" -H 'Authorization: Bearer not.a.jwt' \
         -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"
check "garbage token rejected" "$bogus" "401"

# ── 10. Refresh rotation ──
echo "[10] refresh token rotation"
rot="$(curl -fsS -X POST "$TOKEN_EP" -d "grant_type=refresh_token&refresh_token=$REFRESH")"
REFRESH2="$(jsonget "$rot" "o['refresh_token']")"
if [ -n "$REFRESH2" ] && [ "$REFRESH2" != "$REFRESH" ]; then ok "refresh token rotated"; else bad "refresh token was not rotated"; fi
reused="$(status -X POST "$TOKEN_EP" -d "grant_type=refresh_token&refresh_token=$REFRESH")"
check "consumed refresh token rejected" "$reused" "400"

# ── 11. Legacy static-key contract is untouched ──
echo "[11] legacy static keys still work"
if [ -n "${MCP_API_KEY:-}" ]; then
  legacy="$(curl -fsS -X POST "$BASE/mcp" -H "Authorization: Bearer $MCP_API_KEY" \
            -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"
  has "operator key lists tools" "$legacy" "get_portfolio_summary"
  has "operator key still sees write tools" "$legacy" "add_transaction"
fi
if [ -n "${PUBLIC_MCP_KEY:-}" ]; then
  pub="$(curl -fsS -X POST "$BASE/mcp" -H "Authorization: Bearer $PUBLIC_MCP_KEY" \
         -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"
  has "public key lists tools" "$pub" "get_portfolio_summary"
  hasnt "public key cannot see write tools" "$pub" "add_transaction"
fi

echo
echo "== smoke result: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
