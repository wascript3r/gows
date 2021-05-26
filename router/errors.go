package router

import "errors"

var (
	ErrBadRequest     = errors.New("bad request")
	ErrMethodNotFound = errors.New("method not found")
)
