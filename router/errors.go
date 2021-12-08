package router

import (
	"errors"

	"github.com/wascript3r/cryptopay/pkg/errcode"
)

type Error interface {
	error
	Name() string
	Original() error
}

var (
	MethodNotFoundError = errcode.New(
		"method_not_found",
		errors.New("method not found"),
	)

	BadRequestError = errcode.New(
		"bad_request",
		errors.New("bad request"),
	)

	InternalServerError = errcode.New(
		"internal_server_error",
		errors.New("internal server error"),
	)
)
