package middleware

import (
	"github.com/wascript3r/gows/router"
)

type Middleware func(next router.Handler) router.Handler

type Stack struct {
	middlewares []Middleware
}

func New() *Stack {
	return &Stack{nil}
}

func (s *Stack) Use(m Middleware) {
	s.middlewares = append(s.middlewares, m)
}

func (s *Stack) Wrap(fn router.Handler) router.Handler {
	l := len(s.middlewares)
	if l == 0 {
		return fn
	}

	var result router.Handler
	result = s.middlewares[l-1](fn)

	for i := l - 2; i >= 0; i-- {
		result = s.middlewares[i](result)
	}

	return result
}
