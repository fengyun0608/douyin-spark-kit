package douyin

import "database/sql"

// Manager 在星语里是个大对象；伙伴只需要它承载「拿用户资料」的能力，
// 所以这里裁到最小：一个数据库句柄。
type Manager struct {
	db *sql.DB
}

// NewManager 建一个。
func NewManager(db *sql.DB) *Manager { return &Manager{db: db} }

// DB 便于调用方拿连接。
func (m *Manager) DB() *sql.DB { return m.db }
