package ot

import (
	"context"
	"time"

	"github.com/itcomusic/ot/pkg/oscript"
)

type Metadata struct {
	Categories []Category `oscript:"AttributeGroups"`

	sdoName oscript.SDOName `oscript:"DocMan.Metadata,public"`
}

// Find finds category by display name.
func (m Metadata) Find(name string) *Category {
	for i, c := range m.Categories {
		if c.DisplayName == name {
			return &m.Categories[i]
		}
	}

	return nil
}

type Feature struct {
	Name string `oscript:"Name"`
	Type string `oscript:"Type"`

	BooleanValue *bool      `oscript:"BooleanValue,omitempty"`
	DateValue    *time.Time `oscript:"DateValue,omitempty"`
	IntegerValue *int       `oscript:"IntegerValue,omitempty"`
	LongValue    *float64   `oscript:"LongValue,omitempty"`
	StringValue  *string    `oscript:"StringValue,omitempty"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeFeature,public"`
}

type NodeReservationInfo struct {
	Reserved     bool       `oscript:"Reserved"`
	ReservedBy   int64      `oscript:"ReservedBy"`
	ReservedDate *time.Time `oscript:"ReservedDate,omitempty"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeReservationInfo,public"`
}

type NodeVersionInfo struct {
	AdvancedVersionControl         bool      `oscript:"AdvancedVersionControl"`
	FileDataSize                   int64     `oscript:"FileDataSize"`
	FileResSize                    int64     `oscript:"FileResSize"`
	Major                          int64     `oscript:"Major"`
	MimeType                       string    `oscript:"MimeType"`
	SupportsAdvancedVersionControl bool      `oscript:"SupportsAdvancedVersionControl"`
	VersionNum                     int64     `oscript:"VersionNum"`
	Versions                       []Version `oscript:"Versions"`
	VersionsToKeep                 int       `oscript:"VersionsToKeep"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeVersionInfo,public"`
}

type Version struct {
	Comment        string    `oscript:"Comment"`
	CreateDate     time.Time `oscript:"CreateDate"`
	FileCreateDate time.Time `oscript:"FileCreateDate"`
	FileCreator    string    `oscript:"FileCreator"`
	FileDataSize   int64     `oscript:"FileDataSize"`
	FileModifyDate time.Time `oscript:"FileModifyDate"`
	FileName       string    `oscript:"Filename"`
	FilePlatform   int       `oscript:"FilePlatform"`
	FileResSize    int64     `oscript:"FileResSize"`
	FileType       string    `oscript:"FileType"`
	ID             int64     `oscript:"ID"`
	Locked         int       `oscript:"Locked"`
	LockedBy       int64     `oscript:"LockedBy,omitempty"`
	LockedDate     time.Time `oscript:"LockedDate"`
	Metadata       Metadata  `oscript:"Metadata"`
	MimeType       string    `oscript:"MimeType"`
	ModifyDate     time.Time `oscript:"ModifyDate"`
	Name           string    `oscript:"Name"`
	NodeID         int64     `oscript:"NodeID"`
	Number         int64     `oscript:"Number"`
	Owner          int64     `oscript:"Owner"`
	ProviderID     int64     `oscript:"ProviderID"`
	ProviderName   string    `oscript:"ProviderName"`
	Type           string    `oscript:"Type"`
	VerMajor       int64     `oscript:"VerMajor"`
	VerMinor       int64     `oscript:"VerMinor"`

	sdoName oscript.SDOName `oscript:"DocMan.Version,public"`
}

type NodeReferenceInfo struct {
	OriginalID   int64  `oscript:"OriginalID"`
	OriginalType string `oscript:"OriginalType"`
	VersionNum   int64  `oscript:"VersionNum,omitempty"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeReferenceInfo,public"`
}

type NodeContainerInfo struct {
	ChildCount int      `oscript:"ChildCount"`
	ChildTypes []string `oscript:"ChildTypes"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeContainerInfo,public"`
}

type Node struct {
	Catalog         int32               `oscript:"Catalog,omitempty"`
	Comment         string              `oscript:"Comment"`
	ContainerInfo   NodeContainerInfo   `oscript:"ContainerInfo"`
	CreateDate      time.Time           `oscript:"CreateDate,omitempty"`
	CreatedBy       int32               `oscript:"CreatedBy,omitempty"`
	DisplayType     string              `oscript:"DisplayType"`
	Feature         []Feature           `oscript:"Features"`
	ID              int64               `oscript:"ID"`
	IsContainer     bool                `oscript:"IsContainer"`
	IsReference     bool                `oscript:"IsReference"`
	IsReservable    bool                `oscript:"IsReservable"`
	IsVersional     bool                `oscript:"IsVersionable"`
	Metadata        Metadata            `oscript:"Metadata"`
	ModifyDate      time.Time           `oscript:"ModifyDate,omitempty"`
	Name            string              `oscript:"Name"`
	Nickname        string              `oscript:"Nickname,omitempty"`
	Parent          int64               `oscript:"ParentID"`
	PartialData     bool                `oscript:"PartialData"`
	Permissions     Permissions         `oscript:"Permissions"`
	Position        int64               `oscript:"Position,omitempty"`
	ReferenceInfo   NodeReferenceInfo   `oscript:"ReferenceInfo"`
	Released        bool                `oscript:"Released"`
	ReservationInfo NodeReservationInfo `oscript:"ReservationInfo"`
	Type            string              `oscript:"Type"`
	VersionInfo     NodeVersionInfo     `oscript:"VersionInfo"`
	VolumeID        int64               `oscript:"VolumeID"`

	sdoName oscript.SDOName `oscript:"DocMan.Node,public"`
}

// CreateNode creates node.
func (s *Session) CreateNode(ctx context.Context, node *Node) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "CreateNode", s.auth, oscript.M{"node": node}, node)); err != nil {
		return err
	}
	return nil
}

// GetNode gets node
func (s *Session) GetNode(ctx context.Context, id int64) (*Node, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var node Node
	if err := errIn(c.Exec(docmanService, "GetNode", s.auth, oscript.M{"ID": id}, &node)); err != nil {
		return nil, err
	}
	return &node, nil
}

// GetNodeByNickname gets node by nickname.
func (s *Session) GetNodeByNickname(ctx context.Context, nickname string) (*Node, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var node Node
	if err := errIn(c.Exec(docmanService, "GetNodeByNickname", s.auth, oscript.M{"nickname": nickname}, &node)); err != nil {
		return nil, err
	}
	return &node, nil
}

// GetCategory gets category.
func (s *Session) GetCategory(ctx context.Context, id int64) (*Category, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var cat Category
	if err := errIn(c.Exec(docmanService, "GetCategoryTemplate", s.auth, oscript.M{"categoryID": id}, &cat)); err != nil {
		return nil, err
	}
	return &cat, nil
}

// UpdateNode updates node. Checks on update Catalog, Comment, Name, Position. Always updates the fields Metadata, Nickname.
func (s *Session) UpdateNode(ctx context.Context, node *Node) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "UpdateNode", s.auth, oscript.M{"node": node}, nil)); err != nil {
		return err
	}
	return nil
}

// DeleteNode deletes node.
func (s *Session) DeleteNode(ctx context.Context, id int64) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "DeleteNode", s.auth, oscript.M{"ID": id}, nil)); err != nil {
		return err
	}
	return nil
}

// RenameNode renames node.
func (s *Session) RenameNode(ctx context.Context, id int64, name string) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "RenameNode", s.auth, oscript.M{"ID": id, "newName": name}, nil)); err != nil {
		return err
	}
	return nil
}

// UpdateVersion updates version. Checks on update Comment, MimeType.
func (s *Session) UpdateVersion(ctx context.Context, v Version) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "UpdateVersion", s.auth, oscript.M{"version": v}, nil)); err != nil {
		return err
	}
	return nil
}

// ReserveNode reserves node.
func (s *Session) ReserveNode(ctx context.Context, id int64, user int64) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "ReserveNode", s.auth, oscript.M{"ID": id, "userID": user}, nil)); err != nil {
		return err
	}
	return nil
}

// UnreserveNode unreserves node.
func (s *Session) UnreserveNode(ctx context.Context, id int64) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := errIn(c.Exec(docmanService, "UnreserveNode", s.auth, oscript.M{"ID": id}, nil)); err != nil {
		return err
	}
	return nil
}

// CreateFolder creates folder.
func (s *Session) CreateFolder(ctx context.Context, parentID int64, name, comment string, metadata Metadata) (*Node, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var node Node
	if err := errIn(c.Exec(docmanService, "CreateFolder", s.auth, oscript.M{"parentID": parentID, "name": name, "comment": comment, "metadata": metadata}, &node)); err != nil {
		return nil, err
	}
	return &node, nil
}
