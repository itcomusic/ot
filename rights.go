package ot

import (
	"context"

	"github.com/itcomusic/ot/pkg/oscript"
)

type Permissions struct {
	See        bool `oscript:"SeePermission"`
	SeeContent bool `oscript:"SeeContentsPermission"`
	Modify     bool `oscript:"ModifyPermission"`
	EditAttr   bool `oscript:"EditAttributesPermission"`
	EditPerm   bool `oscript:"EditPermissionsPermission"`
	DeleteVer  bool `oscript:"DeleteVersionsPermission"`
	Delete     bool `oscript:"DeletePermission"`
	Reserve    bool `oscript:"ReservePermission"`
	Create     bool `oscript:"AddItemsPermission"`

	sdoName oscript.SDOName `oscript:"DocMan.NodePermissions,public"`
}

type NodeRight struct {
	ID   int64       `oscript:"RightID"`
	Type string      `oscript:"Type"` // "ACL", "OwnerGroup", "Owner", "Public" - may be set enum?
	Perm Permissions `oscript:"Permissions"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeRight,public"`
}

type NodeRights struct {
	ACLRights       []NodeRight `oscript:"ACLRights"`
	OwnerGroupRight NodeRight   `oscript:"OwnerGroupRight"`
	OwnerRight      NodeRight   `oscript:"OwnerRight"`
	PublicRight     NodeRight   `oscript:"PublicRight"`

	sdoName oscript.SDOName `oscript:"DocMan.NodeRights,public"`
}

func (s *Session) AddNodeRight(ctx context.Context, id int64, right NodeRight) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Write(docmanService, "AddNodeRight", s.auth,
		oscript.M{
			"ID":        id,
			"nodeRight": right,
		}); err != nil {
		return err
	}

	if err := errIn(c.Read(nil)); err != nil {
		return err
	}
	return nil
}

func (s *Session) GetNodeRights(ctx context.Context, id int64) (*NodeRights, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	if err := c.Write(docmanService, "GetNodeRights", s.auth, oscript.M{"ID": id}); err != nil {
		return nil, err
	}

	var r NodeRights
	if err := errIn(c.Read(&r)); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Session) UpdateNodeRight(ctx context.Context, id int64, right NodeRight) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Write(docmanService, "UpdateNodeRight", s.auth,
		oscript.M{
			"ID":        id,
			"nodeRight": right,
		}); err != nil {
		return err
	}

	if err := errIn(c.Read(nil)); err != nil {
		return err
	}
	return nil
}

func (s *Session) RemoveNodeRight(ctx context.Context, id int64, right NodeRight) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Write(docmanService, "RemoveNodeRight", s.auth,
		oscript.M{
			"ID":        id,
			"nodeRight": right, // note: important ID and Type must be fill
		}); err != nil {
		return err
	}

	if err := errIn(c.Read(nil)); err != nil {
		return err
	}
	return nil
}
