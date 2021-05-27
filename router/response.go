package router

import (
	"github.com/wascript3r/gostr"
	"github.com/wascript3r/gows"
)

type Params map[string]interface{}

type Response struct {
	Err    *string `json:"e"`
	Method string  `json:"m"`
	Params Params  `json:"p"`
}

func WriteErr(s *gows.Socket, error, method string) error {
	error = gostr.UpperFirst(error)

	return s.WriteJSON(Response{
		Err:    &error,
		Method: method,
		Params: nil,
	})
}

func WriteBadRequest(s *gows.Socket, method string) error {
	return WriteErr(s, ErrBadRequest.Error(), method)
}

func WriteInternalError(s *gows.Socket, method string) error {
	return WriteErr(s, ErrInternalError.Error(), method)
}

func WriteRes(s *gows.Socket, method string, p Params) error {
	return s.WriteJSON(Response{
		Err:    nil,
		Method: method,
		Params: p,
	})
}
