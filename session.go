package ot

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/itcomusic/ot/internal/client"

	"github.com/itcomusic/ot/internal/conn"
	"github.com/itcomusic/ot/pkg/oscript"
)

var (
	typeDialDebug = reflect.TypeOf(&conn.DialDebug{})
)

// Session a information about authentication user.
type Session struct {
	ep   *Endpoint
	auth fmt.Stringer
}

func (s *Session) clone() *Session {
	return &Session{
		ep: &Endpoint{
			dialer: s.ep.dialer,
		},
		auth: s.auth,
	}
}

// Debug wraps dialer in debug.
func (s *Session) Debug(w io.Writer) *Session {
	if reflect.TypeOf(s.ep.dialer) == typeDialDebug {
		return s //  ¯\_(ツ)_/¯
	}

	c := s.clone()
	c.ep.dialer = &conn.DialDebug{Dial: s.ep.dialer, Out: w}
	return c
}

func (s *Session) connect(ctx context.Context) (*client.Client, error) {
	c, err := s.ep.dialer.DialContext(ctx)
	if err != nil {
		return nil, err
	}

	return client.New(c), nil
}

// Call invokes the service function, waits for it to complete, and returns its error status.
func (s *Session) Call(ctx context.Context, serviceMethod string, args oscript.M, reply interface{}) error {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot <= 0 || dot == len(serviceMethod)-1 {
		return errServiceMethod
	}

	if args == nil {
		args = oscript.M{}
	}

	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(serviceMethod[:dot], serviceMethod[dot+1:], s.auth, args, reply)); err != nil {
		return err
	}
	return nil
}
