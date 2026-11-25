// Package store 负责建表与查询。
//
// 表名统一 Huo 前缀，和星语的 Douyin* 表完全隔离；
// 自建时可以用独立库（DB_DSN 指向 sparkpal 库）。
package store

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Store 包一层 *sql.DB。
type Store struct{ db *sql.DB }

// Open 连接并建表。
func Open(dsn string) (*Store, error) {
	if dsn == "" {
		return nil, ErrNoDSN
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close 关闭连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层连接给后续模块（好友、任务等）。
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS HuoUser (
			id            CHAR(36)    NOT NULL PRIMARY KEY,
			phone         VARCHAR(20) NULL,
			email         VARCHAR(120) NULL,
			passwordHash  VARCHAR(120) NULL,
			nickname      VARCHAR(64) NULL,
			plan          VARCHAR(16) NOT NULL DEFAULT 'free',
			planExpireAt  DATETIME    NULL,
			inviteCode    VARCHAR(16) NULL,
			createdAt     DATETIME(3) NOT NULL,
			updatedAt     DATETIME(3) NOT NULL,
			UNIQUE KEY uk_phone (phone),
			UNIQUE KEY uk_email (email)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS HuoInvite (
			code      VARCHAR(16) NOT NULL PRIMARY KEY,
			maxUses   INT         NOT NULL DEFAULT 3,
			used      INT         NOT NULL DEFAULT 0,
			expireAt  DATETIME    NULL,
			createdAt DATETIME(3) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS HuoDevice (
			id           CHAR(36)    NOT NULL PRIMARY KEY,
			userId       CHAR(36)    NOT NULL,
			deviceId     VARCHAR(64) NOT NULL,
			platform     VARCHAR(16) NULL,
			lastActiveAt DATETIME(3) NULL,
			UNIQUE KEY uk_user_device (userId, deviceId)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS HuoDouyin (
			id          CHAR(36)    NOT NULL PRIMARY KEY,
			userId      CHAR(36)    NOT NULL,
			uid         VARCHAR(32) NULL,
			secUid      VARCHAR(96) NULL,
			cookieEnc   TEXT        NULL,
			nickname    VARCHAR(96) NULL,
			avatar      VARCHAR(512) NULL,
			status      VARCHAR(16) NOT NULL DEFAULT 'active',
			friendCount INT         NOT NULL DEFAULT 0,
			createdAt   DATETIME(3) NOT NULL,
			updatedAt   DATETIME(3) NOT NULL,
			KEY idx_user (userId)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS HuoFriend (
			id           CHAR(36)    NOT NULL PRIMARY KEY,
			accountId    CHAR(36)    NOT NULL,
			convId       VARCHAR(96) NOT NULL,
			uid          VARCHAR(32) NULL,
			secUid       VARCHAR(96) NULL,
			nickname     VARCHAR(96) NULL,
			avatar       VARCHAR(512) NULL,
			sparkDays    INT         NOT NULL DEFAULT 0,
			followStatus TINYINT     NOT NULL DEFAULT 0,
			selected     TINYINT     NOT NULL DEFAULT 0,
			updatedAt    DATETIME(3) NOT NULL,
			KEY idx_acc (accountId),
			UNIQUE KEY uk_acc_conv (accountId, convId)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS HuoTask (
			id          CHAR(36)    NOT NULL PRIMARY KEY,
			accountId   CHAR(36)    NOT NULL,
			hour        TINYINT     NOT NULL DEFAULT 0,
			minute      TINYINT     NOT NULL DEFAULT 0,
			jitter      TINYINT     NOT NULL DEFAULT 10,
			maxPerDay   INT         NOT NULL DEFAULT 50,
			texts       TEXT        NULL,
			enabled     TINYINT     NOT NULL DEFAULT 0,
			lastRunDate VARCHAR(10) NULL,
			updatedAt   DATETIME(3) NOT NULL,
			UNIQUE KEY uk_acc (accountId)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS HuoLog (
			id        CHAR(36)     NOT NULL PRIMARY KEY,
			accountId CHAR(36)     NOT NULL,
			friendId  CHAR(36)     NULL,
			text      VARCHAR(512) NULL,
			ok        TINYINT      NOT NULL DEFAULT 0,
			error     VARCHAR(255) NULL,
			createdAt DATETIME(3)  NOT NULL,
			KEY idx_acc_time (accountId, createdAt)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
