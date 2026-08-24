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

	CREATE TABLE IF NOT EXISTS duplicate_cache (
		cache_key TEXT PRIMARY KEY,
		signature TEXT NOT NULL,
		result_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS recycle_bin (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_path TEXT NOT NULL,
		stored_path TEXT NOT NULL,
		thumb_stored_path TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		is_video INTEGER NOT NULL DEFAULT 0,
		is_dir INTEGER NOT NULL DEFAULT 0,
		deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expire_at DATETIME NOT NULL
	);
	`
	_, err := db.Exec(schema)
	return err
}

// --- 通用查询辅助 ---

// QueryOne 查询单行
func QueryOne(dest interface{}, query string, args ...interface{}) error {
	return fmt.Errorf("QueryOne not implemented")
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

// SetSettings updates several settings atomically.
func SetSettings(settings map[string]string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for key, value := range settings {
		if _, err := tx.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
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

// --- 回收站 ---

// RecycleItem 回收站记录
type RecycleItem struct {
	ID             int64  `json:"id"`
	OriginalPath   string `json:"originalPath"`
	StoredPath     string `json:"storedPath"`
	ThumbStoredPath string `json:"thumbStoredPath"`
	Name           string `json:"name"`
	IsVideo        bool   `json:"isVideo"`
	IsDir          bool   `json:"isDir"`
	DeletedAt      string `json:"deletedAt"`
	ExpireAt       string `json:"expireAt"`
}

// AddRecycleItem 添加回收站记录
func AddRecycleItem(item *RecycleItem) (int64, error) {
	isVideo := 0
	if item.IsVideo {
		isVideo = 1
	}
	isDir := 0
	if item.IsDir {
		isDir = 1
	}
	res, err := db.Exec(`INSERT INTO recycle_bin
		(original_path, stored_path, thumb_stored_path, name, is_video, is_dir, deleted_at, expire_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`,
		item.OriginalPath, item.StoredPath, item.ThumbStoredPath, item.Name, isVideo, isDir, item.ExpireAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetRecycleItem 获取单条回收站记录
func GetRecycleItem(id int64) (*RecycleItem, error) {
	item := &RecycleItem{}
	var isVideo, isDir int
	err := db.QueryRow(`SELECT id, original_path, stored_path, thumb_stored_path, name, is_video, is_dir, deleted_at, expire_at
		FROM recycle_bin WHERE id = ?`, id).
		Scan(&item.ID, &item.OriginalPath, &item.StoredPath, &item.ThumbStoredPath, &item.Name, &isVideo, &isDir, &item.DeletedAt, &item.ExpireAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	item.IsVideo = isVideo == 1
	item.IsDir = isDir == 1
	return item, err
}

// ListRecycleItems 列出所有回收站记录
func ListRecycleItems() ([]RecycleItem, error) {
	rows, err := db.Query(`SELECT id, original_path, stored_path, thumb_stored_path, name, is_video, is_dir, deleted_at, expire_at
		FROM recycle_bin ORDER BY deleted_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RecycleItem
	for rows.Next() {
		var item RecycleItem
		var isVideo, isDir int
		if err := rows.Scan(&item.ID, &item.OriginalPath, &item.StoredPath, &item.ThumbStoredPath, &item.Name, &isVideo, &isDir, &item.DeletedAt, &item.ExpireAt); err != nil {
			return nil, err
		}
		item.IsVideo = isVideo == 1
		item.IsDir = isDir == 1
		items = append(items, item)
	}
	if items == nil {
		items = []RecycleItem{}
	}
	return items, rows.Err()
}

// ListExpiredRecycleItems 列出已过期的回收站记录
func ListExpiredRecycleItems() ([]RecycleItem, error) {
	rows, err := db.Query(`SELECT id, original_path, stored_path, thumb_stored_path, name, is_video, is_dir, deleted_at, expire_at
		FROM recycle_bin WHERE expire_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RecycleItem
	for rows.Next() {
		var item RecycleItem
		var isVideo, isDir int
		if err := rows.Scan(&item.ID, &item.OriginalPath, &item.StoredPath, &item.ThumbStoredPath, &item.Name, &isVideo, &isDir, &item.DeletedAt, &item.ExpireAt); err != nil {
			return nil, err
		}
		item.IsVideo = isVideo == 1
		item.IsDir = isDir == 1
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteRecycleItem 删除回收站记录
func DeleteRecycleItem(id int64) error {
	_, err := db.Exec("DELETE FROM recycle_bin WHERE id = ?", id)
	return err
}

// ClearRecycleBin 清空回收站记录
func ClearRecycleBin() error {
	_, err := db.Exec("DELETE FROM recycle_bin")
	return err
}
