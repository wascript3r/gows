package gows

import (
	"io"

	"github.com/gobwas/ws"
)

type Request struct {
	Op     ws.OpCode
	Reader io.Reader
}

func NewRequest(op ws.OpCode, r io.Reader) *Request {
	return &Request{op, r}
}
