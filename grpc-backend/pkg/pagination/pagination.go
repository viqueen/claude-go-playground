package pagination

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

// DecodePageToken decodes a base64-encoded offset page token.
// An empty token returns offset 0.
func DecodePageToken(pageToken string) (int32, error) {
	if pageToken == "" {
		return 0, nil
	}
	data, err := base64.StdEncoding.DecodeString(pageToken)
	if err != nil {
		return 0, fmt.Errorf("invalid page token: %w", err)
	}
	offset, err := strconv.ParseInt(string(data), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid page token: %w", err)
	}
	return int32(offset), nil
}

// NextPageToken returns the next page token, or empty string if this is the last page.
func NextPageToken(offset, pageSize int32, resultCount int) string {
	if int32(resultCount) < pageSize {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(int64(offset+pageSize), 10)))
}
