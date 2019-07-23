package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseMessage_ErrMessage(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		in    *Response
		desc  string
		ecode string
	}{
		{
			in:    &Response{Desc: "error [E1234]."},
			desc:  "error",
			ecode: "1234",
		},
		{
			in:    &Response{Desc: "error [E]"},
			desc:  "error",
			ecode: "",
		},
		{
			in:    &Response{Desc: "error"},
			desc:  "error",
			ecode: "",
		},
		{
			in:    &Response{Desc: ""},
			desc:  "",
			ecode: "",
		},
		{
			in:    &Response{Desc: "[E"},
			desc:  "[E",
			ecode: "",
		},
	} {
		desc, ecode := tt.in.ErrMessage()
		assert.Equal(t, tt.desc, desc)
		assert.Equal(t, tt.ecode, ecode)
	}
}
