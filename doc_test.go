package ot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/itcomusic/ot/pkg/oscript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileAttr_MarshalOscript(t *testing.T) {
	t.Parallel()

	cr := time.Date(2019, 12, 4, 11, 47, 16, 0, time.UTC)
	md := time.Date(2018, 12, 4, 11, 47, 16, 0, time.UTC)

	got, err := oscript.Marshal(&FileAttr{Created: cr, Modified: md, Name: "test", Size: 1})
	require.Nil(t, err)
	assert.Equal(t, "A<1,?,'CreatedDate'=D/2019/12/4:11:47:16,'FileName'='test','FileSize'=1,'ModifiedDate'=D/2018/12/4:11:47:16,'_SDOName'='Core.FileAtts'>", string(got))
}

func TestFileAttr_UnmarshalOscript(t *testing.T) {
	t.Parallel()

	cr := time.Date(2019, 12, 4, 11, 47, 16, 0, time.UTC)
	md := time.Date(2018, 12, 4, 11, 47, 16, 0, time.UTC)

	got := &FileAttr{}
	err := oscript.Unmarshal([]byte("A<1,?,'CreatedDate'=D/2019/12/4:11:47:16,'Name'='test','DataForkSize'=1,'ModifiedDate'=D/2018/12/4:11:47:16,'_SDOName'='Core.FileAtts'>"), got)
	require.Nil(t, err)
	assert.Equal(t, &FileAttr{Created: cr, Modified: md, Name: "test", Size: 1}, got)
}

func TestSession_ReadFile(t *testing.T) {
	t.Parallel()

	contentFile := "content of the file"
	tm := time.Date(2018, 12, 13, 15, 27, 15, 0, time.UTC)
	expAttr := &FileAttr{
		Created:  tm,
		Modified: tm,
		Name:     "test",
		Size:     int64(len(contentFile)),
		NodeID:   1,
	}

	w := bytes.Buffer{}
	gotAttr, err := session(t, func(r io.Reader, w *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "DocumentManagement",
			"ServiceMethod": "GetVersionContents",
			"Arguments": map[string]interface{}{
				"ID":         float64(1),
				"versionNum": float64(2),
			},
		}), fmt.Sprint(req))

		bytes, err := ioutil.ReadFile("testdata/read-file")
		require.Nil(t, err)

		w.Write(bytes)
		w.WriteString(contentFile)
		assert.Nil(t, w.Flush())
	}).ReadFile(context.Background(), 1, 2, &w)

	require.Nil(t, err)
	assert.Equal(t, contentFile, w.String())
	assert.Equal(t, expAttr, gotAttr)
}

func TestSession_WriteFile(t *testing.T) {
	t.Parallel()

	contentFile := "content of the file"
	tm := time.Date(2018, 12, 13, 15, 27, 15, 0, time.UTC)
	fa := &FileAttr{
		Created:  tm,
		Modified: tm,
		Name:     "file.pdf",
		Size:     int64(len(contentFile)),
	}

	r := &bytes.Buffer{}
	r.WriteString(contentFile)

	err := session(t, func(r io.Reader, w *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "DocumentManagement",
			"ServiceMethod": "CreateSimpleDocument",
			"Arguments": map[string]interface{}{
				"parentID": float64(1),
				"name":     "name",
				"fileAtts": map[string]interface{}{
					"CreatedDate":  tm,
					"ModifiedDate": tm,
					"FileName":     "file.pdf",
					"FileSize":     float64(len(contentFile)),
					"_SDOName":     "Core.FileAtts",
				},
			},
		}), fmt.Sprint(req))

		// read content
		file := make([]byte, len(contentFile))
		_, err := r.Read(file)
		require.Nil(t, err)
		assert.Equal(t, contentFile, string(file))

		w.WriteString("A<1,?,'Results'=3,'_apiError'='','_errMsg'='','_Status'=0,'_StatusMessage'=''>")
		assert.Nil(t, w.Flush())
	}).CreateFile(context.Background(), 1, "name", fa, r)

	require.Nil(t, err)
	assert.Equal(t, int64(3), fa.NodeID)
}

func TestSession_AddVersionFile(t *testing.T) {
	t.Parallel()

	contentFile := "content of the file"
	tm := time.Date(2018, 12, 13, 15, 27, 15, 0, time.UTC)
	fa := &FileAttr{
		Created:  tm,
		Modified: tm,
		Name:     "test",
		Size:     int64(len(contentFile)),
		NodeID:   1,
	}

	r := &bytes.Buffer{}
	r.WriteString(contentFile)

	err := session(t, func(r io.Reader, w *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "DocumentManagement",
			"ServiceMethod": "AddVersion",
			"Arguments": map[string]interface{}{
				"ID":       float64(1),
				"Metadata": nil,
				"fileAtts": map[string]interface{}{
					"CreatedDate":  tm,
					"ModifiedDate": tm,
					"FileName":     "test",
					"FileSize":     float64(len(contentFile)),
					"_SDOName":     "Core.FileAtts",
				},
			},
		}), fmt.Sprint(req))

		// read content
		file := make([]byte, len(contentFile))
		_, err := r.Read(file)
		require.Nil(t, err)
		assert.Equal(t, contentFile, string(file))

		bytes, err := ioutil.ReadFile("testdata/add-version-file")
		require.Nil(t, err)

		w.Write(bytes)
		assert.Nil(t, w.Flush())
	}).AddVersionFile(context.Background(), fa, r)
	require.Nil(t, err)
}
