package managedserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedprotocol"
)

type Config struct {
	Path         string
	Mode         os.FileMode
	Connections  int
	FrameBytes   int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (configuration Config) Validate() error {
	if !filepath.IsAbs(configuration.Path) || configuration.Connections < 1 || configuration.Connections > 1024 ||
		configuration.FrameBytes < 1 || configuration.ReadTimeout <= 0 || configuration.WriteTimeout <= 0 ||
		configuration.Mode&0o600 != 0o600 || configuration.Mode&0o117 != 0 || configuration.Mode > 0o660 {
		return errors.New("invalid managed Unix server configuration")
	}
	return nil
}

type Server struct {
	configuration Config
	submitter     managedprotocol.Submitter
	listener      *net.UnixListener
	identity      os.FileInfo
	connections   chan struct{}
	admitting     atomic.Bool
	closed        atomic.Bool
	wait          sync.WaitGroup
	closeOnce     sync.Once
}

func Listen(configuration Config, submitter managedprotocol.Submitter) (*Server, error) {
	if err := configuration.Validate(); err != nil || submitter == nil {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("managed submitter is required")
	}
	if err := preparePath(configuration.Path); err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: configuration.Path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = listener.Close(); _ = os.Remove(configuration.Path) }
	if err := os.Chmod(configuration.Path, configuration.Mode); err != nil {
		cleanup()
		return nil, err
	}
	identity, err := os.Lstat(configuration.Path)
	if err != nil {
		cleanup()
		return nil, err
	}
	server := &Server{
		configuration: configuration, submitter: submitter, listener: listener, identity: identity,
		connections: make(chan struct{}, configuration.Connections),
	}
	server.admitting.Store(true)
	return server, nil
}

func (server *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	for {
		connection, err := server.listener.AcceptUnix()
		if err != nil {
			if server.closed.Load() || ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		select {
		case server.connections <- struct{}{}:
			server.wait.Add(1)
			go server.serveConnection(ctx, connection)
		default:
			_ = writeResponse(connection, server.configuration.WriteTimeout, managedprotocol.Response{Version: managedprotocol.Version, Outcome: managed.OutcomeOverloaded})
			_ = connection.Close()
		}
	}
}

func (server *Server) CloseAdmission() { server.admitting.Store(false) }

func (server *Server) Close() error {
	var closeErr error
	server.closeOnce.Do(func() {
		server.closed.Store(true)
		server.admitting.Store(false)
		closeErr = server.listener.Close()
		server.wait.Wait()
		if current, err := os.Lstat(server.configuration.Path); err == nil && os.SameFile(server.identity, current) {
			if err := os.Remove(server.configuration.Path); err != nil {
				closeErr = errors.Join(closeErr, err)
			}
		}
	})
	return closeErr
}

func (server *Server) serveConnection(ctx context.Context, connection *net.UnixConn) {
	defer func() {
		_ = connection.Close()
		<-server.connections
		server.wait.Done()
	}()
	if !server.admitting.Load() {
		_ = writeResponse(connection, server.configuration.WriteTimeout, managedprotocol.Response{Version: managedprotocol.Version, Outcome: managed.OutcomeClosed})
		return
	}
	if err := connection.SetReadDeadline(time.Now().Add(server.configuration.ReadTimeout)); err != nil {
		return
	}
	frame, err := managedprotocol.ReadFrame(connection, server.configuration.FrameBytes)
	if err != nil {
		if code, ok := managedprotocol.ErrorCode(err); ok {
			_ = writeResponse(connection, server.configuration.WriteTimeout, managedprotocol.Response{Version: managedprotocol.Version, Outcome: "protocol", Reason: string(code)})
		}
		return
	}
	if !server.admitting.Load() {
		_ = writeResponse(connection, server.configuration.WriteTimeout, managedprotocol.Response{Version: managedprotocol.Version, Outcome: managed.OutcomeClosed})
		return
	}
	response := managedprotocol.Handle(ctx, server.submitter, frame, server.configuration.FrameBytes)
	_ = writeResponse(connection, server.configuration.WriteTimeout, response)
}

func writeResponse(connection net.Conn, timeout time.Duration, response managedprotocol.Response) error {
	content, err := managedprotocol.EncodeResponse(response)
	if err != nil {
		return err
	}
	if err := connection.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	_, err = io.Copy(connection, bytes.NewReader(content))
	return err
}

func preparePath(path string) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return errors.New("managed socket parent must be a private directory")
	}
	if _, err := os.Lstat(path); err == nil {
		return errors.New("managed socket path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
