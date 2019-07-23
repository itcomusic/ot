package conn

import (
	"context"
	"io"
	"net"
	"time"
)

// Dialer is the interface implemented by types that can create new connection.
type Dialer interface {
	DialContext(ctx context.Context) (io.ReadWriteCloser, error)
}

type Dial struct {
	Addr string
}

func (d *Dial) DialContext(ctx context.Context) (io.ReadWriteCloser, error) {
	dialer := &net.Dialer{}
	c, err := dialer.DialContext(ctx, "tcp", d.Addr)
	if err != nil {
		return nil, err
	}

	return &conn{
		ctx:  ctx,
		conn: c,
	}, nil
}

// An conn represents an active connection.
type conn struct {
	ctx  context.Context
	conn net.Conn
}

func (c *conn) Write(p []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
	}

	deadline := time.Time{}
	if dl, ok := c.ctx.Deadline(); ok {
		deadline = dl
	}

	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return 0, err
	}

	return c.conn.Write(p)
}

func (c *conn) Read(p []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
	}

	deadline := time.Time{}
	if dl, ok := c.ctx.Deadline(); ok {
		deadline = dl
	}

	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return 0, err
	}

	return c.conn.Read(p)
}

func (c *conn) Close() error {
	return c.conn.Close()
}
