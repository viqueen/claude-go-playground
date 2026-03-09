package collaboration

import "errors"

var (
	ErrSpaceNotFound   = errors.New("space not found")
	ErrContentNotFound = errors.New("content not found")
)
