package gows

import (
	"time"
)

const (
	DefaultCooldown        = time.Second
	DefaultScheduleTimeout = time.Millisecond
	DefaultIOTimeout       = 10 * time.Second

	DefaultConnIdleTime = 30 * time.Second

	DefaultWheelTick = time.Millisecond
	DefaultWheelSize = 1000
)

type Options struct {
	Cooldown        time.Duration
	ScheduleTimeout time.Duration
	IOTimeout       time.Duration

	ConnIdleTime time.Duration

	WheelTick time.Duration
	WheelSize int64
}

type Option func(*Options)

func newOptions(opt ...Option) *Options {
	opts := &Options{}

	for _, o := range opt {
		o(opts)
	}

	if opts.Cooldown == 0 {
		opts.Cooldown = DefaultCooldown
	}
	if opts.ScheduleTimeout == 0 {
		opts.ScheduleTimeout = DefaultScheduleTimeout
	}
	if opts.IOTimeout == 0 {
		opts.IOTimeout = DefaultIOTimeout
	}
	if opts.ConnIdleTime == 0 {
		opts.ConnIdleTime = DefaultConnIdleTime
	}
	if opts.WheelTick == 0 {
		opts.WheelTick = DefaultWheelTick
	}
	if opts.WheelSize == 0 {
		opts.WheelSize = DefaultWheelSize
	}

	return opts
}

func Cooldown(t time.Duration) Option {
	return func(o *Options) {
		o.Cooldown = t
	}
}

func ScheduleTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.ScheduleTimeout = t
	}
}

func IOTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.IOTimeout = t
	}
}

func ConnIdleTime(t time.Duration) Option {
	return func(o *Options) {
		o.ConnIdleTime = t
	}
}

func WheelTick(t time.Duration) Option {
	return func(o *Options) {
		o.WheelTick = t
	}
}

func WheelSize(s int64) Option {
	return func(o *Options) {
		o.WheelSize = s
	}
}
