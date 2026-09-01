package datasource

import (
	"fmt"
	"io"
)

// readBodyLimited reads r up to limit bytes and errors if the body exceeds the
// limit, instead of silently truncating like a bare io.LimitReader.
func readBodyLimited(r io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return body, nil
}
