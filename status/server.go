package status

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lightninglabs/lightning-terminal/litrpc"
)

// Server is an implementation of the litrpc.StatusServer which can be
// queried for the status of various LiT sub-servers.
type Server struct {
	litrpc.UnimplementedStatusServer

	servers map[string]*Status
	mu      sync.RWMutex
}

// Status represents the status of a sub-server.
type Status struct {
	Running   bool
	Err       string
	Timestamp time.Time
}

// newSubServerStatus constructs a new subServerStatus.
func NewStatus() *Status {
	return &Status{}
}

// NewServer constructs a new statusServer.
func NewServer() *Server {
	return &Server{
		servers: map[string]*Status{},
	}
}

// RegisterServer will create a new sub-server entry for the statusServer to
// keep track of.
func (s *Server) RegisterServer(name string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.servers[name] = NewStatus()
}

// SubServerState queries the current status of a given sub-server.
//
// NOTE: this is part of the litrpc.StatusServer interface.
func (s *Server) SubServerState(_ context.Context,
	_ *litrpc.SubServerStatusReq) (*litrpc.SubServerStatusResp, error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := make(map[string]*litrpc.SubServerStatus, len(s.servers))
	for name, status := range s.servers {
		resp[name] = &litrpc.SubServerStatus{
			Running: status.Running,
			Error:   status.Err,
		}
	}

	return &litrpc.SubServerStatusResp{
		SubServers: resp,
	}, nil
}

// getSubServerState queries the current status of a given sub-server.
func (s *Server) ServerStatus(name string) *Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status, ok := s.servers[name]
	if !ok {
		return &Status{
			Running:   false,
			Err:       "server not found",
			Timestamp: time.Now().UTC(),
		}
	}

	return status
}

// SetRunning can be used to set the status of a sub-server as running
// with no errors.
func (s *Server) SetRunning(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers[name] = &Status{
		Running:   true,
		Timestamp: time.Now().UTC(),
	}
}

// SetStopped can be used to set the status of a sub-server as not running
// and with no errors.
func (s *Server) SetStopped(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers[name] = &Status{
		Running:   false,
		Timestamp: time.Now().UTC(),
	}
}

// setServerErrored can be used to set the status of a sub-server as not running
// and also to set an error message for the sub-server.
func (s *Server) SetExitError(name string, errStr string,
	params ...interface{}) {

	s.mu.Lock()
	defer s.mu.Unlock()

	err := fmt.Sprintf(errStr, params...)
	log.Errorf("could not start the %s sub-server: %s", name, err)

	s.servers[name] = &Status{
		Running:   false,
		Err:       err,
		Timestamp: time.Now().UTC(),
	}
}
