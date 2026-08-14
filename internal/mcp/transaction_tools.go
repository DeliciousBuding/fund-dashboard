package mcp

import (
	"context"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
)

func (s *Server) callAddTransaction(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	item := importTransactionFromArgs(args)
	result, err := s.admin.ImportTransactions(ctx, []adminsvc.ImportTransaction{item})
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(map[string]any{
		"ok":                result.OK,
		"imported":          result.Imported,
		"total":             result.Total,
		"affected_funds":    result.AffectedFunds,
		"decision_boundary": "facts_only",
		"side_effects":      "transaction_write_and_snapshot_recalc",
	})
}

func (s *Server) callImportTransactions(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	raw, ok := args["transactions"].([]any)
	if !ok {
		return nil, jsonrpcError(-32602, "invalid_params: transactions array is required")
	}
	const maxImportRows = 5000
	if len(raw) > maxImportRows {
		return nil, jsonrpcError(-32602, "invalid_params: transactions max 5000")
	}
	items := make([]adminsvc.ImportTransaction, 0, len(raw))
	for i, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, jsonrpcError(-32602, "invalid_params: transactions item must be object")
		}
		items = append(items, importTransactionFromArgs(obj))
		_ = i
	}
	result, err := s.admin.ImportTransactions(ctx, items)
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(map[string]any{
		"ok":                result.OK,
		"imported":          result.Imported,
		"total":             result.Total,
		"affected_funds":    result.AffectedFunds,
		"decision_boundary": "facts_only",
		"side_effects":      "transaction_write_and_snapshot_recalc",
	})
}

func (s *Server) callUpdateTransaction(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	result, err := s.admin.UpdateTransaction(ctx, intArg(args, "seq", 0), updateTransactionFromArgs(args))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(map[string]any{
		"ok":                result.OK,
		"updated":           result.Updated,
		"decision_boundary": "facts_only",
		"side_effects":      "transaction_write_and_snapshot_recalc",
	})
}

func (s *Server) callDeleteTransaction(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	result, err := s.admin.DeleteTransaction(ctx, intArg(args, "seq", 0))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(map[string]any{
		"ok":                result.OK,
		"deleted":           result.Deleted,
		"decision_boundary": "facts_only",
		"side_effects":      "transaction_write_and_snapshot_recalc",
	})
}

func importTransactionFromArgs(args map[string]any) adminsvc.ImportTransaction {
	fee := optionalFloatArg(args, "fee")
	if fee == nil {
		defaultFee := 0.0
		fee = &defaultFee
	}
	return adminsvc.ImportTransaction{
		OrderID:           stringArg(args, "order_id"),
		FundCode:          stringArg(args, "fund_code"),
		SecurityCode:      stringArg(args, "security_code"),
		FundName:          stringArg(args, "fund_name"),
		TradeTime:         stringArg(args, "trade_time"),
		ConfirmDate:       stringArg(args, "confirm_date"),
		TradeType:         stringArg(args, "trade_type"),
		Direction:         stringArg(args, "direction"),
		ConfirmAmount:     firstOptionalFloatArg(args, "confirm_amount", "amount"),
		ConfirmShare:      firstOptionalFloatArg(args, "confirm_share", "shares"),
		Fee:               fee,
		SignedCashFlow:    optionalFloatArg(args, "signed_cash_flow"),
		SignedShareChange: optionalFloatArg(args, "signed_share_change"),
	}
}

func updateTransactionFromArgs(args map[string]any) adminsvc.UpdateTransaction {
	return adminsvc.UpdateTransaction{
		TradeTime:     optionalStringArg(args, "trade_time"),
		ConfirmDate:   optionalStringArg(args, "confirm_date"),
		TradeType:     optionalStringArg(args, "trade_type"),
		Direction:     optionalStringArg(args, "direction"),
		ConfirmAmount: firstOptionalFloatArg(args, "confirm_amount", "amount"),
		ConfirmShare:  firstOptionalFloatArg(args, "confirm_share", "shares"),
		Fee:           optionalFloatArg(args, "fee"),
		FundCode:      optionalStringArg(args, "fund_code"),
	}
}

func optionalStringArg(args map[string]any, key string) *string {
	value, ok := args[key]
	if !ok {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return &text
}

func firstOptionalFloatArg(args map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value := optionalFloatArg(args, key); value != nil {
			return value
		}
	}
	return nil
}

func optionalFloatArg(args map[string]any, key string) *float64 {
	value, ok := args[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		return &typed
	case int:
		converted := float64(typed)
		return &converted
	}
	return nil
}
