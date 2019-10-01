package ot

import (
	"context"

	"github.com/itcomusic/ot/pkg/oscript"
)

const memberService = "MemberService"

// CreateGroup creates group.
func (s *Session) CreateGroup(ctx context.Context, name string, leaderID *string) (int64, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return 0, err
	}
	defer c.Close()

	var id int64
	if err := errIn(c.Exec(memberService, "CreateGroup", s.auth, oscript.M{"name": name, "leaderID": leaderID}, &id)); err != nil {
		return 0, err
	}
	return id, nil
}
