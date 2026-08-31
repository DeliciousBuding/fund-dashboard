package httpapi

import (
	"net"
	"net/http/httptest"
	"testing"
)

// clientIP 表驱动:可信代理白名单(design 06 §2.4)与维持现状(空白名单)。
// 验收矩阵「FUND_TRUSTED_PROXIES:可信网段时取 XFF 客户端位,不可信直连忽略 XFF」。

func mustCIDRs(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatalf("bad cidr %q: %v", c, err)
		}
		out = append(out, ipnet)
	}
	return out
}

func TestClientIPTrustedProxyMatrix(t *testing.T) {
	trusted := mustCIDRs(t, "10.0.0.0/8", "2001:db8::/32")

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		trusted    []*net.IPNet
		want       string
	}{
		{
			name: "无白名单:忽略 XFF 回退 RemoteAddr(fail-closed)", remoteAddr: "192.168.1.2:8888",
			xff: "203.0.113.9, 10.1.1.1", trusted: nil, want: "192.168.1.2",
		},
		{
			name: "无白名单:无 XFF 回退 RemoteAddr", remoteAddr: "198.51.100.7:9999",
			xff: "", trusted: nil, want: "198.51.100.7",
		},
		{
			name: "可信直连+单级 XFF:取客户端位", remoteAddr: "10.0.0.8:443",
			xff: "203.0.113.99", trusted: trusted, want: "203.0.113.99",
		},
		{
			name: "可信直连+多级 XFF 链:跳过可信跳取首个不可信", remoteAddr: "10.0.0.9:443",
			xff: "203.0.113.5, 10.0.0.8, 10.9.9.9", trusted: trusted, want: "203.0.113.5",
		},
		{
			name: "可信直连+XFF 全可信:回退 RemoteAddr", remoteAddr: "10.1.2.3:443",
			xff: "10.0.0.1, 10.0.0.2", trusted: trusted, want: "10.1.2.3",
		},
		{
			name: "不可信直连:忽略 XFF 回退 RemoteAddr", remoteAddr: "192.0.2.10:1234",
			xff: "203.0.113.77, 10.0.0.8", trusted: trusted, want: "192.0.2.10",
		},
		{
			name: "RemoteAddr 无端口(httptest 默认)", remoteAddr: "192.0.2.20",
			xff: "", trusted: trusted, want: "192.0.2.20",
		},
		{
			name: "IPv6 可信直连+XFF", remoteAddr: "[2001:db8::99]:443",
			xff: "203.0.113.5", trusted: trusted, want: "203.0.113.5",
		},
		{
			name: "XFF 空项补齐", remoteAddr: "10.0.0.8:443",
			xff: "203.0.113.9, , 10.0.0.8", trusted: trusted, want: "203.0.113.9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := clientIP(req, tc.trusted); got != tc.want {
				t.Fatalf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}
