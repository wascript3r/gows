package gows

import (
	"net"
	"time"
)

type SafeConn struct {
	net.Conn
	t time.Duration
}

func NewSafeConn(conn net.Conn, t time.Duration) SafeConn {
	return SafeConn{conn, t}
}

func (s SafeConn) Write(p []byte) (int, error) {
	if s.t != 0 {
		if err := s.Conn.SetWriteDeadline(time.Now().Add(s.t)); err != nil {
			return 0, err
		}
	}
	return s.Conn.Write(p)
}

func (s SafeConn) Read(p []byte) (int, error) {
	if s.t != 0 {
		if err := s.Conn.SetReadDeadline(time.Now().Add(s.t)); err != nil {
			return 0, err
		}
	}
	return s.Conn.Read(p)
}

func (s SafeConn) SetReadTimeout(t time.Duration) error {
	return s.Conn.SetReadDeadline(time.Now().Add(t))
}

func (s SafeConn) SetWriteTimeout(t time.Duration) error {
	return s.Conn.SetWriteDeadline(time.Now().Add(t))
}
