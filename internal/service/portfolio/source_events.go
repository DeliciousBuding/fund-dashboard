package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SourceEvent struct {
	ID                  int64   `json:"id"`
	Title               string  `json:"title"`
	URL                 *string `json:"url"`
	Source              string  `json:"source"`
	Snippet             *string `json:"snippet"`
	Query               *string `json:"query"`
	RelatedSecurityCode *string `json:"related_security_code"`
	RelatedSecurityName *string `json:"related_security_name"`
	IsRead              int     `json:"is_read"`
	IsUseful            int     `json:"is_useful"`
	FetchedAt           string  `json:"fetched_at"`
	CreatedAt           string  `json:"created_at"`
}

type CreateSourceEventInput struct {
	Title               string
	URL                 *string
	Source              *string
	Snippet             *string
	Query               *string
	RelatedSecurityCode *string
	RelatedSecurityName *string
}

type GetSourceEventsOptions struct {
	Limit               int
	Offset              int
	RelatedSecurityCode string
	Source              string
	IsRead              *int
	ShowRead            bool
}

type MarkSourceEventInput struct {
	IsRead   *bool
	IsUseful *bool
}

func (s Service) CreateSourceEvent(ctx context.Context, input CreateSourceEventInput) (*SourceEvent, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if len(title) > 500 {
		return nil, fmt.Errorf("title max 500 chars")
	}
	if input.URL != nil && len(*input.URL) > 2000 {
		return nil, fmt.Errorf("url max 2000 chars")
	}
	if input.Snippet != nil && len(*input.Snippet) > 4000 {
		return nil, fmt.Errorf("snippet max 4000 chars")
	}
	if input.Query != nil && len(*input.Query) > 500 {
		return nil, fmt.Errorf("query max 500 chars")
	}
	if input.RelatedSecurityCode != nil {
		code := strings.TrimSpace(*input.RelatedSecurityCode)
		if len(code) > 32 {
			return nil, fmt.Errorf("related_security_code max 32 chars")
		}
		if code == "" {
			input.RelatedSecurityCode = nil
		} else {
			input.RelatedSecurityCode = &code
		}
	}
	if input.RelatedSecurityName != nil {
		name := strings.TrimSpace(*input.RelatedSecurityName)
		if len(name) > 200 {
			return nil, fmt.Errorf("related_security_name max 200 chars")
		}
		if name == "" {
			input.RelatedSecurityName = nil
		} else {
			input.RelatedSecurityName = &name
		}
	}
	source := "websearch"
	if input.Source != nil && *input.Source != "" {
		source = strings.TrimSpace(*input.Source)
		if len(source) > 100 {
			return nil, fmt.Errorf("source max 100 chars")
		}
	}
	fetchedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	input.Title = title
	// RETURNING works for both modernc sqlite and PostgreSQL (pgx rebind).
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO source_events
			(title, url, source, snippet, query, related_security_code, related_security_name, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`,
		input.Title,
		input.URL,
		source,
		input.Snippet,
		input.Query,
		input.RelatedSecurityCode,
		input.RelatedSecurityName,
		fetchedAt,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert source event: %w", err)
	}
	return s.getSourceEventByID(ctx, id)
}

func (s Service) GetSourceEvents(ctx context.Context, opts GetSourceEventsOptions) ([]SourceEvent, error) {
	conditions := []string{}
	args := []any{}
	if opts.RelatedSecurityCode != "" {
		code := strings.TrimSpace(opts.RelatedSecurityCode)
		if len(code) > 32 {
			code = code[:32]
		}
		if code != "" {
			conditions = append(conditions, "related_security_code = ?")
			args = append(args, code)
		}
	}
	if opts.Source != "" {
		source := strings.TrimSpace(opts.Source)
		if len(source) > 100 {
			source = source[:100]
		}
		if source != "" {
			conditions = append(conditions, "source = ?")
			args = append(args, source)
		}
	}
	if !opts.ShowRead {
		isRead := 0
		if opts.IsRead != nil {
			isRead = *opts.IsRead
		}
		conditions = append(conditions, "is_read = ?")
		args = append(args, isRead)
	} else if opts.IsRead != nil {
		conditions = append(conditions, "is_read = ?")
		args = append(args, *opts.IsRead)
	}

	limit := clampSourceEventsLimit(opts.Limit)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	const maxOffset = 100000
	if offset > maxOffset {
		offset = maxOffset
	}
	args = append(args, limit, offset)

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, title, url, source, snippet, query, related_security_code, related_security_name,
			COALESCE(is_read, 0), COALESCE(is_useful, 0), fetched_at, created_at
		FROM source_events
		%s
		ORDER BY fetched_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("query source events: %w", err)
	}
	defer rows.Close()

	var events []SourceEvent
	for rows.Next() {
		event, err := scanSourceEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("source event rows: %w", err)
	}
	return events, nil
}

func (s Service) MarkSourceEventRead(ctx context.Context, id int64, input MarkSourceEventInput) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("id is required")
	}
	sets := []string{}
	args := []any{}
	if input.IsRead != nil {
		sets = append(sets, "is_read = ?")
		args = append(args, boolToInt(*input.IsRead))
	}
	if input.IsUseful != nil {
		sets = append(sets, "is_useful = ?")
		args = append(args, boolToInt(*input.IsUseful))
	}
	if len(sets) == 0 {
		return false, nil
	}
	args = append(args, id)
	result, err := s.db.ExecContext(ctx, "UPDATE source_events SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return false, fmt.Errorf("mark source event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read source event rows affected: %w", err)
	}
	return affected > 0, nil
}

func (s Service) getSourceEventByID(ctx context.Context, id int64) (*SourceEvent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, url, source, snippet, query, related_security_code, related_security_name,
			COALESCE(is_read, 0), COALESCE(is_useful, 0), fetched_at, created_at
		FROM source_events
		WHERE id = ?
	`, id)
	event, err := scanSourceEvent(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

type sourceEventScanner interface {
	Scan(dest ...any) error
}

func scanSourceEvent(scanner sourceEventScanner) (SourceEvent, error) {
	var event SourceEvent
	var url, snippet, query, relatedCode, relatedName sql.NullString
	if err := scanner.Scan(
		&event.ID,
		&event.Title,
		&url,
		&event.Source,
		&snippet,
		&query,
		&relatedCode,
		&relatedName,
		&event.IsRead,
		&event.IsUseful,
		&event.FetchedAt,
		&event.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return SourceEvent{}, err
		}
		return SourceEvent{}, fmt.Errorf("scan source event: %w", err)
	}
	event.URL = stringPtrFromNull(url)
	event.Snippet = stringPtrFromNull(snippet)
	event.Query = stringPtrFromNull(query)
	event.RelatedSecurityCode = stringPtrFromNull(relatedCode)
	event.RelatedSecurityName = stringPtrFromNull(relatedName)
	return event, nil
}

func stringPtrFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func clampSourceEventsLimit(limit int) int {
	if limit <= 0 {
		return 30
	}
	if limit > 100 {
		return 100
	}
	return limit
}
