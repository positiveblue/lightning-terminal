package subservers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lightninglabs/lightning-terminal/perms"
	"github.com/lightninglabs/lightning-terminal/status"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	grpcProxy "github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

var (
	// maxMsgRecvSize is the largest message our REST proxy will receive. We
	// set this to 200MiB atm.
	maxMsgRecvSize = grpc.MaxCallRecvMsgSize(1 * 1024 * 1024 * 200)

	// defaultConnectTimeout is the default timeout for connecting to the
	// backend.
	defaultConnectTimeout = 15 * time.Second
)

// subServer is a wrapper around the SubServer interface and is used by the
// subServerMgr to manage a SubServer.
type subServer struct {
	integratedStarted bool
	startedMu         sync.RWMutex

	stopped sync.Once

	SubServer

	remoteConn *grpc.ClientConn

	wg   sync.WaitGroup
	quit chan struct{}
}

// started returns true if the subServer has been started. This only applies if
// the subServer is running in integrated mode.
func (s *subServer) started() bool {
	s.startedMu.RLock()
	defer s.startedMu.RUnlock()

	return s.integratedStarted
}

// setStarted sets the subServer as started or not. This only applies if the
// subServer is running in integrated mode.
func (s *subServer) setStarted(started bool) {
	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	s.integratedStarted = started
}

// stop the subServer by closing the connection to it if it is remote or by
// stopping the integrated process.
func (s *subServer) stop() error {
	// If the sub-server has not yet started, then we can exit early.
	if !s.started() {
		return nil
	}

	var returnErr error
	s.stopped.Do(func() {
		close(s.quit)
		s.wg.Wait()

		// If running in remote mode, close the connection.
		if s.Remote() && s.remoteConn != nil {
			err := s.remoteConn.Close()
			if err != nil {
				returnErr = fmt.Errorf("could not close "+
					"remote connection: %v", err)
			}
			return
		}

		// Else, stop the integrated sub-server process.
		err := s.Stop()
		if err != nil {
			returnErr = fmt.Errorf("could not close "+
				"integrated connection: %v", err)
			return
		}

		if s.ServerErrChan() == nil {
			return
		}

		select {
		case returnErr = <-s.ServerErrChan():
		default:
		}
	})

	return returnErr
}

// startIntegrated starts the subServer in integrated mode.
func (s *subServer) startIntegrated(lndClient lnrpc.LightningClient,
	lndGrpc *lndclient.GrpcLndServices, withMacaroonService bool,
	onError func(error)) error {

	err := s.Start(lndClient, lndGrpc, withMacaroonService)
	if err != nil {
		return err
	}
	s.setStarted(true)

	if s.ServerErrChan() == nil {
		return nil
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		select {
		case err := <-s.ServerErrChan():
			// The sub server should shut itself down if an error
			// happens. We don't need to try to stop it again.
			s.setStarted(false)

			onError(fmt.Errorf("received "+
				"critical error from sub-server, "+
				"shutting down: %v", err),
			)

		case <-s.quit:
		}
	}()

	return nil
}

// connectRemote attempts to make a connection to the remote sub-server.
func (s *subServer) connectRemote() error {
	certPath := lncfg.CleanAndExpandPath(s.RemoteConfig().TLSCertPath)
	conn, err := dialBackend(s.Name(), s.RemoteConfig().RPCServer, certPath)
	if err != nil {
		return fmt.Errorf("remote dial error: %v", err)
	}

	s.remoteConn = conn

	return nil
}

// Manager manages a set of subServer objects.
type Manager struct {
	servers []*subServer
	mu      sync.RWMutex

	statusServer *status.Server
	permsMgr     *perms.Manager
}

// NewManager constructs a new subServerMgr.
func NewManager(permsMgr *perms.Manager,
	statusServer *status.Server) *Manager {

	return &Manager{
		servers:      []*subServer{},
		statusServer: statusServer,
		permsMgr:     permsMgr,
	}
}

// AddServer adds a new subServer to the manager's set.
func (s *Manager) AddServer(ss SubServer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers = append(s.servers, &subServer{
		SubServer: ss,
		quit:      make(chan struct{}),
	})

	s.statusServer.RegisterServer(ss.Name())
}

// StartIntegratedServers starts all the manager's sub-servers that should be
// started in integrated mode.
func (s *Manager) StartIntegratedServers(lndClient lnrpc.LightningClient,
	lndGrpc *lndclient.GrpcLndServices, withMacaroonService bool) {

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ss := range s.servers {
		if ss.Remote() {
			continue
		}

		err := ss.startIntegrated(
			lndClient, lndGrpc, withMacaroonService,
			func(err error) {
				s.statusServer.SetExitError(
					ss.Name(), err.Error(),
				)
			},
		)
		if err != nil {
			s.statusServer.SetExitError(ss.Name(), err.Error())
			continue
		}

		s.statusServer.SetRunning(ss.Name())
	}
}

// ConnectRemoteSubServers creates connections to all the manager's sub-servers
// that are running remotely.
func (s *Manager) ConnectRemoteSubServers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ss := range s.servers {
		if !ss.Remote() {
			continue
		}

		err := ss.connectRemote()
		if err != nil {
			s.statusServer.SetExitError(ss.Name(), err.Error())
			continue
		}

		s.statusServer.SetRunning(ss.Name())
	}
}

// RegisterRPCServices registers all the manager's sub-servers with the given
// grpc registrar.
func (s *Manager) RegisterRPCServices(server grpc.ServiceRegistrar) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		// In remote mode the "director" of the RPC proxy will act as
		// a catch-all for any gRPC request that isn't known because we
		// didn't register any server for it. The director will then
		// forward the request to the remote service.
		if ss.Remote() {
			continue
		}

		ss.RegisterGrpcService(server)
	}
}

// GetRemoteConn checks if any of the manager's sub-servers owns the given uri
// and if so, the remote connection to that sub-server is returned. The bool
// return value indicates if the uri is managed by one of the sub-servers
// running in remote mode.
func (s *Manager) GetRemoteConn(uri string) (bool, *grpc.ClientConn) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !s.permsMgr.IsSubServerURI(ss.PermsSubServer(), uri) {
			continue
		}

		if !ss.Remote() {
			return false, nil
		}

		return true, ss.remoteConn
	}

	return false, nil
}

// ValidateMacaroon checks if any of the manager's sub-servers owns the given
// uri and if so, if it is running in remote mode, then true is returned since
// the macaroon will be validated by the remote subserver itself when the
// request arrives. Otherwise, the integrated sub-server's validator validates
// the macaroon.
func (s *Manager) ValidateMacaroon(ctx context.Context,
	requiredPermissions []bakery.Op, uri string) (bool, error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !s.permsMgr.IsSubServerURI(ss.PermsSubServer(), uri) {
			continue
		}

		if ss.Remote() {
			return true, nil
		}

		if !ss.started() {
			return true, fmt.Errorf("%s is not yet ready for "+
				"requests, lnd possibly still starting or "+
				"syncing", ss.Name())
		}

		err := ss.ValidateMacaroon(ctx, requiredPermissions, uri)
		if err != nil {
			return true, fmt.Errorf("invalid macaroon: %v", err)
		}
	}

	return false, nil
}

// HandledBy returns true if one of its sub-servers owns the given URI.
func (s *Manager) HandledBy(uri string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !s.permsMgr.IsSubServerURI(ss.PermsSubServer(), uri) {
			continue
		}

		return true, ss.Name()
	}

	return false, ""
}

// MacaroonPath checks if any of the manager's sub-servers owns the given uri
// and if so, the appropriate macaroon path is returned for that sub-server.
func (s *Manager) MacaroonPath(uri string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !s.permsMgr.IsSubServerURI(ss.PermsSubServer(), uri) {
			continue
		}

		if ss.Remote() {
			return true, ss.RemoteConfig().MacaroonPath
		}

		return true, ss.MacPath()
	}

	return false, ""
}

// ReadRemoteMacaroon checks if any of the manager's sub-servers running in
// remote mode owns the given uri and if so, the appropriate macaroon path is
// returned for that sub-server.
func (s *Manager) ReadRemoteMacaroon(uri string) (macaroonPath string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !s.permsMgr.IsSubServerURI(ss.PermsSubServer(), uri) {
			continue
		}

		if !ss.Remote() {
			// return false, nil, nil
			return ""
		}

		return ss.RemoteConfig().MacaroonPath

	}

	return ""
}

// Stop stops all the manager's sub-servers
func (s *Manager) Stop() error {
	var returnErr error

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if ss.Remote() {
			continue
		}

		err := ss.stop()
		if err != nil {
			log.Errorf("Error stopping %s: %v", ss.Name(), err)
			returnErr = err
		}

		s.statusServer.SetStopped(ss.Name())
	}

	return returnErr
}

// dialBackend connects to a gRPC backend through the given address and uses the
// given TLS certificate to authenticate the connection.
func dialBackend(name, dialAddr, tlsCertPath string) (*grpc.ClientConn, error) {
	tlsConfig, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("could not read %s TLS cert %s: %v",
			name, tlsCertPath, err)
	}

	opts := []grpc.DialOption{
		// From the grpcProxy doc: This codec is *crucial* to the
		// functioning of the proxy.
		grpc.WithCodec(grpcProxy.Codec()), // nolint
		grpc.WithTransportCredentials(tlsConfig),
		grpc.WithDefaultCallOptions(maxMsgRecvSize),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: defaultConnectTimeout,
		}),
	}

	log.Infof("Dialing %s gRPC server at %s", name, dialAddr)
	cc, err := grpc.Dial(dialAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed dialing %s backend: %v", name,
			err)
	}
	return cc, nil
}
