package vsock

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/mdlayher/vsock"
	log "github.com/sirupsen/logrus"
)

type Proxy struct {
	cid    uint32
	port   uint32
	socket string
	done   chan struct{}
	start  func(socket string, port, cid uint32, done chan struct{}) error
}

func NewProxyUnixSocketToVsock(port, cid uint32, socket string) *Proxy {
	p := &Proxy{
		cid:    cid,
		port:   port,
		socket: socket,
		done:   make(chan struct{}),
		start:  startUnixToVsock,
	}
	return p
}

func NewProxyVSockToUnixSocket(port uint32, socket string) *Proxy {
	p := &Proxy{
		port:   port,
		socket: socket,
		done:   make(chan struct{}),
		start:  startVsockToUnix,
	}
	return p
}

func (proxy *Proxy) GetSocket() string {
	return proxy.socket
}

func (proxy *Proxy) Stop() {
	select {
	case <-proxy.done:
		// already closed
	default:
		close(proxy.done)
	}
	os.Remove(proxy.socket)
	log.Debugf("Stopped proxy")
}

func (p *Proxy) Start() error {
	return p.start(p.socket, p.port, p.cid, p.done)
}

func startUnixToVsock(socket string, port, cid uint32, done chan struct{}) error {
	_ = os.Remove(socket)

	unixListener, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("Failed to listen on unix socket: %v", err)
	}
	go func() {
		defer unixListener.Close()

		for {
			select {
			case <-done:
				return
			default:
				unixConn, err := unixListener.Accept()
				if err != nil {
					log.Warnf("Accept error: %v", err)
					continue
				}
				log.Debugf("Accepted connection from %s to port %d and cid", socket, port, cid)

				go handleConnectionToVsock(unixConn, port, cid, done)
			}
		}
	}()

	log.Debugf("Started proxy at: %s", socket)

	return nil
}

func handleConnectionToVsock(unixConn net.Conn, port, cid uint32, done chan struct{}) {
	defer unixConn.Close()
	vsockConn, err := vsock.Dial(cid, port, nil)
	if err != nil {
		log.Printf("vsock connect error (cid: %d, port: %d): %v", cid, port, err)
		return
	}
	defer vsockConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 2)
	go proxy(ctx, vsockConn, unixConn, errCh, done)
	go proxy(ctx, unixConn, vsockConn, errCh, done)

	// Wait for the first error or cancellation
	select {
	case <-done:
	case err := <-errCh:
		if err != nil && err != io.EOF {
			log.Errorf("proxy error: %v", err)
		}
	}
}

func proxy(ctx context.Context, src, dst net.Conn, errCh chan error, done chan struct{}) {
	go func() {
		_, err := io.Copy(dst, src)
		errCh <- err
	}()
	select {
	case <-ctx.Done():
	case <-done:
	case <-errCh:
	}
}

func startVsockToUnix(socket string, port, cid uint32, done chan struct{}) error {
	vsockListener, err := vsock.Listen(port, &vsock.Config{})
	if err != nil {
		return fmt.Errorf("failed to listen on vsock port %d: %v", port, err)
	}
	go func() {
		defer vsockListener.Close()

		for {
			select {
			case <-done:
				return
			default:
				vsockConn, err := vsockListener.Accept()
				if err != nil {
					log.Warnf("Accept error: %v", err)
					continue
				}
				log.Debugf("Accepted connection from port %d to socket %d", port, socket)

				go handleConnectionToUnix(vsockConn, socket, port, done)
			}
		}
	}()

	log.Debugf("Started proxy at port: %d", port)

	return nil
}

func handleConnectionToUnix(vsockConn net.Conn, socket string, port uint32, done chan struct{}) {
	defer vsockConn.Close()

	conn, err := net.Dial("unix", socket)
	if err != nil {
		log.Errorf("failed to connect: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 2)
	go proxy(ctx, conn, vsockConn, errCh, done)
	go proxy(ctx, vsockConn, conn, errCh, done)

	// Wait for the first error or cancellation
	select {
	case <-done:
	case err := <-errCh:
		if err != nil && err != io.EOF {
			log.Errorf("proxy error: %v", err)
		}
	}
}
