package main

import (
	"bytes"
	"strings"
	"testing"
)

func lookup(m map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

func TestReadPasswordInputEnvTakesPrecedence(t *testing.T) {
	in, err := readPasswordInput([]string{"argv-secret"}, lookup(map[string]string{passwordEnvVar: "env-secret"}), strings.NewReader("stdin-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if in.value != "env-secret" || in.source != sourceEnv {
		t.Fatalf("input = %+v, want env-secret/env", in)
	}
}

func TestReadPasswordInputArgvFallback(t *testing.T) {
	in, err := readPasswordInput([]string{"argv-secret"}, lookup(nil), strings.NewReader("ignored"))
	if err != nil {
		t.Fatal(err)
	}
	if in.value != "argv-secret" || in.source != sourceArgv {
		t.Fatalf("input = %+v, want argv-secret/argv", in)
	}
}

func TestReadPasswordInputStdin(t *testing.T) {
	in, err := readPasswordInput(nil, lookup(nil), strings.NewReader("piped-secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	if in.value != "piped-secret" || in.source != sourceStdin {
		t.Fatalf("input = %+v, want piped-secret/stdin", in)
	}
}

func TestReadPasswordInputStdinTrimsOnlyTrailingNewlines(t *testing.T) {
	in, err := readPasswordInput(nil, lookup(nil), strings.NewReader("  spaced secret \r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if in.value != "  spaced secret " {
		t.Fatalf("value = %q, want trailing CRLF removed but spaces preserved", in.value)
	}
}

func TestReadPasswordInputRejectsEmpty(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		env   map[string]string
		stdin string
	}{
		{"empty env", []string{"x"}, map[string]string{passwordEnvVar: ""}, "ignored"},
		{"empty argv", []string{""}, nil, "ignored"},
		{"empty stdin", nil, nil, ""},
		{"newline-only stdin", nil, nil, "\r\n"},
		{"too many args", []string{"a", "b"}, nil, "ignored"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := readPasswordInput(c.args, lookup(c.env), strings.NewReader(c.stdin)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestRunEnvPathHashesWithoutWarning(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(&out, &errOut,
		[]string{"ignored-arg"},
		lookup(map[string]string{passwordEnvVar: "correct horse battery staple 1"}),
		strings.NewReader(""))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	hash := strings.TrimSpace(out.String())
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("stdout = %q, want argon2id PHC", hash)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on env path", errOut.String())
	}
}

func TestRunArgvPathHashesAndWarns(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(&out, &errOut,
		[]string{"correct horse battery staple 1"},
		lookup(nil),
		strings.NewReader(""))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	hash := strings.TrimSpace(out.String())
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("stdout = %q, want argon2id PHC", hash)
	}
	if !strings.Contains(errOut.String(), "warning") {
		t.Fatalf("stderr = %q, want argv warning", errOut.String())
	}
}

func TestRunRejectsEmptyInput(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(&out, &errOut, nil, lookup(nil), strings.NewReader(""))
	if code == 0 {
		t.Fatal("exit = 0, want failure on empty input")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", out.String())
	}
	if !strings.Contains(errOut.String(), "空") {
		t.Fatalf("stderr = %q, want empty-password error", errOut.String())
	}
}
