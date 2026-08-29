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
	}
}
