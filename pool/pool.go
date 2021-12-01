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
	ErrRoomAlreadyExists  = errors.New("room already exists")
	ErrRoomDoesNotExist   = errors.New("room does not exist")
	ErrRoomAlreadyJoined  = errors.New("room is already joined")
	ErrRoomIsNotJoined    = errors.New("room is not joined")
)

type RoomName string

type RoomConfig struct {
	name            RoomName
	deleteWhenEmpty bool
}

type room struct {
	config  *RoomConfig
	sockets map[gows.UUID]*socket
}

func NewRoomConfig(name RoomName, deleteWhenEmpty bool) *RoomConfig {
	return &RoomConfig{
		name:            name,
		deleteWhenEmpty: deleteWhenEmpty,
	}
}

func (r *RoomConfig) Name() RoomName {
	return r.name
}

func (r *RoomConfig) NameString() string {
	return string(r.name)
}

type socket struct {
	*gows.Socket
	rooms []RoomName
}

func (s socket) isJoined(name RoomName) bool {
	for _, r := range s.rooms {
		if r == name {
			return true
		}
	}
	return false
}

func (s *socket) joinRoom(name RoomName) error {
	if s.isJoined(name) {
		return ErrRoomAlreadyJoined
	}
	s.rooms = append(s.rooms, name)
	return nil
}

func (s *socket) leaveRoom(name RoomName) error {
	for i, r := range s.rooms {
		if r == name {
			s.rooms = append(s.rooms[:i], s.rooms[i+1:]...)
			return nil
		}
	}
	return ErrRoomIsNotJoined
}

type SocketFilter func(socket *gows.Socket) bool

type emitReq struct {
	roomName *RoomName
	res      *router.Response
	filter   SocketFilter
}

type Pool struct {
	pool *gopool.Pool
	log  logger.Usecase

	mx      *sync.RWMutex
	sockets map[gows.UUID]*socket
	rooms   map[RoomName]*room
	emitC   chan emitReq
}

func NewPool(ctx context.Context, pool *gopool.Pool, log logger.Usecase, ev gows.EventBus) (*Pool, error) {
	p := &Pool{
		pool: pool,
		log:  log,

		mx:      &sync.RWMutex{},
		sockets: make(map[gows.UUID]*socket),
		rooms:   make(map[RoomName]*room),
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

	for _, name := range ss.rooms {
		r, ok := p.rooms[name]
		if !ok {
			continue
		}

		delete(r.sockets, s.GetUUID())
		if r.config.deleteWhenEmpty && len(r.sockets) == 0 {
			delete(p.rooms, name)
		}
	}
	delete(p.sockets, s.GetUUID())
}

func (p *Pool) NumSockets() int {
	p.mx.RLock()
	defer p.mx.RUnlock()

	return len(p.sockets)
}

func (p *Pool) RoomNumSockets(name RoomName) (int, error) {
	p.mx.RLock()
	defer p.mx.RUnlock()

	r, ok := p.rooms[name]
	if !ok {
		return 0, ErrRoomDoesNotExist
	}

	return len(r.sockets), nil
}

func (p *Pool) emitMany(r emitReq) error {
	bs, err := json.Marshal(r.res)
	if err != nil {
		return err
	}

	p.mx.RLock()
	defer p.mx.RUnlock()

	var sockets map[gows.UUID]*socket
	if r.roomName == nil {
		sockets = p.sockets
	} else {
		room, ok := p.rooms[*r.roomName]
		if !ok {
			return nil
		}
		sockets = room.sockets
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

func (p *Pool) CreateRoom(c *RoomConfig) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	if _, ok := p.rooms[c.name]; ok {
		return ErrRoomAlreadyExists
	}

	p.rooms[c.name] = &room{
		config:  c,
		sockets: make(map[gows.UUID]*socket),
	}
	return nil
}

func (p *Pool) DeleteRoom(name RoomName) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	r, ok := p.rooms[name]
	if !ok {
		return ErrRoomDoesNotExist
	}

	for _, s := range r.sockets {
		s.leaveRoom(name)
	}

	delete(p.rooms, name)
	return nil
}

func (p *Pool) JoinRoom(s *gows.Socket, name RoomName) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	r, ok := p.rooms[name]
	if !ok {
		return ErrRoomDoesNotExist
	}

	ss, ok := p.sockets[s.GetUUID()]
	if !ok {
		return ErrSocketDoesNotExist
	}

	if err := ss.joinRoom(name); err != nil {
		return err
	}

	r.sockets[s.GetUUID()] = ss
	return nil
}

func (p *Pool) LeaveRoom(s *gows.Socket, name RoomName) error {
	p.mx.Lock()
	defer p.mx.Unlock()

	r, ok := p.rooms[name]
	if !ok {
		return ErrRoomDoesNotExist
	}

	ss, ok := p.sockets[s.GetUUID()]
	if !ok {
		return ErrSocketDoesNotExist
	}

	if err := ss.leaveRoom(name); err != nil {
		return err
	}

	delete(r.sockets, s.GetUUID())
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
		roomName: nil,
		res:      res,
		filter:   nil,
	}
}

func (p *Pool) EmitAllFilter(res *router.Response, f SocketFilter) {
	p.emitC <- emitReq{
		roomName: nil,
		res:      res,
		filter:   f,
	}
}

func (p *Pool) EmitRoom(name RoomName, res *router.Response) {
	p.emitC <- emitReq{
		roomName: &name,
		res:      res,
		filter:   nil,
	}
}

func (p *Pool) EmitRoomFilter(name RoomName, res *router.Response, f SocketFilter) {
	p.emitC <- emitReq{
		roomName: &name,
		res:      res,
		filter:   f,
	}
}
