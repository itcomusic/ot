package ot

import (
	"errors"
	"fmt"

	"github.com/itcomusic/ot/internal/client"
)

var (
	ErrTokenExpire = fmt.Errorf("ot: token expired")
)

type NodeRetrievalError struct {
	*client.OpError
	isNotFound bool
}

func (re *NodeRetrievalError) NotFound() bool {
	return re.isNotFound
}

func errIn(r *client.Response, err error) error {
	if err != nil {
		return err
	}

	// -2147482645, -2147482644, -2147482643 login failed
	// -2147482642 session expire, when using token may be?
	// 903102 not found service
	// 903101 custom error
	switch r.Status {
	case 0:
		return nil
	case -2147482642:
		return ErrTokenExpire
	case -2147482645, -2147482644, -2147482643, 903102:
		return errors.New("ot: " + r.StatusMessage)
	default:
		switch r.StatusMessage {
		case "DocMan.NodeRetrievalError":
			nfound := false

			desc, ecode := r.ErrMessage()
			switch ecode {
			case "662241287":
				nfound = true
			}

			return &NodeRetrievalError{OpError: &client.OpError{Service: r.Service, Err: errors.New(desc)}, isNotFound: nfound}
		}

		return errors.New("ot: " + r.Desc)
	}
}
