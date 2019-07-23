package ot

import (
	"errors"
	"fmt"
	"testing"

	"github.com/itcomusic/ot/internal/client"

	"github.com/stretchr/testify/assert"
)

func Test_Errors(t *testing.T) {
	for i, tt := range []struct {
		in  *client.Response
		err error
	}{
		{
			in:  &client.Response{Status: 903101, StatusMessage: "DocMan.NodeRetrievalError", Desc: "error", Service: "service.method"},
			err: &NodeRetrievalError{OpError: &client.OpError{Service: "service.method", Err: errors.New("error")}},
		},
		{
			in:  &client.Response{Status: 903101, StatusMessage: "DocMan.NodeRetrievalError", Desc: "error [E662241287]", Service: "service.method"},
			err: &NodeRetrievalError{OpError: &client.OpError{Service: "service.method", Err: errors.New("error")}, isNotFound: true},
		},
	} {
		assert.Equal(t, tt.err, errIn(tt.in, nil), fmt.Sprintf("%d", i))
	}
}
