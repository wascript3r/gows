package gows

import (
	"context"
	"net"
	"time"

	"github.com/RussellLuo/timingwheel"
	"github.com/gobwas/ws"
	"github.com/mailru/easygo/netpoll"
	"github.com/wascript3r/gopool"
)

type Server struct {
	pool     *gopool.Pool
	listener net.Listener
	poller   netpoll.Poller
	desc     *netpoll.Desc
	tWheel   *timingwheel.TimingWheel

	ev   EventBus
	opts *Options
}

func NewServer(pool *gopool.Pool, listener net.Listener, ev EventBus, opt ...Option) (*Server, error) {
	p, err := netpoll.New(nil)
	if err != nil {
		return nil, err
	}

	opts := newOptions(opt...)

	return &Server{
		pool:     pool,
		listener: listener,
		poller:   p,
		tWheel:   timingwheel.NewTimingWheel(opts.WheelTick, opts.WheelSize),

		ev:   ev,
		opts: opts,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.tWheel.Start()

	err := s.startPoll(ctx)
	if err != nil {
		return err
	}

	return s.pool.Schedule(func() {
		s.waitForStop(ctx)
	})
}

func (s *Server) startPoll(ctx context.Context) error {
	d, err := netpoll.HandleListener(s.listener, netpoll.EventRead)
	if err != nil {
		return err
	}
	s.desc = d

	return s.poller.Start(s.desc, s.handlePollEvent(ctx))
}

func (s *Server) handlePollEvent(ctx context.Context) func(netpoll.Event) {
	connErr := make(chan error, 1)

	return func(ev netpoll.Event) {
		// Pool schedule error
		err := s.pool.ScheduleTimeout(s.opts.ScheduleTimeout, func() {
			// Connection accept error
			conn, err := s.listener.Accept()
			if err != nil {
				connErr <- err
				return
			}
			connErr <- nil

			s.handleConn(ctx, conn)
		})
		if err != nil {
			if err == gopool.ErrScheduleTimeout {
				s.cooldown(ctx, s.opts.Cooldown)
			}
			return
		}

		if err := <-connErr; err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				s.cooldown(ctx, s.opts.Cooldown)
			}
		}
	}
}

func (s *Server) waitForStop(ctx context.Context) {
	select {
	case <-ctx.Done():
		s.stopPoll()
	}
}

func (s *Server) stopPoll() {
	s.poller.Stop(s.desc)
	s.desc.Close()
}

func (s *Server) cooldown(ctx context.Context, t time.Duration) {
	timer := time.NewTimer(t)
	select {
	case <-ctx.Done():
		timer.Stop()
	case <-timer.C:
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) error {
	safeConn := NewSafeConn(conn, s.opts.IOTimeout)
	socket := NewSocket(
		safeConn,
		s.opts.ConnIdleTime,

		s.pool,
		s.poller,
		s.tWheel,
		s.ev,
	)

	_, err := ws.Upgrade(safeConn)
	if err != nil {
		safeConn.Close()
		return err
	}

	err = socket.startPoll(ctx)
	if err != nil {
		safeConn.Close()
		return err
	}

	if s.opts.ConnIdleTime > 0 {
		socket.setActiveTime()
		socket.startTimeoutTimer(ctx)
	}

	s.ev.PublishSync(NewConnectionEvent, ctx, socket, nil)
	return nil
}
