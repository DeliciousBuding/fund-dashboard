package portfolio

import "database/sql"

// nullable* helpers normalize database/sql nullable scalars into their domain
// equivalents. They are shared across every read path that hydrates a DTO from
// a *sql.Rows scan, so they live in their own file rather than in any single
// DTO file.

func nullableStringValue(value sql.NullString, fallback string) string {
	if !value.Valid {
		return fallback
	}
	return value.String
}

func nullableStringValuePtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func dateOnlyNullablePtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	date := dateOnly(value.String)
	return &date
}

func nullableFloat64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func nullableIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}
