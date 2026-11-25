package store

import "errors"

// ErrNoDSN 表示没配置数据库。
var ErrNoDSN = errors.New("store: DB_DSN 未配置")
