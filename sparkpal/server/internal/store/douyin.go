package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/fengyun0608/sparkpal/server/internal/douyin"
)

// DouyinAccount 一个绑定的抖音号。一个伙伴账号可以有多个。
type DouyinAccount struct {
	ID          string
	UserID      string
	UID         string
	SecUID      string
	Nickname    string
	Avatar      string
	Status      string
	FriendCount int
	CreatedAt   time.Time
}

// UpsertDouyin 绑定或更新。
//
// 换号时必须清空旧好友：星语那边踩过一次——
// accountId 没变，上一个号的好友全继承过来，列表里全是自己。
func (s *Store) UpsertDouyin(ctx context.Context, userID, uid, secUID, cookie, nick, avatar string) (*DouyinAccount, error) {
	var id string
	var oldUID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(uid,'') FROM HuoDouyin WHERE userId = ? ORDER BY createdAt LIMIT 1`,
		userID).Scan(&id, &oldUID)

	now := time.Now()
	enc := encrypt(cookie)

	if err == sql.ErrNoRows {
		id = NewID()
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO HuoDouyin (id, userId, uid, secUid, cookieEnc, nickname, avatar, status, createdAt, updatedAt)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
			id, userID, uid, secUID, enc, nick, avatar, now, now)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if oldUID != "" && oldUID != uid {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM HuoFriend WHERE accountId = ?`, id)
			_, _ = s.db.ExecContext(ctx, `DELETE FROM HuoTask WHERE accountId = ?`, id)
			_, _ = s.db.ExecContext(ctx, `DELETE FROM HuoLog WHERE accountId = ?`, id)
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE HuoDouyin SET uid=?, secUid=?, cookieEnc=?, nickname=?, avatar=?,
			   status='active', updatedAt=? WHERE id=?`,
			uid, secUID, enc, nick, avatar, now, id)
		if err != nil {
			return nil, err
		}
	}
	return &DouyinAccount{ID: id, UserID: userID, UID: uid, SecUID: secUID,
		Nickname: nick, Avatar: avatar, Status: "active"}, nil
}

// ReplaceFriends 用最新会话列表覆盖好友。
//
// 规则：跳过自己；跳过 convId 里"我这一侧"是本号的记录。
func (s *Store) ReplaceFriends(ctx context.Context, accountID, selfUID string, convs []douyin.Conversation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM HuoFriend WHERE accountId = ?`, accountID); err != nil {
		return err
	}

	ins, err := tx.PrepareContext(ctx,
		`INSERT INTO HuoFriend
		   (id, accountId, convId, uid, secUid, nickname, avatar, sparkDays, followStatus, selected, updatedAt)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()

	now := time.Now()
	seen := map[string]bool{}
	n := 0
	for _, c := range convs {
		if c.UID == "" || c.UID == selfUID {
			continue
		}
		if seen[c.ConvID] {
			continue
		}
		seen[c.ConvID] = true
		if _, err := ins.ExecContext(ctx,
			NewID(), accountID, c.ConvID, c.UID, c.SecUID, c.Nickname, c.AvatarURL,
			c.SparkDays, 0, now); err != nil {
			return err
		}
		n++
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE HuoDouyin SET friendCount = ? WHERE id = ?`, n, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListDouyin 某人的全部抖音号。
func (s *Store) ListDouyin(ctx context.Context, userID string) ([]DouyinAccount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, userId, COALESCE(uid,''), COALESCE(secUid,''),
		        COALESCE(nickname,''), COALESCE(avatar,''), COALESCE(status,'active'),
		        COALESCE(friendCount,0), createdAt
		 FROM HuoDouyin WHERE userId = ? ORDER BY createdAt`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DouyinAccount
	for rows.Next() {
		var a DouyinAccount
		if err := rows.Scan(&a.ID, &a.UserID, &a.UID, &a.SecUID,
			&a.Nickname, &a.Avatar, &a.Status, &a.FriendCount, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Cookie 解密出 cookie（只有内部发送用，接口永不外泄）。
func (s *Store) Cookie(accountID string) (string, error) {
	var enc string
	if err := s.db.QueryRow(`SELECT COALESCE(cookieEnc,'') FROM HuoDouyin WHERE id = ?`, accountID).Scan(&enc); err != nil {
		return "", err
	}
	return decrypt(enc), nil
}
