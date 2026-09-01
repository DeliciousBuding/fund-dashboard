package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

// plainStub implements only the mandatory driver.Conn surface, forcing the
// wrapper onto its fallback paths.
type plainStub struct{}

func (plainStub) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (plainStub) Close() error                        { return nil }
func (plainStub) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

// fullStub additionally implements the optional interfaces the wrapper must
// forward to the inner pgx connection.
type fullStub struct {
	plainStub
	pingErr    error
	pingCalls  int
	checkErr   error
	checkCalls int
	isValid    bool
}

func (s *fullStub) Ping(ctx context.Context) error {
	s.pingCalls++
	return s.pingErr
}

func (s *fullStub) CheckNamedValue(nv *driver.NamedValue) error {
	s.checkCalls++
	return s.checkErr
}

func (s *fullStub) ResetSession(ctx context.Context) error { return nil }

func (s *fullStub) IsValid() bool { return s.isValid }

func TestRebindConnPingForwarded(t *testing.T) {
	sentinel := errors.New("ping failed")
	inner := &fullStub{pingErr: sentinel}
	c := &rebindConn{inner: inner}
	if err := c.Ping(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("Ping = %v, want forwarded sentinel", err)
	}
	if inner.pingCalls != 1 {
		t.Fatalf("inner ping calls = %d, want 1", inner.pingCalls)
	}
}

func TestRebindConnPingFallbackWhenUnimplemented(t *testing.T) {
	c := &rebindConn{inner: plainStub{}}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping = %v, want nil fallback", err)
	}
}

func TestRebindConnCheckNamedValueForwarded(t *testing.T) {
	sentinel := errors.New("bad value")
	inner := &fullStub{checkErr: sentinel}
	c := &rebindConn{inner: inner}
	nv := &driver.NamedValue{Ordinal: 1, Value: []string{"00700"}}
	if err := c.CheckNamedValue(nv); !errors.Is(err, sentinel) {
		t.Fatalf("CheckNamedValue = %v, want forwarded sentinel", err)
	}
	if inner.checkCalls != 1 {
		t.Fatalf("inner check calls = %d, want 1", inner.checkCalls)
	}
}

func TestRebindConnCheckNamedValueFallbackUsesDefaultConverter(t *testing.T) {
	c := &rebindConn{inner: plainStub{}}
	// The default converter accepts the builtin set.
	if err := c.CheckNamedValue(&driver.NamedValue{Ordinal: 1, Value: int64(7)}); err != nil {
		t.Fatalf("CheckNamedValue(int64) = %v, want nil", err)
	}
	// And rejects types pgx would accept natively, proving the fallback is
	// the restrictive default converter rather than pgx's permissive checker.
	if err := c.CheckNamedValue(&driver.NamedValue{Ordinal: 1, Value: []string{"a"}}); err == nil {
		t.Fatal("CheckNamedValue([]string) = nil, want default-converter rejection")
	}
}

func TestRebindConnIsValidForwarded(t *testing.T) {
	inner := &fullStub{isValid: false}
	c := &rebindConn{inner: inner}
	if c.IsValid() {
		t.Fatal("IsValid = true, want forwarded false from inner")
	}
	inner.isValid = true
	if !c.IsValid() {
		t.Fatal("IsValid = false, want forwarded true from inner")
	}
}

func TestRebindConnIsValidFallbackWhenUnimplemented(t *testing.T) {
	c := &rebindConn{inner: plainStub{}}
	if !c.IsValid() {
		t.Fatal("IsValid = false, want true fallback when inner lacks driver.Validator")
	}
}

func TestRebindDriverOpenRejectsMalformedDSN(t *testing.T) {
	d := &rebindDriver{}
	if _, err := d.Open("postgres://user%zz@host/db"); err == nil {
		t.Fatal("Open(malformed DSN) = nil error, want parse failure")
	}
}
