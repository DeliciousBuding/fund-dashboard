package oauth

import (
	"context"
	"net"
	"testing"
)

func TestRegisterClientRedirectURIValidation(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	for _, uri := range []string{
		"https://chatgpt.com/oauth/callback",
		"http://127.0.0.1:8976/callback",
		"http://localhost:3000/cb",
	} {
		if _, _, err := svc.RegisterClient(ctx, RegisterClientRequest{RedirectURIs: []string{uri}}); err != nil {
			t.Fatalf("valid redirect_uri %q rejected: %v", uri, err)
		}
	}

	invalid := map[string]string{
		"plain http":    "http://client.example/cb",
		"fragment":      "https://client.example/cb#frag",
		"relative":      "/callback",
		"custom scheme": "myapp://callback",
		"empty":         "",
		"wildcard host": "https://*.example.com/cb",
		"userinfo":      "https://user:pass@client.example/cb",
	}
	for name, uri := range invalid {
		_, _, err := svc.RegisterClient(ctx, RegisterClientRequest{RedirectURIs: []string{uri}})
		if err == nil {
			t.Fatalf("%s redirect_uri %q was accepted", name, uri)
		}
		failure, ok := asFailure(err)
		if !ok {
			t.Fatalf("%s: error is not an OAuth Failure: %v", name, err)
		}
		if failure.Code != ErrInvalidRedirectURI {
			t.Fatalf("%s: error code = %v, want invalid_redirect_uri", name, failure.Code)
		}
	}

	if _, _, err := svc.RegisterClient(ctx, RegisterClientRequest{}); err == nil {
		t.Fatal("registration without redirect_uris was accepted")
	}
	if _, _, err := svc.RegisterClient(ctx, RegisterClientRequest{
		RedirectURIs:            []string{"https://client.example/cb"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}); err == nil {
		t.Fatal("confidential client registration was accepted")
	}
}

func TestRegisterClientIsPublicAndPersisted(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()
	client, response, err := svc.RegisterClient(ctx, RegisterClientRequest{
		ClientName:   "ChatGPT connector",
		RedirectURIs: []string{"https://chatgpt.com/cb", "https://chatgpt.com/cb"},
		GrantTypes:   []string{"authorization_code", "implicit", "refresh_token"},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if client.TokenEndpointAuthMethod != "none" || response.TokenEndpointAuthMethod != "none" {
		t.Fatalf("client is not public: %q", client.TokenEndpointAuthMethod)
	}
	if len(client.RedirectURIs) != 1 {
		t.Fatalf("duplicate redirect_uris not deduped: %v", client.RedirectURIs)
	}
	// "implicit" must never survive: OAuth 2.1 removes it.
	for _, grant := range client.GrantTypes {
		if grant == "implicit" {
			t.Fatalf("implicit grant was accepted: %v", client.GrantTypes)
		}
	}
	reloaded, err := svc.ResolveClient(ctx, client.ID)
	if err != nil {
		t.Fatalf("resolve persisted client: %v", err)
	}
	if reloaded.Name != "ChatGPT connector" || len(reloaded.RedirectURIs) != 1 {
		t.Fatalf("client did not round-trip: %+v", reloaded)
	}
	count, err := svc.store.CountClients(ctx)
	if err != nil || count != 1 {
		t.Fatalf("client count = %d, err = %v", count, err)
	}
}

func TestValidateRedirectURIExactMatchAndLoopbackPort(t *testing.T) {
	client := &Client{RedirectURIs: []string{"https://chatgpt.com/cb", "http://127.0.0.1:8976/cb"}}
	if _, err := ValidateRedirectURI(client, "https://chatgpt.com/cb"); err != nil {
		t.Fatalf("exact match rejected: %v", err)
	}
	if _, err := ValidateRedirectURI(client, "https://chatgpt.com/cb/extra"); err == nil {
		t.Fatal("path-extended redirect_uri accepted")
	}
	if _, err := ValidateRedirectURI(client, "https://evil.example/cb"); err == nil {
		t.Fatal("unregistered redirect_uri accepted")
	}
	if got, err := ValidateRedirectURI(client, "http://127.0.0.1:9999/cb"); err != nil || got != "http://127.0.0.1:9999/cb" {
		t.Fatalf("loopback port variation rejected (RFC 8252 7.3): got=%q err=%v", got, err)
	}
	if _, err := ValidateRedirectURI(client, "https://chatgpt.com:8443/cb"); err == nil {
		t.Fatal("https port variation accepted")
	}
	single := &Client{RedirectURIs: []string{"https://chatgpt.com/cb"}}
	if got, err := ValidateRedirectURI(single, ""); err != nil || got != "https://chatgpt.com/cb" {
		t.Fatalf("omitted redirect_uri not defaulted: got=%q err=%v", got, err)
	}
	multi := &Client{RedirectURIs: []string{"https://a.example/cb", "https://b.example/cb"}}
	if _, err := ValidateRedirectURI(multi, ""); err == nil {
		t.Fatal("ambiguous omitted redirect_uri accepted")
	}
}

func TestCIMDRejectsNonAllowlistedHostWithoutFetching(t *testing.T) {
	svc := newTestService(t, nil)
	fetched := false
	svc.cimd.fetchFn = func(context.Context, string) ([]byte, error) {
		fetched = true
		return []byte(`{"redirect_uris":["https://evil.example/cb"]}`), nil
	}
	// An attacker-supplied client_id must never cause an outbound request: that
	// is precisely the SSRF this allowlist exists to prevent.
	for _, clientID := range []string{
		"https://evil.example/client.json",
		"https://chatgpt.com.evil.example/client.json",
		"https://evil.example/chatgpt.com/client.json",
		"https://169.254.169.254/latest/meta-data/",
		"https://localhost/client.json",
		"https://127.0.0.1/client.json",
	} {
		if _, err := svc.ResolveClient(context.Background(), clientID); err == nil {
			t.Fatalf("non-allowlisted client_id %q resolved", clientID)
		}
	}
	if fetched {
		t.Fatal("a non-allowlisted client_id triggered a fetch (SSRF)")
	}
	if _, err := svc.ResolveClient(context.Background(), "opaque-client-id"); err == nil {
		t.Fatal("unknown opaque client_id resolved")
	}
	if _, err := svc.ResolveClient(context.Background(), ""); err == nil {
		t.Fatal("empty client_id resolved")
	}
}

func TestCIMDResolvesAllowlistedDocument(t *testing.T) {
	svc := newTestService(t, nil)
	var requested string
	svc.cimd.fetchFn = func(_ context.Context, rawURL string) ([]byte, error) {
		requested = rawURL
		return []byte(`{"client_name":"ChatGPT","redirect_uris":["https://chatgpt.com/oauth/mcp/callback"]}`), nil
	}
	clientID := "https://chatgpt.com/oauth/client/metadata/abc"
	client, err := svc.ResolveClient(context.Background(), clientID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if requested != clientID {
		t.Fatalf("fetched %q, want %q", requested, clientID)
	}
	if !client.MetadataDocument || client.Name != "ChatGPT" {
		t.Fatalf("unexpected client: %+v", client)
	}
	if len(client.RedirectURIs) != 1 || client.RedirectURIs[0] != "https://chatgpt.com/oauth/mcp/callback" {
		t.Fatalf("redirect_uris not adopted: %v", client.RedirectURIs)
	}
	// The document is cached so repeated connector probes do not become repeated
	// outbound fetches.
	requested = ""
	if _, err := svc.ResolveClient(context.Background(), clientID); err != nil {
		t.Fatalf("cached resolve: %v", err)
	}
	if requested != "" {
		t.Fatalf("cache missed: refetched %q", requested)
	}
}

func TestCIMDRejectsInvalidDocument(t *testing.T) {
	cases := map[string]string{
		"not json":     "hello",
		"no redirects": `{"client_name":"x"}`,
		"bad redirect": `{"redirect_uris":["http://evil.example/cb"]}`,
		"json array":   `["https://chatgpt.com/cb"]`,
		"empty object": `{}`,
	}
	for name, body := range cases {
		svc := newTestService(t, nil)
		svc.cimd.fetchFn = func(context.Context, string) ([]byte, error) { return []byte(body), nil }
		if _, err := svc.ResolveClient(context.Background(), "https://chatgpt.com/m/"+name); err == nil {
			t.Fatalf("%s: invalid metadata document accepted", name)
		}
	}
	svc := newTestService(t, nil)
	svc.cimd.fetchFn = func(context.Context, string) ([]byte, error) { return nil, net.ErrClosed }
	if _, err := svc.ResolveClient(context.Background(), "https://chatgpt.com/m/unreachable"); err == nil {
		t.Fatal("unreachable metadata document accepted")
	}
}

func TestIsPublicUnicastBlocksInternalRanges(t *testing.T) {
	for _, raw := range []string{
		"127.0.0.1", "10.0.0.5", "192.168.1.1", "172.16.0.1", "169.254.169.254",
		"100.64.0.1", "::1", "0.0.0.0", "255.255.255.255", "224.0.0.1", "fe80::1",
	} {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("test bug: %q did not parse", raw)
		}
		if isPublicUnicast(ip) {
			t.Fatalf("%q was treated as public unicast", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !isPublicUnicast(net.ParseIP(raw)) {
			t.Fatalf("%q was blocked but is public unicast", raw)
		}
	}
	if isPublicUnicast(nil) {
		t.Fatal("nil IP treated as public")
	}
}

func TestNegotiateScopes(t *testing.T) {
	svc := newTestService(t, nil)
	if got := svc.NegotiateScopes(ScopeRead); len(got) != 1 || got[0] != ScopeRead {
		t.Fatalf("read scope not granted: %v", got)
	}
	// fund.write is not advertised by default, so requesting it must down-scope
	// rather than escalate.
	if got := svc.NegotiateScopes(ScopeRead + " " + ScopeWrite); len(got) != 1 || got[0] != ScopeRead {
		t.Fatalf("write scope leaked into the grant: %v", got)
	}
	if got := svc.NegotiateScopes("openid profile email"); len(got) != 1 || got[0] != ScopeRead {
		t.Fatalf("empty intersection did not fall back to read: %v", got)
	}
	if got := svc.NegotiateScopes(""); len(got) != 1 || got[0] != ScopeRead {
		t.Fatalf("empty scope did not fall back to read: %v", got)
	}

	writable := newTestService(t, func(o *Options) { o.AllowWriteScope = true })
	got := writable.NegotiateScopes(ScopeRead + " " + ScopeWrite)
	if len(got) != 2 {
		t.Fatalf("write scope not granted when enabled: %v", got)
	}
	if RoleForScopes(got) != "operator" {
		t.Fatalf("write scope did not map to operator: %v", got)
	}
	if RoleForScopes([]string{ScopeRead}) != "analyst" {
		t.Fatal("read scope did not map to analyst")
	}
	if RoleForScopes(nil) != "analyst" {
		t.Fatal("empty scopes did not map to analyst")
	}
	if len(svc.opts.ScopesSupported()) != 1 {
		t.Fatalf("default deployment advertises write scope: %v", svc.opts.ScopesSupported())
	}
}
