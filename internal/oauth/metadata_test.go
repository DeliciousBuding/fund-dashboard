package oauth

import (
	"strings"
	"testing"
)

func TestProtectedResourceMetadata(t *testing.T) {
	svc := newTestService(t, nil)
	prm := svc.ProtectedResourceMetadata(testIssuer)
	if prm["resource"] != testIssuer+"/mcp" {
		t.Fatalf("resource = %v, want %s/mcp", prm["resource"], testIssuer)
	}
	servers, ok := prm["authorization_servers"].([]string)
	if !ok || len(servers) != 1 || servers[0] != testIssuer {
		t.Fatalf("authorization_servers = %v", prm["authorization_servers"])
	}
	scopes, ok := prm["scopes_supported"].([]string)
	if !ok || len(scopes) != 1 || scopes[0] != ScopeRead {
		t.Fatalf("scopes_supported = %v", prm["scopes_supported"])
	}
	methods, ok := prm["bearer_methods_supported"].([]string)
	if !ok || len(methods) != 1 || methods[0] != "header" {
		t.Fatalf("bearer_methods_supported = %v", prm["bearer_methods_supported"])
	}
}

func TestAuthorizationServerMetadata(t *testing.T) {
	svc := newTestService(t, nil)
	asm := svc.AuthorizationServerMetadata(testIssuer)
	if asm["issuer"] != testIssuer {
		t.Fatalf("issuer = %v", asm["issuer"])
	}
	for _, key := range []string{
		"authorization_endpoint", "token_endpoint", "registration_endpoint", "jwks_uri",
		"scopes_supported", "response_types_supported", "grant_types_supported",
		"code_challenge_methods_supported", "token_endpoint_auth_methods_supported",
		"client_id_metadata_document_supported",
	} {
		if _, ok := asm[key]; !ok {
			t.Fatalf("authorization server metadata missing %q", key)
		}
	}
	wantEndpoints := map[string]string{
		"authorization_endpoint": "/oauth/authorize",
		"token_endpoint":         "/oauth/token",
		"registration_endpoint":  "/oauth/register",
		"jwks_uri":               "/oauth/jwks",
		"revocation_endpoint":    "/oauth/revoke",
	}
	for key, suffix := range wantEndpoints {
		got, _ := asm[key].(string)
		if got != testIssuer+suffix {
			t.Fatalf("%s = %q, want %q", key, got, testIssuer+suffix)
		}
		if !strings.HasPrefix(got, testIssuer) {
			t.Fatalf("%s is not absolute under the issuer: %q", key, got)
		}
	}
	responseTypes := asm["response_types_supported"].([]string)
	if len(responseTypes) != 1 || responseTypes[0] != "code" {
		t.Fatalf("response_types_supported = %v, want [code] (implicit must not be offered)", responseTypes)
	}
	challenges := asm["code_challenge_methods_supported"].([]string)
	if len(challenges) != 1 || challenges[0] != "S256" {
		t.Fatalf("code_challenge_methods_supported = %v, want [S256]", challenges)
	}
	authMethods := asm["token_endpoint_auth_methods_supported"].([]string)
	if len(authMethods) != 1 || authMethods[0] != "none" {
		t.Fatalf("token_endpoint_auth_methods_supported = %v, want [none] (public client)", authMethods)
	}
	if asm["client_id_metadata_document_supported"] != true {
		t.Fatal("client_id_metadata_document_supported must be advertised for OpenAI connectors")
	}
	grants := asm["grant_types_supported"].([]string)
	if len(grants) != 2 {
		t.Fatalf("grant_types_supported = %v", grants)
	}
}

func TestWellKnownPathsCoverBareAndSuffixedForms(t *testing.T) {
	prm := WellKnownPathProtectedResource("/mcp")
	if len(prm) != 2 || prm[0] != "/.well-known/oauth-protected-resource" ||
		prm[1] != "/.well-known/oauth-protected-resource/mcp" {
		t.Fatalf("protected resource paths = %v", prm)
	}
	asm := WellKnownPathAuthorizationServer("/mcp")
	want := map[string]bool{
		"/.well-known/oauth-authorization-server":     true,
		"/.well-known/oauth-authorization-server/mcp": true,
		"/.well-known/openid-configuration":           true,
		"/.well-known/openid-configuration/mcp":       true,
	}
	if len(asm) != len(want) {
		t.Fatalf("authorization server paths = %v", asm)
	}
	for _, path := range asm {
		if !want[path] {
			t.Fatalf("unexpected well-known path %q", path)
		}
	}
	if got := WellKnownPathProtectedResource("/"); len(got) != 1 {
		t.Fatalf("root resource should advertise a single path: %v", got)
	}
	if got := WellKnownPathAuthorizationServer(""); len(got) != 2 {
		t.Fatalf("root resource should advertise both authorization server names: %v", got)
	}
}

func TestAboutDocument(t *testing.T) {
	svc := newTestService(t, nil)
	doc := svc.AboutDocument(testIssuer)
	if doc["resource"] != testIssuer+"/mcp" {
		t.Fatalf("about resource = %v", doc["resource"])
	}
	scopes, ok := doc["scopes"].([]map[string]string)
	if !ok || len(scopes) != 1 {
		t.Fatalf("about scopes = %v", doc["scopes"])
	}
	if scopes[0]["role"] != "analyst" {
		t.Fatalf("read scope should map to analyst: %v", scopes[0])
	}

	writable := newTestService(t, func(o *Options) { o.AllowWriteScope = true })
	wideScopes := writable.AboutDocument(testIssuer)["scopes"].([]map[string]string)
	if len(wideScopes) != 2 || wideScopes[1]["role"] != "operator" {
		t.Fatalf("write scope not described when enabled: %v", wideScopes)
	}
}

func TestJWKSIsSafeToPublish(t *testing.T) {
	svc := newTestService(t, nil)
	jwks, err := svc.JWKS()
	if err != nil {
		t.Fatalf("jwks: %v", err)
	}
	keys, ok := jwks["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("jwks keys = %v", jwks["keys"])
	}
	key := keys[0].(map[string]any)
	for _, field := range []string{"kty", "crv", "x", "y", "kid", "alg", "use"} {
		if _, ok := key[field]; !ok {
			t.Fatalf("jwk missing %q", field)
		}
	}
	if key["kty"] != "EC" || key["crv"] != "P-256" || key["alg"] != AlgES256 || key["use"] != "sig" {
		t.Fatalf("jwk has unexpected parameters: %v", key)
	}
	encoded := mustJSON(key)
	if strings.Contains(encoded, `"d"`) {
		t.Fatal("jwks published the private exponent")
	}
	if strings.Contains(encoded, "PRIVATE KEY") {
		t.Fatal("jwks published PEM private material")
	}
}
