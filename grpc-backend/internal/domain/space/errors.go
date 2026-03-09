package space

import "errors"

var (
	ErrNotFound      = errors.New("space not found")
	ErrAlreadyExists = errors.New("space already exists")
)
