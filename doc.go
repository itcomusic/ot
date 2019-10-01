package ot

import (
	"context"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/itcomusic/ot/pkg/oscript"
)

const docmanService = "DocumentManagement"

// FileAttr information about files.
type FileAttr struct {
	NodeID   int64     `oscript:"-"`
	Created  time.Time `oscript:"CreatedDate"`
	Modified time.Time `oscript:"ModifiedDate"`
	Name     string    `oscript:"Name"`
	Size     int64     `oscript:"DataForkSize"`
}

// MarshalOscriptBuf marshals to specific format, the fields for process unmarshal/marshal are identified differently.
func (f *FileAttr) MarshalOscriptBuf(buf oscript.Buffer) error {
	buf.WriteString("A<1,?,'CreatedDate'=")
	buf.WriteEncode(f.Created)
	buf.WriteString(",'FileName'=")
	buf.WriteStringValue(f.Name)
	buf.WriteString(",'FileSize'=" + strconv.FormatInt(f.Size, 10))
	buf.WriteString(",'ModifiedDate'=")
	buf.WriteEncode(f.Modified)
	buf.WriteString(",'_SDOName'='Core.FileAtts'")
	buf.WriteByte('>')

	return nil
}

// OpenFile opens the named file for reading.
func OpenFile(name string) (*os.File, *FileAttr, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}

	return f, &FileAttr{
		Name:     s.Name(),
		Size:     s.Size(),
		Created:  time.Now(),
		Modified: s.ModTime(),
	}, nil
}

// CreateFile creates a document.
func (s *Session) CreateFile(ctx context.Context, parent int64, name string, file *FileAttr, r io.Reader) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Write(docmanService, "CreateSimpleDocument", s.auth,
		oscript.M{
			"parentID": parent,
			"name":     name,
			"fileAtts": file,
		}); err != nil {
		return err
	}

	if err := c.WriteFrom(r); err != nil {
		return err
	}

	if err := errIn(c.Read(&file.NodeID)); err != nil {
		return err
	}
	return nil
}

// AddVersionFile adds new version of the file.
func (s *Session) AddVersionFile(ctx context.Context, file *FileAttr, r io.Reader) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Write(docmanService, "AddVersion", s.auth,
		oscript.M{
			"ID":       file.NodeID,
			"Metadata": nil,
			"fileAtts": file,
		}); err != nil {
		return err
	}

	if err := c.WriteFrom(r); err != nil {
		return err
	}

	if err := errIn(c.Read(nil)); err != nil {
		return err
	}
	return nil
}

// ReadFile reads content and returns information about the file.
func (s *Session) ReadFile(ctx context.Context, id, version int64, w io.Writer) (*FileAttr, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	if err := c.Write(docmanService, "GetVersionContents", s.auth,
		oscript.M{
			"ID":         id,
			"versionNum": version,
		}); err != nil {
		return nil, err
	}

	fa := &FileAttr{}
	if err := errIn(c.ReadFile(fa)); err != nil {
		return nil, err
	}

	if err := c.ReadTo(w); err != nil {
		return nil, err
	}

	fa.NodeID = id
	return fa, nil
}

type Document struct {
	Comment        string
	Name           string
	Parent         int64
	Metadata       Metadata
	VersionControl bool
	File           *FileAttr
	Reader         io.Reader
}

// CreateDocument creates document.
func (s *Session) CreateDocument(ctx context.Context, doc Document) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Write(docmanService, "CreateDocument", s.auth,
		oscript.M{
			"parentID":               doc.Parent,
			"name":                   doc.Name,
			"comment":                doc.Comment,
			"advancedVersionControl": doc.VersionControl,
			"metadata":               doc.Metadata, // inherit metadata from the parent object if metadata not set
			"fileAtts":               doc.File,
		}); err != nil {
		return err
	}

	if err := c.WriteFrom(doc.Reader); err != nil {
		return err
	}

	var node Node
	if err := errIn(c.Read(&node)); err != nil {
		return err
	}

	doc.File.NodeID = node.ID
	return nil
}
