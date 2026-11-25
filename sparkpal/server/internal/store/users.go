package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// User 是伙伴账号。注意：它和抖音号是一对多，不是同一个东西。
type User struct {
	ID           string
	Phone        sql.NullString
	Email        sql.NullString
	Nickname     sql.NullString
	Plan         string
	PlanExpireAt sql.NullTime
}

// NewID 生成 36 位 id（带横线，和星语习惯一致）。
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))[:32]
	}
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// UpsertUserByPhone 手机号找不到就建，找到就返回。
func (s *Store) UpsertUserByPhone(phone string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, phone, email, nickname, plan, planExpireAt
		 FROM HuoUser WHERE phone = ? LIMIT 1`, phone).
		Scan(&u.ID, &u.Phone, &u.Email, &u.Nickname, &u.Plan, &u.PlanExpireAt)
	if err == nil {
		_, _ = s.db.Exec(`UPDATE HuoUser SET updatedAt = ? WHERE id = ?`, time.Now(), u.ID)
		return &u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	u = User{ID: NewID(), Phone: sql.NullString{String: phone, Valid: true}, Plan: "free"}
	now := time.Now()
	_, err = s.db.Exec(
		`INSERT INTO HuoUser (id, phone, plan, createdAt, updatedAt) VALUES (?, ?, 'free', ?, ?)`,
		u.ID, phone, now, now)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUser 按 id 查。
func (s *Store) GetUser(id string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, phone, email, nickname, plan, planExpireAt
		 FROM HuoUser WHERE id = ? LIMIT 1`, id).
		Scan(&u.ID, &u.Phone, &u.Email, &u.Nickname, &u.Plan, &u.PlanExpireAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// SetInviteCode 记录用户使用的邀请码（审计用）。
func (s *Store) SetInviteCode(userID, code string) {
	_, _ = s.db.Exec(`UPDATE HuoUser SET inviteCode = ? WHERE id = ?`, code, userID)
}

// TouchDevice 记录设备。
func (s *Store) TouchDevice(userID, deviceID, platform string) {
	if deviceID == "" {
		return
	}
	_, _ = s.db.Exec(
		`INSERT INTO HuoDevice (id, userId, deviceId, platform, lastActiveAt)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE lastActiveAt = VALUES(lastActiveAt), platform = VALUES(platform)`,
		NewID(), userID, deviceID, platform, time.Now())
}
