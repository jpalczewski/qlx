package transport

import (
	"context"
	"fmt"
	"time"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// ioContext derives the context for one I/O operation from the transport's
// close context: it ends when the timeout elapses or when Close() cancels
// parent, whichever comes first. A nil parent means "never cancelled by Close".
func ioContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, timeout)
}

// classifyIOError wraps err with ErrTimeout when the operation's own deadline
// expired. An operation aborted by Close() (parent cancelled, no deadline hit)
// is returned unchanged: that is a shutdown, not an unresponsive device.
func classifyIOError(op string, ioCtx context.Context, timeout time.Duration, err error) error {
	if err == nil || ioCtx.Err() != context.DeadlineExceeded {
		return err
	}
	webutil.LogTrace("%s: no answer within %v: %v", op, timeout, err)
	return fmt.Errorf("%s: %w after %v: %v", op, ErrTimeout, timeout, err)
}
