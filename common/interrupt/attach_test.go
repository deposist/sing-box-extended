package interrupt

import (
	"sync/atomic"
	"testing"
)

type attachTestCloser struct{ closes atomic.Int32 }

func (c *attachTestCloser) Close() error { c.closes.Add(1); return nil }

func TestAttachedConnectionsDetachAfterInterrupt(t *testing.T) {
	group := NewGroup()
	internal, external, detached := new(attachTestCloser), new(attachTestCloser), new(attachTestCloser)
	removeInternal := group.Add(internal, false)
	removeExternal := group.Add(external, true)
	removeDetached := group.Add(detached, false)
	removeDetached()
	group.Interrupt(false)
	removeInternal()
	removeInternal()
	if internal.closes.Load() != 1 || external.closes.Load() != 0 || detached.closes.Load() != 0 {
		t.Fatal("internal interruption must retain external and detached connections")
	}
	group.Interrupt(true)
	removeExternal()
	group.Interrupt(true)
	if external.closes.Load() != 1 || detached.closes.Load() != 0 {
		t.Fatal("interrupt/detach must not close a connection twice")
	}
}
