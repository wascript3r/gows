package pool

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/wascript3r/cryptopay/pkg/logger"
	"github.com/wascript3r/gopool"
	"github.com/wascript3r/gows"
	"github.com/wascript3r/gows/router"
)

var (
	ErrSocketDoesNotExist = errors.New("socket does not exist")
	ErrRoomDoesNotExist   = errors.New("room does not exist")
	ErrRoomAlreadyJoined  = errors.New("room is already joined")
	ErrRoomIsNotJoined    = errors.New("room is not joined")
)

type Room struct {
	Name            string
	DeleteWhenEmpty bool
}

func NewRoom(name string, deleteWhenEmpty bool) *Room {
	return &Room{name, deleteWhenEmpty}
}

type socket struct {
	*gows.Socket
	rooms []*Room
}

func (s socket) isJoined(room *Room) bool {
	for _, r := range s.rooms {
		if r == room {
			return true
		}
	}
	return false
}

func (s *socket) joinRoom(room *Room) error {
	if s.isJoined(room) {
		return ErrRoomAlreadyJoined
	}
	s.rooms = append(s.rooms, room)
	return nil
}

func (s *socket) leaveRoom(room *Room) error {
	for i, r := range s.rooms {
		if r == room {
			s.rooms = append(s.rooms[:i], s.rooms[i+1:]...)
			return nil
		}
	}
	return ErrRoomIsNotJoined
}

type SocketFilter func(socket *gows.Socket) bool

type emitReq struct {
	room   *Room
	res    *router.Response
	filter SocketFilter
}

type Pool struct {
	pool *gopool.Pool
	log  logger.Usecase

	mx      *sync.RWMutex
	sockets map[gows.UUID]*socket
	rooms   map[*Room]map[gows.UUID]*socket
	emitC   chan emitReq
}

func New(ctx context.Context, pool *gopool.Pool, log logger.Usecase, ev gows.EventBus) (*Pool, error) {
	p := &Pool{
		pool: pool,
		log:  log,

		mx:      &sync.RWMutex{},
		sockets: make(map[gows.UUID]*socket),
		rooms:   make(map[*Room]map[gows.UUID]*socket),
		emitC:   make(chan emitReq, 1),
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
			case r := <-p.emitC:
				err = p.emitMany(r)
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
	close(p.emitC)
}

func (p *Pool) handleNewConn(_ context.Context, s *gows.Socket, _ *gows.Request) {
	p.mx.Lock()
	defer p.mx.Unlock()

	p.sockets[s.GetUUID()] = &socket{s, nil}
}

func (p *Pool) handleDisconnect(_ context.Context, s *gows.Socket, _ *gows.Request) {
	p.mx.Lock()
	defer p.mx.Unlock()

	ss, ok := p.sockets[s.GetUUID()]
	if !ok {
		return
	}

	for _, r := range ss.rooms {
		delete(p.rooms[r], s.GetUUID())
		if r.DeleteWhenEmpty && len(p.rooms[r]) == 0 {
			delete(p.rooms, r)
		}
	}
	delete(p.sockets, s.GetUUID())
}

func (p *Pool) NumSockets() int {
	p.mx.RLock()
	defer p.mx.RUnlock()

	return len(p.sockets)
}

func (p *Pool) RoomNumSockets(r *Room) (int, error) {
	p.mx.RLock()
	defer p.mx.RUnlock()

	sockets, ok := p.rooms[r]
	if !ok {
		return 0, ErrRoomDoesNotExist
	}

	return len(sockets), nil
}

func (p *Pool) emitMany(r emitReq) error {
	bs, err := json.Marshal(r.res)
	if err != nil {
		return err
	}

	p.mx.RLock()
	defer p.mx.RUnlock()

	var sockets map[gows.UUID]*socket
	if r.room == nil {
		sockets = p.sockets
	} else {
		sockets = p.rooms[r.room]
	}

	count := len(sockets)
	if count == 0 {
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(count)

	for _, ss := range sockets {
		if r.filter != nil && !r.filter(ss.Socket) {
			continue
		}

		ss := ss
		p.pool.Schedule(func() {
			ss.Write(bs)
			wg.Done()
		})
	}

	wg.Wait()
	return nil
}

func (p *Pool) CreateRoom(r *Room) {
	p.mx.Lock()
	defer p.mx.Unlock()

	p.rooms[r] = make(map[gows.UUID]*socket)
}

func (p *Pool) DeleteRoom(r *Room) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	sockets, ok := p.rooms[r]
	if !ok {
		return ErrRoomDoesNotExist
	}

	for _, ss := range sockets {
		ss.leaveRoom(r)
	}

	delete(p.rooms, r)
	return nil
}

func (p *Pool) JoinRoom(s *gows.Socket, r *Room) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	if _, ok := p.rooms[r]; !ok {
		return ErrRoomDoesNotExist
	}

	ss, ok := p.sockets[s.GetUUID()]
	if !ok {
		return ErrSocketDoesNotExist
	}

	if err := ss.joinRoom(r); err != nil {
		return err
	}

	p.rooms[r][s.GetUUID()] = ss
	return nil
}

func (p *Pool) LeaveRoom(s *gows.Socket, r *Room) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	if _, ok := p.rooms[r]; !ok {
		return ErrRoomDoesNotExist
	}

	ss, ok := p.sockets[s.GetUUID()]
	if !ok {
		return ErrSocketDoesNotExist
	}

	if err := ss.leaveRoom(r); err != nil {
		return err
	}

	delete(p.rooms[r], s.GetUUID())
	return nil
}

func (p *Pool) EmitUUID(uuid gows.UUID, res *router.Response) error {
	p.mx.RLock()
	defer p.mx.RUnlock()

	ss, ok := p.sockets[uuid]
	if !ok {
		return ErrSocketDoesNotExist
	}

	return ss.WriteJSON(res)
}

func (p *Pool) EmitAll(res *router.Response) {
	p.emitC <- emitReq{
		room:   nil,
		res:    res,
		filter: nil,
	}
}

func (p *Pool) EmitAllFilter(res *router.Response, f SocketFilter) {
	p.emitC <- emitReq{
		room:   nil,
		res:    res,
		filter: f,
	}
}

func (p *Pool) EmitRoom(r *Room, res *router.Response) {
	p.emitC <- emitReq{
		room:   r,
		res:    res,
		filter: nil,
	}
}

func (p *Pool) EmitRoomFilter(r *Room, res *router.Response, f SocketFilter) {
	p.emitC <- emitReq{
		room:   r,
		res:    res,
		filter: f,
	}
}
