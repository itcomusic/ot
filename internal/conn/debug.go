package conn

import (
	"context"
	"fmt"
	"io"
)

var (
	lenRead  = 9 // status request
	lenWrite = 8 // open request
)

// A DialDebug represents an log in stdout writing and reading bytes except protocol.
// debug-read shows count bytes which were read and debug-write shows count bytes which will be written.
type DialDebug struct {
	Dial Dialer
	Out  io.Writer
}

func (d *DialDebug) DialContext(ctx context.Context) (io.ReadWriteCloser, error) {
	c, err := d.Dial.DialContext(ctx)
	if err != nil {
		return nil, err
	}

	return &connDebug{ // TODO: create uuid
		conn:      c,
		w:         d,
		skipRead:  lenRead,
		skipWrite: lenWrite,
	}, nil
}

func (d *DialDebug) Write(b []byte) (n int, err error) {
	n, err = d.Out.Write(b)
	return n, err
}

type connDebug struct {
	skipRead  int
	skipWrite int
	conn      io.ReadWriteCloser
	w         io.Writer
}

func (cd *connDebug) Read(p []byte) (n int, err error) {
	n, err = cd.conn.Read(p)
	if n == 0 {
		return n, err
	}

	if n <= cd.skipRead {
		cd.skipRead -= n
		return n, err
	}
	// TODO: may be need sync
	io.WriteString(cd.w, fmt.Sprintf("debug-read(%d-bytes): %s\n", n-cd.skipRead, p[cd.skipRead:n]))
	if cd.skipRead != 0 {
		cd.skipRead = 0
	}
	return n, err
}

func (cd *connDebug) Write(p []byte) (n int, err error) {
	l := len(p)
	if l == 0 {
		return n, err
	}

	if l <= cd.skipWrite {
		cd.skipWrite -= l

		n, err = cd.conn.Write(p)
		return n, err
	}

	io.WriteString(cd.w, fmt.Sprintf("debug-write(%d-bytes): %s\n", l-cd.skipWrite, p[cd.skipWrite:]))
	if cd.skipWrite != 0 {
		cd.skipWrite = 0
	}

	n, err = cd.conn.Write(p)
	return n, err
}

func (cd *connDebug) Close() error {
	return cd.conn.Close()
}
