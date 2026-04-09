package server

import (
	pb "github.com/digital-dream-labs/api/go/chipperpb"
)

// Server defines the service used.
type Server struct {
	pb.UnimplementedChipperGrpcServer
}

// New accepts a list of args and returns the service
func New(opts ...Option) (*Server, error) {
	cfg := options{
		//log: log.Base(),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	s := Server{}
	return &s, nil

}
