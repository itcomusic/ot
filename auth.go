package ot

import (
	"context"
	"time"

	"github.com/itcomusic/ot/pkg/oscript"
)

const authService = "Authentication"

// GetToken creates token.
func (s *Session) GetToken(ctx context.Context, username, password string) (string, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return "", err
	}
	defer c.Close()

	var t string
	if err := errIn(c.Exec(authService, "AuthenticateUser", s.auth, oscript.M{
		"userName":     username,
		"userPassword": password,
	}, &t)); err != nil {
		return "", err
	}
	return t, nil
}

// RefreshToken refreshes or creates token.
func (s *Session) RefreshToken(ctx context.Context) (string, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return "", err
	}
	defer c.Close()

	var t string
	if err := errIn(c.Exec(authService, "RefreshToken", s.auth, oscript.M{}, &t)); err != nil {
		return "", err
	}
	return t, nil
}

// GetSessionExpiration returns expiration time.
func (s *Session) GetSessionExpiration(ctx context.Context) (time.Time, error) {
	c, err := s.connect(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer c.Close()

	var t time.Time
	if err := errIn(c.Exec(authService, "GetSessionExpirationDate", s.auth, oscript.M{}, &t)); err != nil {
		return time.Time{}, err
	}
	return t, nil
}
