package jobs_test

import (
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// Compile-time check: portfolio.Service satisfies jobs.DCARunner.
var _ jobs.DCARunner = portfoliosvc.Service{}
