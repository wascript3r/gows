package router

import "errors"

var (
	ErrBadRequest     = errors.New("bad request")
	ErrInternalError  = errors.New("internal error")
	ErrMethodNotFound = errors.New("method not found")
)
