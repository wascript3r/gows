package router

import (
	"github.com/wascript3r/gostr"
	"github.com/wascript3r/gows"
)

type jsonError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

func newJsonError(err Error) *jsonError {
	return &jsonError{
		Name:    err.Name(),
		Message: gostr.UpperFirst(err.Error()),
	}
}

type Params map[string]interface{}

type Response struct {
	Error  *jsonError  `json:"error"`
	Method *string     `json:"method"`
	Data   interface{} `json:"data"`
}

func WriteErr(s *gows.Socket, err Error, method *string) error {
	jsonErr := newJsonError(err)

	return s.WriteJSON(Response{
		Error:  jsonErr,
		Method: method,
		Data:   nil,
	})
}

func WriteBadRequest(s *gows.Socket, method *string) error {
	return WriteErr(s, BadRequestError, method)
}

func WriteInternalError(s *gows.Socket, method *string) error {
	return WriteErr(s, InternalServerError, method)
}

func WriteRes(s *gows.Socket, method *string, p interface{}) error {
	return s.WriteJSON(Response{
		Error:  nil,
		Method: method,
		Data:   p,
	})
}
