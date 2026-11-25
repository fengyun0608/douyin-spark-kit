package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrInviteInvalid 邀请码不可用。
var ErrInviteInvalid = errors.New("invite invalid")

// Invite 邀请码。
type Invite struct {
	Code     string
	MaxUses  int
	Used     int
	ExpireAt sql.NullTime
}

// GetInvite 查邀请码。
func (s *Store) GetInvite(code string) (*Invite, error) {
	var it Invite
	err := s.db.QueryRow(
		`SELECT code, maxUses, used, expireAt FROM HuoInvite WHERE code = ?`, code).
		Scan(&it.Code, &it.MaxUses, &it.Used, &it.ExpireAt)
	if err == sql.ErrNoRows {
		return nil, ErrInviteInvalid
	}
	if err != nil {
		return nil, err
	}
	if it.ExpireAt.Valid && it.ExpireAt.Time.Before(time.Now()) {
		return nil, ErrInviteInvalid
	}
	if it.Used >= it.MaxUses {
		return nil, ErrInviteInvalid
	}
	return &it, nil
}

// ConsumeInvite 占用一次。
func (s *Store) ConsumeInvite(code string) error {
	res, err := s.db.Exec(
		`UPDATE HuoInvite SET used = used + 1 WHERE code = ? AND used < maxUses`, code)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInviteInvalid
	}
	return nil
}

// CreateInvite 建邀请码（运维用）。
func (s *Store) CreateInvite(code string, maxUses int, ttl time.Duration) error {
	var exp any
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	_, err := s.db.Exec(
		`INSERT INTO HuoInvite (code, maxUses, used, expireAt, createdAt)
		 VALUES (?, ?, 0, ?, ?)
		 ON DUPLICATE KEY UPDATE maxUses = VALUES(maxUses), expireAt = VALUES(expireAt)`,
		code, maxUses, exp, time.Now())
	return err
}
