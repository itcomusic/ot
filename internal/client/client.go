package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/itcomusic/ot/pkg/oscript"
)

var (
	// errUnexpectedEOF returned by unexpected closed request.
	errUnexpectedEOF = errors.New("to check trace file in opentext")
	// errOpenRequest returned by not accepting open request.
	errOpenRequest = errors.New("protocol error")
)

// OpenRequest is a expectation bytes on first place in every request service.
var OpenRequest = []byte{1, 8, 1, 3, 0, 0, 2, 4}

// An OpError is the error type usually returned by functions in the ot package.
// It describes the service method, and text of an error.
type OpError struct {
	// service is a service with happened error
	Service string
	// Err is a message of the error
	Err error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("ot: %s %s", e.Service, e.Err)
}

type Client struct {
	conn    io.ReadWriteCloser
	dec     *oscript.Decoder
	enc     *oscript.Encoder
	encBuf  *bufio.Writer
	opened  bool
	service string
}

func New(conn io.ReadWriteCloser) *Client {
	encBuf := bufio.NewWriter(conn)

	return &Client{
		conn:   conn,
		dec:    oscript.NewDecoder(conn),
		enc:    oscript.NewEncoder(encBuf),
		encBuf: encBuf,
	}
}

func (c *Client) Write(service, method string, auth fmt.Stringer, args oscript.M) error {
	c.service = service + "." + method

	if _, err := c.encBuf.Write(OpenRequest); err != nil {
		return &OpError{Service: c.service, Err: err}
	}

	if err := c.enc.Encode(&request{
		Service: service,
		Method:  method,
		Auth:    auth,
		Args:    args,
	}); err != nil {
		return &OpError{Service: c.service, Err: err}
	}

	if err := c.encBuf.Flush(); err != nil {
		return &OpError{Service: c.service, Err: err}
	}

	return nil
}

func (c *Client) WriteFrom(r io.Reader) error {
	if _, err := io.Copy(c.conn, r); err != nil {
		return &OpError{Service: c.service, Err: err}
	}

	return nil
}

func (c *Client) readMessage(resp *Response) (*Response, error) {
	status := make([]byte, 9)
	if _, err := c.conn.Read(status); err != nil {
		return nil, &OpError{Service: c.service, Err: err}
	}

	// expecting bytes
	if int(status[1]) != 9 || int(status[7]) != 1 {
		return nil, &OpError{Service: c.service, Err: errOpenRequest}
	}

	// open-request was sent and got success
	c.opened = true
	if err := c.dec.Decode(resp); err != nil {
		if _, ok := err.(*net.OpError); ok {
			return nil, &OpError{Service: c.service, Err: errUnexpectedEOF}
		}
		return nil, &OpError{Service: c.service, Err: err}
	}

	return resp, nil
}

func (c *Client) Read(r interface{}) (*Response, error) {
	if r == nil {
		r = nilResponse
	}
	return c.readMessage(&Response{Results: r, Service: c.service})
}

func (c *Client) ReadFile(fa interface{}) (*Response, error) {
	return c.readMessage(&Response{FileAttr: fa, Service: c.service})
}

func (c *Client) Exec(service, method string, auth fmt.Stringer, args oscript.M, result interface{}) (*Response, error) {
	if err := c.Write(service, method, auth, args); err != nil {
		return nil, err
	}

	return c.Read(result)
}

func (c *Client) ReadTo(w io.Writer) error {
	// notice: io.EOF not returned by empty buffer because io.Copy checks it
	if _, err := io.Copy(w, c.dec.Buffered()); err != nil {
		return &OpError{Service: c.service, Err: err}
	}

	if _, err := io.Copy(w, c.conn); err != nil {
		return &OpError{Service: c.service, Err: err}
	}

	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
