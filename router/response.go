package router

import (
	"github.com/wascript3r/gostr"
	"github.com/wascript3r/gows"
)

type Params map[string]interface{}

type Response struct {
	Err    *string `json:"e"`
	Params Params  `json:"p"`
}

func WriteErr(s *gows.Socket, e string) error {
	e = gostr.UpperFirst(e)

	return s.WriteJSON(Response{
		Err:    &e,
		Params: nil,
	})
}

func WriteBadRequest(s *gows.Socket) error {
	return WriteErr(s, ErrBadRequest.Error())
}

func WriteInternalError(s *gows.Socket) error {
	return WriteErr(s, ErrInternalError.Error())
}

func WriteRes(s *gows.Socket, p Params) error {
	return s.WriteJSON(Response{
		Err:    nil,
		Params: p,
	})
}
