package gows

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/RussellLuo/timingwheel"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
	"github.com/wascript3r/gopool"
)

var (
	ErrUnexpectedClose = errors.New("unexpected close")
)

type Socket struct {
	mx         *sync.RWMutex
	sConn      SafeConn
	idleTime   time.Duration
	activeTime time.Time
	data       map[string]interface{}

	pool   *gopool.Pool
	poller netpoll.Poller
	desc   *netpoll.Desc
	tWheel *timingwheel.TimingWheel
	timer  *timingwheel.Timer
	ev     EventBus
}

func NewSocket(sConn SafeConn, idleTime time.Duration, pool *gopool.Pool, poller netpoll.Poller, tWheel *timingwheel.TimingWheel, ev EventBus) *Socket {
	s := &Socket{
		mx:         &sync.RWMutex{},
		sConn:      sConn,
		idleTime:   idleTime,
		activeTime: time.Time{},
		data:       make(map[string]interface{}),

		pool:   pool,
		poller: poller,
		desc:   nil,
		tWheel: tWheel,
		ev:     ev,
	}
	return s
}

func (s *Socket) startPoll(ctx context.Context) error {
	// Using conn instead of safeConn because of "could not get file descriptor" error
	d, err := netpoll.HandleReadOnce(s.sConn.Conn)
	if err != nil {
		return err
	}
	s.desc = d

	return s.poller.Start(s.desc, s.handlePollEvent(ctx))
}

func (s *Socket) handlePollEvent(ctx context.Context) func(netpoll.Event) {
	return func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			s.stopPoll()
			if err := s.sConn.Close(); err == nil {
				s.ev.Publish(DisconnectEvent, ctx, s, nil)
			}
			return
		}

		err := s.pool.Schedule(func() {
			op, r, err := s.getReaderUnsafe()
			if err != nil {
				s.stopPoll()
				if err := s.sConn.Close(); err == nil {
					s.ev.PublishSync(DisconnectEvent, ctx, s, nil)
				}
				return
			}

			defer s.poller.Resume(s.desc)

			if r == nil || !op.IsData() {
				return
			}

			s.setActiveTime()

			s.ev.PublishSync(
				NewMessageEvent,
				ctx,
				s,
				NewRequest(op, r),
			)
		})
		if err != nil || ctx.Err() != nil {
			s.Disconnect(ctx, ErrUnexpectedClose.Error())
		}
	}
}

func (s *Socket) startTimeoutTimer(ctx context.Context) {
	s.timer = s.tWheel.AfterFunc(
		s.idleTime,
		s.handleTimeout(ctx),
	)
}

func (s *Socket) handleTimeout(ctx context.Context) func() {
	return func() {
		if s.isIdle() {
			s.stopPoll()
			if err := s.sConn.Close(); err == nil {
				s.ev.PublishSync(DisconnectEvent, ctx, s, nil)
			}
			return
		}

		s.timer = s.tWheel.AfterFunc(
			s.idleTime,
			s.handleTimeout(ctx),
		)
	}
}

func (s *Socket) stopPoll() {
	s.poller.Stop(s.desc)
	s.desc.Close()

	if s.timer != nil {
		s.timer.Stop()
	}
}

func (s *Socket) getReaderUnsafe() (ws.OpCode, io.Reader, error) {
	h, r, err := wsutil.NextReader(s.sConn, ws.StateServerSide)
	if err != nil {
		return 0, nil, err
	}
	if h.OpCode.IsControl() {
		return h.OpCode, nil, wsutil.ControlFrameHandler(s.sConn, ws.StateServerSide)(h, r)
	}

	return h.OpCode, r, nil
}

func (s *Socket) ReadJSON(v interface{}) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	_, r, err := s.getReaderUnsafe()
	if err != nil {
		return err
	}

	return json.NewDecoder(r).Decode(v)
}

func (s *Socket) getWriterUnsafe(op ws.OpCode) *wsutil.Writer {
	return wsutil.NewWriter(s.sConn, ws.StateServerSide, op)
}

func (s *Socket) write(op ws.OpCode, p []byte) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	return wsutil.WriteServerMessage(s.sConn, op, p)
}

func (s *Socket) Write(p []byte) error {
	return s.write(ws.OpText, p)
}

func (s *Socket) WriteJSON(v interface{}) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	w := s.getWriterUnsafe(ws.OpText)
	enc := json.NewEncoder(w)

	if err := enc.Encode(v); err != nil {
		return err
	}

	return w.Flush()
}

func (s *Socket) Disconnect(ctx context.Context, reason string) {
	s.write(ws.OpClose, ws.NewCloseFrameBody(1000, reason))

	s.stopPoll()
	if err := s.sConn.Close(); err == nil {
		s.ev.PublishSync(DisconnectEvent, ctx, s, nil)
	}
}

func (s *Socket) isIdle() bool {
	s.mx.Lock()
	defer s.mx.Unlock()

	return time.Since(s.activeTime) > s.idleTime
}

func (s *Socket) setActiveTime() {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.activeTime = time.Now()
}

func (s *Socket) GetData(key string) (interface{}, bool) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	v, ok := s.data[key]
	return v, ok
}

func (s *Socket) SetData(key string, val interface{}) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.data[key] = val
}
