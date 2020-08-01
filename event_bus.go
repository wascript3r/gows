package gows

import (
	"context"
)

type Event uint32

const (
	NewConnectionEvent Event = iota
	NewMessageEvent
	DisconnectEvent
	InvalidEvent
)

func (e Event) String() string {
	switch e {
	case NewConnectionEvent:
		return "NewConnection"
	case NewMessageEvent:
		return "NewMessage"
	case DisconnectEvent:
		return "Disconnect"
	default:
		return "Invalid"
	}
}

type EventHnd func(ctx context.Context, socket *Socket, req *Request)

type EventBus interface {
	Subscribe(Event, EventHnd)
	Publish(Event, context.Context, *Socket, *Request)
	PublishSync(Event, context.Context, *Socket, *Request)
}
