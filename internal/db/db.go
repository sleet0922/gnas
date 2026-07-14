package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	once sync.Once
)

// Init 初始化数据库，dbPath 为数据库文件路径
func Init(dbPath string) error {
	var initErr error
	once.Do(func() {
		// 确保目录存在
		os.MkdirAll(filepath.Dir(dbPath), 0755)

		var err error
		db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
		if err != nil {
			initErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}

		// 性能优化
		db.SetMaxOpenConns(1) // SQLite 单写
		db.SetMaxIdleConns(1)

		if err = migrate(); err != nil {
			initErr = fmt.Errorf("数据库迁移失败: %w", err)
			return
		}
	})
	return initErr
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return db
}

// Close 关闭数据库
func Close() {
	if db != nil {
		db.Close()
	}
}

// migrate 数据库迁移
func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS file_meta (
		path TEXT PRIMARY KEY,
		display_name TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '',
		favorite INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}

// --- 通用查询辅助 ---

// QueryOne 查询单行
func QueryOne(dest interface{}, query string, args ...interface{}) error {
	// 由调用方自行 scan
	return nil
}

// GetSetting 获取设置
func GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting 设置
func SetSetting(key, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

// --- 用户 ---

// User 用户模型
type User struct {
	ID        int64
	Username  string
	Password  string
	CreatedAt string
}

// GetUser 获取用户
func GetUser(username string) (*User, error) {
	u := &User{}
	err := db.QueryRow("SELECT id, username, password, created_at FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUserByID 通过 ID 获取用户
func GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow("SELECT id, username, password, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// CreateUser 创建用户
func CreateUser(username, hashedPassword string) error {
	_, err := db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", username, hashedPassword)
	return err
}

// UpdateUserPassword 更新密码
func UpdateUserPassword(username, hashedPassword string) error {
	_, err := db.Exec("UPDATE users SET password = ? WHERE username = ?", hashedPassword, username)
	return err
}

// HasAnyUser 是否有用户
func HasAnyUser() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

// GetAllUsers 获取所有用户
func GetAllUsers() ([]*User, error) {
	rows, err := db.Query("SELECT id, username, password, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}



// --- 文件元数据 ---

// FileMeta 文件元数据
type FileMeta struct {
	Path        string `json:"path"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Favorite    bool   `json:"favorite"`
}

// GetFileMeta 获取文件元数据
func GetFileMeta(path string) (*FileMeta, error) {
	m := &FileMeta{}
	var fav int
	err := db.QueryRow("SELECT path, display_name, description, tags, favorite FROM file_meta WHERE path = ?", path).
		Scan(&m.Path, &m.DisplayName, &m.Description, &m.Tags, &fav)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	m.Favorite = fav == 1
	return m, err
}

// SetFileMeta 设置文件元数据
func SetFileMeta(m *FileMeta) error {
	fav := 0
	if m.Favorite {
		fav = 1
	}
	_, err := db.Exec("INSERT OR REPLACE INTO file_meta (path, display_name, description, tags, favorite, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
		m.Path, m.DisplayName, m.Description, m.Tags, fav)
	return err
}

// GetFavoriteFiles 获取收藏文件
func GetFavoriteFiles() ([]FileMeta, error) {
	rows, err := db.Query("SELECT path, display_name, description, tags, favorite FROM file_meta WHERE favorite = 1 ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []FileMeta
	for rows.Next() {
		var m FileMeta
		var fav int
		if err := rows.Scan(&m.Path, &m.DisplayName, &m.Description, &m.Tags, &fav); err != nil {
			return nil, err
		}
		m.Favorite = fav == 1
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if metas == nil {
		metas = []FileMeta{}
	}
	return metas, nil
}

// DeleteFileMeta 删除文件元数据
func DeleteFileMeta(path string) error {
	_, err := db.Exec("DELETE FROM file_meta WHERE path = ?", path)
	return err
}

