package ot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/itcomusic/ot/internal/client"

	"github.com/itcomusic/ot/internal/conn"
	"github.com/itcomusic/ot/pkg/oscript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var statusRequest = []byte{0, 9, 0, 0, 0, 0, 0, 1, 0}

type mockServer struct {
	t      *testing.T
	handle func(io.Reader, *bufio.Writer, map[string]interface{})
}

type buffer struct {
	d    *oscript.Decoder
	conn net.Conn
}

func (b *buffer) Read(p []byte) (int, error) {
	n1, err1 := b.d.Buffered().Read(p)
	if err1 != nil && err1 != io.EOF {
		return 0, err1
	}

	n2, err2 := b.conn.Read(p[n1:])
	if err2 != nil {
		return 0, err2
	}

	return n1 + n2, nil
}

func (s *mockServer) DialContext(_ context.Context) (io.ReadWriteCloser, error) {
	tcl, server := net.Pipe()

	go func(conn net.Conn) {
		defer conn.Close()

		w := bufio.NewWriter(server)
		w.Write(statusRequest)

		r := make([]byte, 8)
		if n, err := server.Read(r); err != nil || n != 8 || !bytes.Equal(r, client.OpenRequest) {
			s.t.Error("open request server failed")
			return
		}

		var req map[string]interface{}
		d := oscript.NewDecoder(conn)
		err := d.Decode(&req)
		require.Nil(s.t, err)

		s.handle(&buffer{d: d, conn: conn}, w, req)
	}(server)

	return tcl, nil
}

func endpoint(t *testing.T, f func(r io.Reader, buf *bufio.Writer, req map[string]interface{})) *Endpoint {
	return &Endpoint{dialer: &mockServer{t: t, handle: f}}
}

func session(t *testing.T, f func(r io.Reader, buf *bufio.Writer, req map[string]interface{})) *Session {
	return endpoint(t, f).User("u", "p")
}

func TestNewEndpoint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		addr string
		exp  string
	}{
		{addr: "127.0.0.1", exp: "127.0.0.1:2099"},
		{addr: "127.0.0.1:8080", exp: "127.0.0.1:8080"},
	}

	for i, v := range testCases {
		endp := NewEndpoint(v.addr)
		assert.Equal(t, v.exp, endp.dialer.(*conn.Dial).Addr, fmt.Sprintf("#%d", i))
	}
}
