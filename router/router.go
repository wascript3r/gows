package router

import (
	"context"
	"errors"
	"sync"

	"github.com/wascript3r/gows"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrMethodNotFound = errors.New("method not found")
)

type Handler func(context.Context, *gows.Socket, *Request)

type MiddleWare func(context.Context, *gows.Socket, *Request) (next bool)

type Router struct {
	mx          *sync.RWMutex
	methods     map[string]Handler
	middlewares []MiddleWare
}

func New(ev gows.EventBus) *Router {
	r := &Router{
		mx:          &sync.RWMutex{},
		methods:     make(map[string]Handler),
		middlewares: nil,
	}

	ev.Subscribe(gows.NewMessageEvent, r.handle)
	return r
}

func (r *Router) handleMiddleWares(ctx context.Context, s *gows.Socket, req *Request) (next bool) {
	r.mx.RLock()
	middlewares := r.middlewares
	r.mx.RUnlock()

	for _, m := range middlewares {
		if next := m(ctx, s, req); !next {
			return false
		}
	}

	return true
}

func (r *Router) handle(ctx context.Context, s *gows.Socket, req *gows.Request) {
	pr, err := ParseRequest(req.Reader)
	if err != nil {
		WriteErr(s, ErrInvalidRequest.Error())
		return
	}

	r.mx.RLock()
	hnd, ok := r.methods[pr.Method]
	r.mx.RUnlock()

	if !ok {
		WriteErr(s, ErrMethodNotFound.Error())
		return
	}

	if next := r.handleMiddleWares(ctx, s, pr); !next {
		return
	}

	hnd(ctx, s, pr)
}

func (r *Router) HandleMethod(method string, hnd Handler) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.methods[method] = hnd
}

func (r *Router) PushMiddleWare(m MiddleWare) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.middlewares = append(r.middlewares, m)
}
