package eventbus

import (
	"context"
	"sync"

	"github.com/wascript3r/cryptopay/pkg/logger"
	"github.com/wascript3r/gopool"
	"github.com/wascript3r/gows"
)

type WsEventBus struct {
	pool *gopool.Pool
	log  logger.Usecase

	mx       *sync.RWMutex
	handlers map[gows.Event][]gows.EventHnd
}

func NewWsEventBus(pool *gopool.Pool, log logger.Usecase) *WsEventBus {
	return &WsEventBus{
		pool: pool,
		log:  log,

		mx:       &sync.RWMutex{},
		handlers: make(map[gows.Event][]gows.EventHnd),
	}
}

func (w *WsEventBus) Subscribe(ev gows.Event, hnd gows.EventHnd) {
	w.mx.Lock()
	defer w.mx.Unlock()

	w.handlers[ev] = append(w.handlers[ev], hnd)
}

func (w *WsEventBus) Publish(ev gows.Event, ctx context.Context, socket *gows.Socket, req *gows.Request) {
	w.mx.RLock()
	defer w.mx.RUnlock()

	hnds := w.handlers[ev]
	count := len(hnds)
	if count == 0 {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(count)

	for _, h := range hnds {
		h := h
		err := w.pool.Schedule(func() {
			h(ctx, socket, req)
			wg.Done()
		})
		if err != nil {
			w.log.Error("Cannot publish ws %s event because of pool schedule error: %s", ev, err)
			wg.Done()
		}
	}

	wg.Wait()
}

func (w *WsEventBus) PublishSync(ev gows.Event, ctx context.Context, socket *gows.Socket, req *gows.Request) {
	w.mx.RLock()
	defer w.mx.RUnlock()

	hnds := w.handlers[ev]
	count := len(hnds)
	if count == 0 {
		return
	}

	for _, h := range hnds {
		h(ctx, socket, req)
	}
}
