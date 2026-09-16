package mtproxy

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	N "github.com/sagernet/sing/common/network"
)

type shutdownTestLogger struct {
	log.ContextLogger
	started chan struct{}
	once    sync.Once
}

func (l *shutdownTestLogger) Info(args ...any) {
	if len(args) == 1 && args[0] == "Stream has been started" {
		l.once.Do(func() { close(l.started) })
	}
}

// Background probes and handshake fallbacks must never leave the test process.
type shutdownTestRouter struct{ adapter.Router }

func (shutdownTestRouter) RouteConnectionEx(_ context.Context, conn net.Conn, _ adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := conn.Close()
	if onClose != nil {
		onClose(err)
	}
}

func TestInboundCloseReleasesListenerAndConnections(t *testing.T) {
	logger := &shutdownTestLogger{
		ContextLogger: log.NewNOPFactory().Logger(),
		started:       make(chan struct{}),
	}
	inbound, err := NewInbound(context.Background(), shutdownTestRouter{}, logger, "test", option.MTProxyInboundOptions{
		Users: []option.MTProxyUser{{Name: "test", Secret: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := inbound.(*Inbound)
	if err := n.Start(adapter.StartStateStart); err != nil {
		t.Fatal(err)
	}
	listener := n.listener.TCPListener()
	// Also release a leaked listener when running against the broken implementation.
	t.Cleanup(func() { _ = listener.Close() })
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	select {
	case <-logger.started:
	case <-time.After(5 * time.Second):
		t.Fatal("connection was not accepted")
	}

	closed := make(chan error, 1)
	go func() { closed <- n.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		_ = listener.Close()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown remained blocked after listener cleanup")
		}
		t.Fatal("Close blocked while the MTProxy listener was still open")
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var data [1]byte
	if _, err := conn.Read(data[:]); err == nil {
		t.Fatal("client connection remained open after Close")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("client connection was not closed before the deadline")
	}
	if next, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		_ = next.Close()
		t.Fatal("listener accepted a connection after Close")
	}
	rebound, err := net.Listen("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("listener address was not released: %v", err)
	}
	if err := rebound.Close(); err != nil {
		t.Fatal(err)
	}
}
