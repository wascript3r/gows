package router

import (
	"encoding/json"
	"io"
)

type Request struct {
	Method string          `json:"method"`
	Data   json.RawMessage `json:"data"`
}

func ParseRequest(r io.Reader) (*Request, error) {
	req := &Request{}

	err := json.NewDecoder(r).Decode(req)
	if err != nil {
		return nil, err
	}

	return req, nil
}
