package content

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

func decodePageToken(pageToken string) (int32, error) {
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

func encodePageToken(offset int32) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(int64(offset), 10)))
}

func nextPageToken(offset, pageSize int32, resultCount int) string {
	if int32(resultCount) < pageSize {
		return ""
	}
	return encodePageToken(offset + pageSize)
}
