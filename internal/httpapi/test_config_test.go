package httpapi

import "github.com/DeliciousBuding/fund-dashboard/internal/config"

const testAdminKey = "test-admin-key"
const testPublicMCPKey = "test-public-mcp-key"
const testEdgeKey = "test-edge-key"

// testCfg returns a config with the test admin key and defaults suitable for tests.
func testCfg() config.Config {
	return config.Config{
		ServiceName:      "fund-dashboard-go",
		Version:          "test",
		AdminKey:         testAdminKey,
		PublicMCPKey:     testPublicMCPKey,
		EdgeKey:          testEdgeKey,
		EdgeAuthEnabled:  true,
		AuthSecureCookie: true,
		// High enough that OAuth flow tests never trip the per-IP bucket.
		OAuthRPM: 60000,
		// High enough that general tests never trip the /mcp pre-auth per-IP
		// bucket; the dedicated pre-auth tests build a small config themselves.
		MCPPreAuthRPM:    60000,
		OAuthEnabled:     true,
		OAuthAutoApprove: true,
	}
}
