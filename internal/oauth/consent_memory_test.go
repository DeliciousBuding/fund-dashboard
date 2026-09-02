package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// The consent memory is what makes first-use consent safe to skip afterwards, so
// its failure modes are the interesting ones: it must be per client, per scope,
// and it must fail closed when there is nowhere durable to remember an approval.

func TestConsentRequiredFailsClosedWithoutDurableMemory(t *testing.T) {
	svc := NewService(nil, Options{PublicBaseURL: testIssuer, AutoApprove: true, Now: time.Now})
	required, err := svc.consentRequired(context.Background(), "any-client", []string{ScopeRead})
	if err != nil {
		t.Fatalf("consentRequired: %v", err)
	}
	if !required {
		t.Fatal("a server with no durable consent memory must ask; an unproven approval may never become a silent grant")
	}
}

func TestConsentRequiredAsksForNonReadOnlyScopes(t *testing.T) {
	svc := newTestService(t, func(o *Options) { o.AllowWriteScope = true })
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	if err := svc.store.RecordClientConsent(context.Background(), clientID, ScopeRead, time.Now().Unix()); err != nil {
		t.Fatalf("record read consent: %v", err)
	}
	required, err := svc.consentRequired(context.Background(), clientID, []string{ScopeRead, ScopeWrite})
	if err != nil {
		t.Fatalf("consentRequired: %v", err)
	}
	if !required {
		t.Fatal("a prior read approval must not silently cover a write scope")
	}
}

func TestConsentRequiredHonoursAutoApproveOff(t *testing.T) {
	svc := newTestService(t, func(o *Options) { o.AutoApprove = false })
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	if err := svc.store.RecordClientConsent(context.Background(), clientID, ScopeRead, time.Now().Unix()); err != nil {
		t.Fatalf("record consent: %v", err)
	}
	required, err := svc.consentRequired(context.Background(), clientID, []string{ScopeRead})
	if err != nil {
		t.Fatalf("consentRequired: %v", err)
	}
	if !required {
		t.Fatal("AutoApprove=false must ask every time, even for a previously approved client")
	}
}

func TestApprovalForOneClientDoesNotMakeAnotherSilent(t *testing.T) {
	svc := newTestService(t, nil)
	approved := registerTestClient(t, svc, "https://chatgpt.com/cb")
	other := registerTestClient(t, svc, "https://chatgpt.com/cb")

	first := authorizeReadOnlyForOwner(t, svc, approved, "s1")
	if first.Kind != DecisionConsent {
		t.Fatalf("first kind = %s, want consent", first.Kind)
	}
	approveConsentForTest(t, svc, first.Grant, "s1")

	if got := authorizeReadOnlyForOwner(t, svc, approved, "s2"); got.Kind != DecisionRedirect {
		t.Fatalf("approved client kind = %s, want silent redirect", got.Kind)
	}
	// Consent is per client: an approval says "this connector may read", never
	// "any connector may read".
	if got := authorizeReadOnlyForOwner(t, svc, other, "s3"); got.Kind != DecisionConsent {
		t.Fatalf("unapproved client kind = %s, want consent", got.Kind)
	}
}

func TestStoreClientConsentIsPerClientAndPerScope(t *testing.T) {
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	now := time.Now().Unix()

	has, err := store.HasClientConsent(ctx, "c1", []string{ScopeRead})
	if err != nil || has {
		t.Fatalf("fresh store: has=%v err=%v, want false/nil", has, err)
	}
	if err := store.RecordClientConsent(ctx, "c1", ScopeRead, now); err != nil {
		t.Fatalf("record: %v", err)
	}
	if has, err = store.HasClientConsent(ctx, "c1", []string{ScopeRead}); err != nil || !has {
		t.Fatalf("after record: has=%v err=%v, want true/nil", has, err)
	}
	// Recording twice must not duplicate or error (the row is idempotent).
	if err := store.RecordClientConsent(ctx, "c1", ScopeRead, now+10); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	if has, err = store.HasClientConsent(ctx, "c2", []string{ScopeRead}); err != nil || has {
		t.Fatalf("other client: has=%v err=%v, want false/nil", has, err)
	}
	// All requested scopes must be covered, not just one of them.
	if has, err = store.HasClientConsent(ctx, "c1", []string{ScopeRead, ScopeWrite}); err != nil || has {
		t.Fatalf("partial coverage: has=%v err=%v, want false/nil", has, err)
	}
	if err := store.RecordClientConsent(ctx, "c1", ScopeWrite, now); err != nil {
		t.Fatalf("record write scope: %v", err)
	}
	if has, err = store.HasClientConsent(ctx, "c1", []string{ScopeRead, ScopeWrite}); err != nil || !has {
		t.Fatalf("full coverage: has=%v err=%v, want true/nil", has, err)
	}
	// An empty scope list is never "already approved".
	if has, err = store.HasClientConsent(ctx, "c1", nil); err != nil || has {
		t.Fatalf("empty scopes: has=%v err=%v, want false/nil", has, err)
	}
}

func TestNilStoreConsentMemoryIsInert(t *testing.T) {
	var store *Store
	ctx := context.Background()
	if err := store.RecordClientConsent(ctx, "c1", ScopeRead, 1); err != nil {
		t.Fatalf("nil store record: %v", err)
	}
	has, err := store.HasClientConsent(ctx, "c1", []string{ScopeRead})
	if err != nil || has {
		t.Fatalf("nil store has=%v err=%v, want false/nil", has, err)
	}
	if err := store.DeleteClientConsent(ctx, "c1"); err != nil {
		t.Fatalf("nil store delete: %v", err)
	}
}

func TestDeleteClientForgetsItsConsent(t *testing.T) {
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	client := Client{ID: "c1", Name: "connector", RedirectURIs: []string{"https://chatgpt.com/cb"},
		GrantTypes: []string{"authorization_code"}, TokenEndpointAuthMethod: "none", CreatedAt: time.Now().Unix()}
	if err := store.InsertClient(ctx, client); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	if err := store.RecordClientConsent(ctx, "c1", ScopeRead, time.Now().Unix()); err != nil {
		t.Fatalf("record consent: %v", err)
	}
	if err := store.DeleteClient(ctx, "c1"); err != nil {
		t.Fatalf("delete client: %v", err)
	}
	has, err := store.HasClientConsent(ctx, "c1", []string{ScopeRead})
	if err != nil {
		t.Fatalf("has after delete: %v", err)
	}
	if has {
		t.Fatal("deleting a registration must not leave a silent approval behind")
	}
}
