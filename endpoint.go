package ot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/itcomusic/ot/internal/conn"
)

var (
	// errServiceMethod returned by invalid service method.
	errServiceMethod = errors.New("ot: service/method requester ill-formed")
)

var (
	defaultPort = ":2099"
)

// Endpoint represents connection address.
type Endpoint struct {
	dialer conn.Dialer
}

// NewEndpoint creates information about connection to the server opentext. No creates connection to server.
func NewEndpoint(addr string) *Endpoint {
	if !strings.Contains(addr, ":") {
		addr += defaultPort
	}

	return &Endpoint{dialer: &conn.Dial{Addr: addr}}
}

// User creates new session with auth authentication.
func (e *Endpoint) User(username, password string) *Session {
	return &Session{
		ep:   e,
		auth: &auth{enc: fmt.Sprintf("'_UserName'='%s','_UserPassword'='%s'", username, password)},
	}
}

// Token creates new session with token authentication.
func (e *Endpoint) Token(token string) *Session {
	return &Session{
		ep:   e,
		auth: &auth{enc: fmt.Sprintf("'_Cookie'='%s'", token)},
	}
}

// dial using for tests.
func (e Endpoint) dial(d conn.Dialer) *Endpoint {
	e.dialer = d
	return &e
}

type auth struct {
	enc string
}

func (u *auth) String() string {
	return u.enc
}
