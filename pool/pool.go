package pool

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/wascript3r/cryptopay/pkg/logger"
	"github.com/wascript3r/gopool"
	"github.com/wascript3r/gows"
	"github.com/wascript3r/gows/router"
)

type SocketFilter func(socket *gows.Socket) bool

type writeReq struct {
	*router.Response
	SocketFilter
}

type Pool struct {
	pool *gopool.Pool
	log  logger.Usecase

	mx        *sync.RWMutex
	sockets   map[*gows.Socket]struct{}
	writeJSON chan writeReq
}

func New(ctx context.Context, pool *gopool.Pool, log logger.Usecase, ev gows.EventBus) (*Pool, error) {
	p := &Pool{
		pool: pool,
		log:  log,

		mx:        &sync.RWMutex{},
		sockets:   make(map[*gows.Socket]struct{}),
		writeJSON: make(chan writeReq, 1),
	}

	ev.Subscribe(gows.NewConnectionEvent, p.handleNewConn)
	ev.Subscribe(gows.DisconnectEvent, p.handleDisconnect)

	err := p.start(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Pool) start(ctx context.Context) error {
	return p.pool.Schedule(func() {
		var err error

		for {
			select {
			case r := <-p.writeJSON:
				err = p.writeAllJSON(r)
				if err != nil {
					p.log.Error("Cannot write message to all sockets because an error occurred: %s", err)
				}

			case <-ctx.Done():
				p.stop()
				p.log.Info("Stopped socket pool...")
				return
			}
		}
	})
}

func (p *Pool) stop() {
	close(p.writeJSON)
}

func (p *Pool) handleNewConn(_ context.Context, socket *gows.Socket, _ *gows.Request) {
	p.mx.Lock()
	defer p.mx.Unlock()

	p.sockets[socket] = struct{}{}
}

func (p *Pool) handleDisconnect(_ context.Context, socket *gows.Socket, _ *gows.Request) {
	p.mx.Lock()
	defer p.mx.Unlock()

	delete(p.sockets, socket)
}

func (p *Pool) NumSockets() int {
	p.mx.RLock()
	defer p.mx.RUnlock()

	return len(p.sockets)
}

func (p *Pool) writeAllJSON(r writeReq) error {
	bs, err := json.Marshal(r.Response)
	if err != nil {
		return err
	}

	p.mx.RLock()
	defer p.mx.RUnlock()

	count := len(p.sockets)
	if count == 0 {
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(count)

	for s := range p.sockets {
		if r.SocketFilter != nil && !r.SocketFilter(s) {
			continue
		}

		s := s
		p.pool.Schedule(func() {
			s.Write(bs)
			wg.Done()
		})
	}

	wg.Wait()
	return nil
}

func (p *Pool) WriteAllJSON(r *router.Response) {
	p.writeJSON <- writeReq{r, nil}
}

func (p *Pool) WriteJSON(r *router.Response, f SocketFilter) {
	p.writeJSON <- writeReq{r, f}
}
