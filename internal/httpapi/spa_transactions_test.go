package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/contracts"
)

// TestAnalysisCompareEnforcesSharedCodeBound pins the REST compare surface to
// the fan-out bound owned by internal/contracts. The same logical operation is
// reachable through MCP compare_funds; a per-surface copy of the limit is how
// the two surfaces drift into accepting different batch sizes.
func TestAnalysisCompareEnforcesSharedCodeBound(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()
	router := newAuthedRouter(t, testCfg(), db)

	codes := make([]string, 0, contracts.MaxCompareCodes+1)
	for i := 0; i <= contracts.MaxCompareCodes; i++ {
		codes = append(codes, fmt.Sprintf("CMP%02d", i))
	}

	over := doJSONRequest(t, router, http.MethodGet,
		"/api/analysis/compare?codes="+strings.Join(codes, ","), nil, http.StatusBadRequest)
	want := fmt.Sprintf("codes max %d", contracts.MaxCompareCodes)
	if over["error"] != want {
		t.Fatalf("error = %v, want %q", over["error"], want)
	}

	// The guard is "more than the bound", so exactly MaxCompareCodes is served.
	atBound := doJSONRequest(t, router, http.MethodGet,
		"/api/analysis/compare?codes="+strings.Join(codes[:contracts.MaxCompareCodes], ","), nil, http.StatusOK)
	if _, ok := atBound["funds"].([]any); !ok {
		t.Fatalf("funds = %s, want an array at the bound", toJSONString(t, atBound))
	}
}
