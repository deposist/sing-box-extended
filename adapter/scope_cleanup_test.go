package adapter

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"

	E "github.com/sagernet/sing/common/exceptions"
)

func TestScopeCleanupKeepsReverseOrderAndRealErrors(t *testing.T) {
	scope := NewScope(context.Background(), nil)
	var order []int
	realFailure := errors.New("cleanup failed")
	scope.Add(func() error { order = append(order, 1); return E.Errors(net.ErrClosed, realFailure) })
	scope.Add(func() error { order = append(order, 2); return context.Canceled })
	scope.Add(func() error { order = append(order, 3); return nil })
	err := scope.Close()
	if !slices.Equal(order, []int{3, 2, 1}) {
		t.Fatalf("cleanup order = %v", order)
	}
	if !errors.Is(err, realFailure) || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		t.Fatalf("real cleanup error must survive closed/canceled filtering: %v", err)
	}
}
