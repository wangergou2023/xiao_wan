package server

import "github.com/digital-dream-labs/hugh/log"

type options struct {
	log         log.Logger
}

// Option is the list of options
type Option func(*options)

// WithLogger sets the logger
func WithLogger(l log.Logger) Option {
	return func(o *options) {
		o.log = l
	}
}
