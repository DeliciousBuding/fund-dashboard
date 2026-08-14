package db

import "testing"

func TestRebind(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "no placeholders",
			query: "SELECT id FROM funds",
			want:  "SELECT id FROM funds",
		},
		{
			name:  "multiple placeholders",
			query: "SELECT * FROM funds WHERE id = ? AND status = ?",
			want:  "SELECT * FROM funds WHERE id = $1 AND status = $2",
		},
		{
			name:  "placeholder inside single-quoted string preserved",
			query: "SELECT * FROM funds WHERE note = 'what?' AND id = ?",
			want:  "SELECT * FROM funds WHERE note = 'what?' AND id = $1",
		},
		{
			name:  "escaped quotes inside string",
			query: "SELECT * FROM funds WHERE name = 'O''Reilly?' AND id = ?",
			want:  "SELECT * FROM funds WHERE name = 'O''Reilly?' AND id = $1",
		},
		{
			name:  "mix of placeholders and string with question mark",
			query: "INSERT INTO events (kind, payload, ref) VALUES (?, 'has ? inside', ?)",
			want:  "INSERT INTO events (kind, payload, ref) VALUES ($1, 'has ? inside', $2)",
		},
		{
			name:  "string-only question mark no rebind",
			query: "SELECT 'is this a ?' AS q",
			want:  "SELECT 'is this a ?' AS q",
		},
		{
			name:  "adjacent escaped quotes then placeholder",
			query: "SELECT * FROM t WHERE s = '''?' AND id = ?",
			want:  "SELECT * FROM t WHERE s = '''?' AND id = $1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rebind(tt.query)
			if got != tt.want {
				t.Fatalf("rebind(%q)\n got: %q\nwant: %q", tt.query, got, tt.want)
			}
		})
	}
}
