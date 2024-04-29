package fatal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecover(t *testing.T) {
	ts := time.Date(1234, 1, 2, 3, 4, 0, 0, time.UTC)
	now = func() time.Time {
		return ts
	}

	ctx, cancel := Context(context.Background())
	defer cancel(nil)

	defer func() {
		require.Equal(t, true, Failed(ctx))
		require.Equal(t, context.Canceled, ctx.Err())
		require.Equal(t, Error{Time: ts}, context.Cause(ctx))
	}()

	defer Recover(ctx)
	require.Equal(t, false, Failed(ctx))

	func() {
		panic("this is bad")
	}()

}
