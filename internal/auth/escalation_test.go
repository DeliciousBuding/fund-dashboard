package auth

import (
	"errors"
	"testing"
	"time"
)

// 递增锁定（design 06 §2.2）：连续触发锁定按 2 的幂次翻倍、封顶 24h、成功清零。
// 验收矩阵「锁定递增：连续 3 轮触发锁定 → Retry-After 递增且 ≤24h 封顶；成功后清零」。

func roundTrip(t *testing.T, l *Limiter, key string, now *time.Time) time.Duration {
	t.Helper()
	for i := 0; i < l.MaxFails; i++ {
		if _, ok := l.Allow(key); !ok {
			t.Fatalf("attempt %d should be allowed before lockout", i)
		}
		l.Failure(key)
	}
	retryAfter, ok := l.Allow(key)
	if ok {
		t.Fatal("expected key to be locked after MaxFails failures")
	}
	return retryAfter
}

func TestLimiterEscalatingLockoutDoublesPerStrike(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := NewLimiter(func() time.Time { return now })

	ra1 := roundTrip(t, l, "ip:1", &now)
	if ra1 != 15*time.Minute {
		t.Fatalf("strike 1 retry = %v, want 15m", ra1)
	}
	now = now.Add(16 * time.Minute) // 解锁
	ra2 := roundTrip(t, l, "ip:1", &now)
	if ra2 != 30*time.Minute {
		t.Fatalf("strike 2 retry = %v, want 30m", ra2)
	}
	now = now.Add(31 * time.Minute)
	ra3 := roundTrip(t, l, "ip:1", &now)
	if ra3 != time.Hour {
		t.Fatalf("strike 3 retry = %v, want 1h", ra3)
	}
	if !(ra1 < ra2 && ra2 < ra3) {
		t.Fatalf("retry must escalate: %v / %v / %v", ra1, ra2, ra3)
	}
}

func TestLimiterEsclatedLockoutCapsAt24h(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := NewLimiter(func() time.Time { return now })
	var last time.Duration
	for i := 0; i < 12; i++ {
		last = roundTrip(t, l, "cap:1", &now)
		if last > maxLockout {
			t.Fatalf("round %d retry = %v, exceeds 24h cap", i+1, last)
		}
		// 解锁:推进 25h（最长锁定 24h）
		now = now.Add(25 * time.Hour)
	}
	// 第 8 轮起已封顶 24h。
	if last != maxLockout {
		t.Fatalf("settled retry = %v, want 24h cap", last)
	}
}

func TestLimiterSuccessResetsStrikes(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := NewLimiter(func() time.Time { return now })

	roundTrip(t, l, "ip:9", &now)
	now = now.Add(16 * time.Minute)
	roundTrip(t, l, "ip:9", &now) // strike 2 → 30m
	now = now.Add(31 * time.Minute)

	// 成功登录清零 strikes → 下一轮恢复基础锁定 15 分钟。
	l.Success("ip:9")
	ra := roundTrip(t, l, "ip:9", &now)
	if ra != 15*time.Minute {
		t.Fatalf("retry after success = %v, want base 15m", ra)
	}
}

func TestEscalatedLockoutShiftOverflowSafe(t *testing.T) {
	// 直接测纯函数:大 strikes 不能溢出为负/极小值。
	if got := escalatedLockout(15*time.Minute, 10000); got != maxLockout {
		t.Fatalf("huge strikes = %v, want 24h cap", got)
	}
	if got := escalatedLockout(15*time.Minute, 7); got != 16*time.Hour {
		t.Fatalf("strike 7 = %v, want 16h", got)
	}
	if got := escalatedLockout(15*time.Minute, 8); got != maxLockout {
		t.Fatalf("strike 8 = %v, want 24h cap (32h doubled capped)", got)
	}
	if got := escalatedLockout(0, 5); got != 0 {
		t.Fatalf("zero base = %v, want 0", got)
	}
}

func TestLimiterLockStrikesSaturateInsteadOfOverflowing(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := NewLimiter(func() time.Time { return now })

	// Seed the escalation counter one below the saturation cap, then keep
	// tripping lockouts: the counter must stop growing and the lock duration
	// must stay pinned at maxLockout.
	l.mu.Lock()
	l.lockStrikes["ip:sat"] = maxLockStrikes - 1
	l.mu.Unlock()

	ra := roundTrip(t, l, "ip:sat", &now) // increments to maxLockStrikes
	if ra != maxLockout {
		t.Fatalf("first trip retry = %v, want 24h cap", ra)
	}
	now = now.Add(25 * time.Hour)
	ra = roundTrip(t, l, "ip:sat", &now) // must not exceed the cap
	if ra != maxLockout {
		t.Fatalf("post-saturation retry = %v, want 24h cap", ra)
	}

	l.mu.Lock()
	strikes := l.lockStrikes["ip:sat"]
	l.mu.Unlock()
	if strikes != maxLockStrikes {
		t.Fatalf("strikes = %d, want saturated at %d", strikes, maxLockStrikes)
	}
}

func TestPasswordPolicyLetterAndDigit(t *testing.T) {
	cases := []struct {
		password string
		wantErr  bool
	}{
		{"12345678901", true},                // 11 位纯数字 → 长度不足
		{"abcdefghijk1", false},              // 12 位字母+数字
		{"A1bcdefghijk", false},              // 大写字母也可以
		{"abcdefghijkl", true},               // 12 位纯字母 → 缺数字
		{"123456789012", true},               // 12 位纯数字 → 缺字母
		{"abcd-1234-ef", false},              // 混合符号 + 字母数字
		{"a1", true},                         // 超短（长度不足）
		{"abcdefghijklmnopqrstuvwxyz", true}, // 长度够但纯字母
	}
	for _, tc := range cases {
		svc := newTestService(t, Options{}) // 每个用例独立实例,避免已初始化干扰
		_, err := svc.Setup(t.Context(), tc.password, "", "")
		if tc.wantErr {
			if err == nil || !errors.Is(err, ErrWeakPassword) {
				t.Fatalf("password %q want ErrWeakPassword, got %v", tc.password, err)
			}
		} else if err != nil {
			t.Fatalf("password %q should pass: %v", tc.password, err)
		}
	}
}
