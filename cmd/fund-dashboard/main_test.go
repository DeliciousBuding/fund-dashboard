package main

import "testing"

func TestEnvironMap(t *testing.T) {
	cases := []struct {
		name  string
		pairs []string
		want  map[string]string
	}{
		{"simple pairs", []string{"FUND_ENV=production", "PORT=8765"}, map[string]string{"FUND_ENV": "production", "PORT": "8765"}},
		{"value containing equals", []string{"FUND_PG_DSN=postgres://u:p@h/db?x=1"}, map[string]string{"FUND_PG_DSN": "postgres://u:p@h/db?x=1"}},
		{"empty value", []string{"EMPTY="}, map[string]string{"EMPTY": ""}},
		{"malformed entry skipped", []string{"NO_EQUALS"}, map[string]string{}},
		{"last duplicate wins", []string{"K=first", "K=second"}, map[string]string{"K": "second"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := environMap(tc.pairs)
			if len(got) != len(tc.want) {
				t.Fatalf("environMap(%v) = %v, want %v", tc.pairs, got, tc.want)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Fatalf("environMap(%v)[%q] = %q, want %q", tc.pairs, k, got[k], want)
				}
			}
		})
	}
}
