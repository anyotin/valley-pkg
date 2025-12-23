package mysql

import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"time"
)

type MysqlClient struct {
	config *mysql.Config
	db     *sql.DB
}

// NewMysqlClient コンストラクタ
func NewMysqlClient() (*MysqlClient, error) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	c := mysql.Config{
		DBName:               "sample",
		User:                 "root",
		Passwd:               "pass",
		Addr:                 "db:3306",
		Net:                  "tcp",
		ParseTime:            true,
		Collation:            "utf8mb4_unicode_ci",
		AllowNativePasswords: true,
		Loc:                  jst,
	}

	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return nil, err
	}

	// プール設定は任意（推奨）
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(10 * time.Minute)

	return &MysqlClient{config: &c, db: db}, nil
}
