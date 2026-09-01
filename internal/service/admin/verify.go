package admin

import (
	"context"
	"fmt"
)

type VerifyReport struct {
	OK               bool          `json:"ok"`
	Issues           []string      `json:"issues"`
	Details          VerifyDetails `json:"details"`
	DecisionBoundary string        `json:"decision_boundary"`
}

type VerifyDetails struct {
	SecuritiesWithoutNAV []string           `json:"securities_without_nav"`
	NegativePositions    []NegativePosition `json:"negative_positions"`
	MissingSettlement    int                `json:"missing_settlement_count"`
}

type NegativePosition struct {
	Code   string  `json:"code"`
	Shares float64 `json:"shares"`
}

func (s Service) VerifyData(ctx context.Context) (VerifyReport, error) {
	missingNAV, err := s.querySecuritiesWithoutNAV(ctx)
	if err != nil {
		return VerifyReport{}, err
	}
	negativePositions, err := s.queryNegativePositions(ctx)
	if err != nil {
		return VerifyReport{}, err
	}
	missingSettlementDays, err := s.countRows(ctx, "SELECT COUNT(*) FROM transactions WHERE settlement_days IS NULL")
	if err != nil {
		return VerifyReport{}, fmt.Errorf("verify settlement_days: %w", err)
	}

	issues := make([]string, 0)
	if len(missingNAV) > 0 {
		issues = append(issues, fmt.Sprintf("%d funds missing NAV", len(missingNAV)))
	}
	if len(negativePositions) > 0 {
		issues = append(issues, fmt.Sprintf("%d negative positions", len(negativePositions)))
	}
	if missingSettlementDays > 0 {
		issues = append(issues, fmt.Sprintf("%d tx missing settlement_days", missingSettlementDays))
	}
	if len(issues) == 0 {
		issues = append(issues, "all clear")
	}

	return VerifyReport{
		OK:     len(issues) == 1 && issues[0] == "all clear",
		Issues: issues,
		Details: VerifyDetails{
			SecuritiesWithoutNAV: missingNAV,
			NegativePositions:    negativePositions,
			MissingSettlement:    missingSettlementDays,
		},
		DecisionBoundary: "facts_only",
	}, nil
}

func (s Service) querySecuritiesWithoutNAV(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fd.fund_code
		FROM fund_details fd
		JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code
		WHERE ps.held_shares > 0.001
		  AND fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)
		ORDER BY fd.fund_code
		LIMIT ?
	`, adminListMaxRows)
	if err != nil {
		return nil, fmt.Errorf("verify missing NAV: %w", err)
	}
	defer rows.Close()

	codes := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan missing NAV security: %w", err)
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("missing NAV security rows: %w", err)
	}
	return codes, nil
}

func (s Service) queryNegativePositions(ctx context.Context) ([]NegativePosition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fund_code, held_shares
		FROM portfolio_snapshot
		WHERE held_shares < -0.001
		ORDER BY fund_code
		LIMIT ?
	`, adminListMaxRows)
	if err != nil {
		return nil, fmt.Errorf("verify negative positions: %w", err)
	}
	defer rows.Close()

	positions := []NegativePosition{}
	for rows.Next() {
		var position NegativePosition
		if err := rows.Scan(&position.Code, &position.Shares); err != nil {
			return nil, fmt.Errorf("scan negative position: %w", err)
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("negative position rows: %w", err)
	}
	return positions, nil
}

func (s Service) countRows(ctx context.Context, query string, args ...any) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
