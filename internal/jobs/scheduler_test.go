package jobs_test

import (
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
)

// Compile-time check: portfolio.Service satisfies jobs.DCARunner.
var _ jobs.DCARunner = portfoliosvc.Service{}
