package db

import (
	"strings"
	"testing"
)

func TestRebind(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    string
		wantErr string
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
		{
			name:  "double quoted identifier hides question mark",
			query: "SELECT \"col?name\" FROM funds WHERE id = ?",
			want:  "SELECT \"col?name\" FROM funds WHERE id = $1",
		},
		{
			name:  "double quoted identifier with escaped quote",
			query: "SELECT \"a\"\"?b\" FROM funds WHERE id = ?",
			want:  "SELECT \"a\"\"?b\" FROM funds WHERE id = $1",
		},
		{
			name:  "single quote inside double quoted identifier",
			query: "SELECT \"it's ?\" FROM t WHERE id = ?",
			want:  "SELECT \"it's ?\" FROM t WHERE id = $1",
		},
		{
			name:  "double quote inside single quoted string",
			query: "SELECT 'say \"what?\" now' WHERE id = ?",
			want:  "SELECT 'say \"what?\" now' WHERE id = $1",
		},
		{
			name:  "line comment hides question mark",
			query: "SELECT ? -- note? 'not a string'\nFROM funds",
			want:  "SELECT $1 -- note? 'not a string'\nFROM funds",
		},
		{
			name:  "line comment at end of query",
			query: "SELECT ? -- trailing ?",
			want:  "SELECT $1 -- trailing ?",
		},
		{
			name:  "block comment hides question mark",
			query: "SELECT ? /* ? 'x' \"y\" -- */ , ?",
			want:  "SELECT $1 /* ? 'x' \"y\" -- */ , $2",
		},
		{
			name:  "nested block comments",
			query: "SELECT /* outer ? /* inner ? */ ? */ ?",
			want:  "SELECT /* outer ? /* inner ? */ ? */ $1",
		},
		{
			name:  "slash star inside string is not a comment",
			query: "SELECT 'a/*b?' FROM t WHERE id = ?",
			want:  "SELECT 'a/*b?' FROM t WHERE id = $1",
		},
		{
			name:  "dashes inside string are not a comment",
			query: "SELECT '-- ?' , ?",
			want:  "SELECT '-- ?' , $1",
		},
		{
			name:  "dollar quote empty tag",
			query: "SELECT ?, $$a?b$$, ?",
			want:  "SELECT $1, $$a?b$$, $2",
		},
		{
			name:  "dollar quote identifier tag",
			query: "SELECT $body$a? 'b' \"c\"$body$, ?",
			want:  "SELECT $body$a? 'b' \"c\"$body$, $1",
		},
		{
			name:  "dollar quote containing dashes and stars",
			query: "SELECT $$--? /*?*/$$, ?",
			want:  "SELECT $$--? /*?*/$$, $1",
		},
		{
			name:  "dollar quote followed by placeholder",
			query: "SELECT $$a$$, ?",
			want:  "SELECT $$a$$, $1",
		},
		{
			name:  "positional dollar parameters untouched",
			query: "SELECT $1, $2 FROM t WHERE x = '?'",
			want:  "SELECT $1, $2 FROM t WHERE x = '?'",
		},
		{
			name:  "minus operator is not a comment",
			query: "SELECT 1 - ?",
			want:  "SELECT 1 - $1",
		},
		{
			name:  "question mark after block comment",
			query: "SELECT /* c */ ?",
			want:  "SELECT /* c */ $1",
		},
		{
			name:  "backslash does not escape quote in standard string",
			query: "SELECT 'a\\' = 'x' AND id = ?",
			want:  "SELECT 'a\\' = 'x' AND id = $1",
		},
		{
			name:  "dollar quote without placeholders bypasses scanner",
			query: "DO $$ BEGIN PERFORM 'x'; END $$",
			want:  "DO $$ BEGIN PERFORM 'x'; END $$",
		},
		{
			name:    "unterminated string is an error",
			query:   "SELECT 'oops ?",
			wantErr: "unterminated string literal",
		},
		{
			name:    "unterminated quoted identifier is an error",
			query:   "SELECT \"oops ?",
			wantErr: "unterminated quoted identifier",
		},
		{
			name:    "unterminated block comment is an error",
			query:   "SELECT /* oops ?",
			wantErr: "unterminated block comment",
		},
		{
			name:    "unterminated dollar quote is an error",
			query:   "SELECT $body$oops ?",
			wantErr: "unterminated dollar-quoted string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rebind(tt.query)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("rebind(%q) = %q, want error containing %q", tt.query, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("rebind(%q) error = %q, want containing %q", tt.query, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("rebind(%q) unexpected error: %v", tt.query, err)
			}
			if got != tt.want {
				t.Fatalf("rebind(%q)\n got: %q\nwant: %q", tt.query, got, tt.want)
			}
		})
	}
}
