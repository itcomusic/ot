package ot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/itcomusic/ot/pkg/oscript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_ErrServiceMethod(t *testing.T) {
	t.Parallel()

	endp := NewEndpoint("")
	for i, serviceMethod := range []string{".", "s.", ".m", ""} {
		assert.Equal(t, errServiceMethod, endp.User("", "").Call(context.Background(), serviceMethod, nil, nil), fmt.Sprintf("#%d", i))
	}
}

func TestSession_Call(t *testing.T) {
	t.Parallel()

	// session(t,
	var result string
	err := endpoint(t, func(r io.Reader, buf *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "service",
			"ServiceMethod": "method",
			"Arguments": map[string]interface{}{
				"name": "gopher",
				"year": float64(2009),
			},
		}), fmt.Sprint(req))

		buf.WriteString("A<1,?,'_Status'=0,'_apiError'='','_StatusMessage'='','_errMsg'='','Results'='hello'>")
		assert.Nil(t, buf.Flush())
	}).User("u", "p").Call(context.Background(), "service.method", oscript.M{"name": "gopher", "year": 2009}, &result)

	require.Nil(t, err)
	assert.Equal(t, "hello", result)
}

func TestSession_DebugCall(t *testing.T) {
	t.Parallel()

	var w bytes.Buffer
	var result string
	err := endpoint(t, func(r io.Reader, buf *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "service",
			"ServiceMethod": "method",
			"Arguments": map[string]interface{}{
				"name": "gopher",
			},
		}), fmt.Sprint(req))

		buf.WriteString("A<1,?,'_Status'=0,'_apiError'='','_StatusMessage'='','_errMsg'='','Results'='hello'>")
		assert.Nil(t, buf.Flush())
	}).User("u", "p").Debug(&w).Call(context.Background(), "service.method", oscript.M{"name": "gopher"}, &result)
	require.Nil(t, err)

	exp := "debug-write(153-bytes): A<1,N,'_ApiName'='InvokeService','ServiceName'='service','ServiceMethod'='method','_UserName'='u','_UserPassword'='p','Arguments'=A<1,?,'name'='gopher'>>\ndebug-read(84-bytes): A<1,?,'_Status'=0,'_apiError'='','_StatusMessage'='','_errMsg'='','Results'='hello'>\n"
	assert.Equal(t, exp, w.String())
	assert.Equal(t, "hello", result)
}
