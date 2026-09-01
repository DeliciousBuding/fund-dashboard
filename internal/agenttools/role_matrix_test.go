package agenttools

import "testing"

func TestRoleAllowsScopeMatrix(t *testing.T) {
	cases := []struct {
		role  Role
		scope Scope
		want  bool
	}{
		{RoleViewer, ScopeRead, true},
		{RoleViewer, ScopeWrite, false},
		{RoleViewer, ScopeMaintenance, false},
		{RoleViewer, ScopeExternalContext, false},
		{RoleViewer, ScopeDisabled, false},
		{RoleAnalyst, ScopeRead, true},
		{RoleAnalyst, ScopeExternalContext, true},
		{RoleAnalyst, ScopeWrite, false},
		{RoleAnalyst, ScopeMaintenance, false},
		{RoleMaintainer, ScopeRead, true},
		{RoleMaintainer, ScopeMaintenance, true},
		{RoleMaintainer, ScopeExternalContext, true},
		{RoleMaintainer, ScopeWrite, false},
		{RoleOperator, ScopeRead, true},
		{RoleOperator, ScopeMaintenance, true},
		{RoleOperator, ScopeExternalContext, true},
		{RoleOperator, ScopeWrite, true},
		{Role(""), ScopeRead, false},
		{Role("unknown"), ScopeRead, false},
	}
	for _, tc := range cases {
		if got := RoleAllowsScope(tc.role, tc.scope); got != tc.want {
			t.Errorf("RoleAllowsScope(%q, %q) = %v, want %v", tc.role, tc.scope, got, tc.want)
		}
	}
}
