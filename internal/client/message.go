package client

import (
	"fmt"
	"strings"

	"github.com/itcomusic/ot/pkg/oscript"
)

var nilResponse struct{}

// request is request about called service and method with arguments.
type request struct {
	Service string
	Method  string
	Auth    fmt.Stringer
	Args    oscript.M
}

func (r *request) MarshalOscriptBuf(buf oscript.Buffer) error {
	buf.WriteString("A<1,N,'_ApiName'='InvokeService','ServiceName'=")
	buf.WriteStringValue(r.Service)
	buf.WriteString(",'ServiceMethod'=")
	buf.WriteStringValue(r.Method)
	buf.WriteByte(',')
	buf.WriteString(r.Auth.String())
	buf.WriteString(",'Arguments'=")
	buf.WriteEncode(r.Args)
	buf.WriteByte('>')

	return nil
}

// Response is response on any request.
type Response struct {
	Status        int         `oscript:"_Status"`
	API           string      `oscript:"_apiError"`
	StatusMessage string      `oscript:"_StatusMessage"`
	Desc          string      `oscript:"_errMsg"`
	Results       interface{} `oscript:"Results"`
	FileAttr      interface{} `oscript:"FileAttributes"`
	Service       string      `oscript:"-"`
}

func (r *Response) ErrMessage() (desc string, ecode string) {
	st := strings.LastIndex(r.Desc, "[E")
	if st == -1 {
		return r.Desc, ""
	}

	st += 2
	en := strings.IndexByte(r.Desc[st:], ']')
	if en == -1 {
		return r.Desc, ""
	}

	en += st
	ecode = r.Desc[st:en]
	if st-2 != 0 && r.Desc[st-3] == ' ' {
		return r.Desc[:st-3], ecode
	}

	return r.Desc[:st-2], ecode
}
