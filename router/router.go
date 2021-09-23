package router

import (
	"context"
	"errors"
	"sync"

	"github.com/wascript3r/gows"
)

const (
	PingMethod = "ping"
)

var (
	ErrUseOfReservedMethod = errors.New("use of reserved method")
)

type Handler func(context.Context, *gows.Socket, *Request)

type Router struct {
	mx      *sync.RWMutex
	methods map[string]Handler
}

func New(ev gows.EventBus) *Router {
	r := &Router{
		mx:      &sync.RWMutex{},
		methods: make(map[string]Handler),
	}
	r.methods[PingMethod] = EmptyHandler

	ev.Subscribe(gows.NewMessageEvent, r.handle)
	return r
}

func (r *Router) handle(ctx context.Context, s *gows.Socket, req *gows.Request) {
	pr, err := ParseRequest(req.Reader)
	if err != nil {
		WriteBadRequest(s, nil)
		return
	}

	r.mx.RLock()
	hnd, ok := r.methods[pr.Method]
	r.mx.RUnlock()

	if !ok {
		WriteErr(s, ErrMethodNotFound, nil)
		return
	}

	hnd(ctx, s, pr)
}

func (r *Router) HandleMethod(method string, hnd Handler) {
	if method == PingMethod {
		panic(ErrUseOfReservedMethod)
	}

	r.mx.Lock()
	defer r.mx.Unlock()

	r.methods[method] = hnd
}
