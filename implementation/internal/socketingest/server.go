package socketingest

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
)

type Server struct {
	protocol    *Protocol
	listener    net.Listener
	connections chan struct{}
	mu          sync.Mutex
	active      int
}

func Listen(path string, configuration Config, publisher interface {
	Publish(context.Context, ingestion.Transport, []byte) ingestion.Result
}, observer Observer) (*Server, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("socket path must be absolute")
	}
	protocolHandler, err := NewProtocol(configuration, publisher, observer)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o660); err != nil {
		listener.Close()
		return nil, err
	}
	return &Server{protocol: protocolHandler, listener: listener, connections: make(chan struct{}, configuration.Connections)}, nil
}

func (server *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		server.listener.Close()
	}()
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case server.connections <- struct{}{}:
			server.changed(1)
			go func() {
				defer func() {
					connection.Close()
					<-server.connections
					server.changed(-1)
				}()
				_ = server.protocol.ServeConnection(ctx, connection)
			}()
		default:
			connection.Close()
		}
	}
}

func (server *Server) Close() error { return server.listener.Close() }

func (server *Server) changed(delta int) {
	server.mu.Lock()
	server.active += delta
	active := server.active
	server.mu.Unlock()
	server.protocol.observer.ConnectionChanged(active)
}
