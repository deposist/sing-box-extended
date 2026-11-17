package multiplex

import (
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"
)

// TestSession_ReceiveQueueOverflowResetsStream verifies that exceeding the
// per-stream queue limit resets that stream instead of blocking the demux loop.
func TestSession_ReceiveQueueOverflowResetsStream(t *testing.T) {
	c1, c2 := net.Pipe()

	client, err := NewClientSession(c1)
	if err != nil {
		t.Fatalf("client session: %v", err)
	}
	server, err := NewServerSession(c2)
	if err != nil {
		t.Fatalf("server session: %v", err)
	}
	defer client.Close()
	defer server.Close()

	clientStream, err := client.OpenStream([]byte("hello"))
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer clientStream.Close()
	accepted, openPayload, err := server.AcceptStream()
	if err != nil {
		t.Fatalf("accept stream: %v", err)
	}
	defer accepted.Close()
	if string(openPayload) != "hello" {
		t.Fatalf("open payload: got %q", openPayload)
	}
	serverStream := accepted.(*stream)

	const total = 12 * 1024 * 1024
	payload := make([]byte, total)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand: %v", err)
	}

	writeDone := make(chan error, 1)
	go func() {
		_, err := clientStream.Write(payload)
		writeDone <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !errors.Is(serverStream.closedErr(), errMuxReceiveQueueFull) {
		if time.Now().After(deadline) {
			t.Fatalf("stream was not reset: %v", serverStream.closedErr())
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case <-writeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("write did not finish after the stream reset")
	}
}
