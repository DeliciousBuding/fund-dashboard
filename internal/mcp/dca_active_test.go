package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

func TestUpsertDCAPlanAllowsExplicitActiveValues(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServerWithRole(t, db, agenttools.RoleOperator)

	for _, tc := range []struct {
		name string
		arg  string
		want int
	}{
		{"disable with zero", "0", 0},
		{"enable with one", "1", 1},
	} {
		resp := server.Handle(context.Background(), Request{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"` + tc.name + `"`),
			Method:  "tools/call",
			Params: json.RawMessage(`{"name":"upsert_dca_plan","arguments":{` +
				`"confirmation_id":1,"confirmation_token":"test-token",` +
				`"id":1,"fund_code":"019173","amount":10,"active":` + tc.arg + `}}`),
		})
		if resp.Error != nil {
			t.Fatalf("%s: response = %#v, want success", tc.name, resp)
		}
		var active int
		if err := db.QueryRowContext(context.Background(), `SELECT active FROM dca_plans WHERE id = 1`).Scan(&active); err != nil {
			t.Fatalf("%s: query active: %v", tc.name, err)
		}
		if active != tc.want {
			t.Fatalf("%s: active = %d, want %d", tc.name, active, tc.want)
		}
	}
}

func TestUpsertDCAPlanRejectsInvalidActive(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServerWithRole(t, db, agenttools.RoleOperator)

	for _, tc := range []struct {
		name string
		arg  string
	}{
		{"out of range", "2"},
		{"negative", "-1"},
		{"fractional", "1.5"},
		{"bool", "true"},
		{"string", `"x"`},
	} {
		resp := server.Handle(context.Background(), Request{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"` + tc.name + `"`),
			Method:  "tools/call",
			Params: json.RawMessage(`{"name":"upsert_dca_plan","arguments":{` +
				`"confirmation_id":1,"confirmation_token":"test-token",` +
				`"id":1,"fund_code":"019173","amount":10,"active":` + tc.arg + `}}`),
		})
		if resp.Error == nil || !strings.Contains(resp.Error.Message, "invalid_params") {
			t.Fatalf("%s: response = %#v, want invalid_params", tc.name, resp)
		}
		var active int
		if err := db.QueryRowContext(context.Background(), `SELECT active FROM dca_plans WHERE id = 1`).Scan(&active); err != nil {
			t.Fatalf("%s: query active: %v", tc.name, err)
		}
		if active != 1 {
			t.Fatalf("%s: active = %d, want unchanged 1", tc.name, active)
		}
	}
}
