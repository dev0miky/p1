package db

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

func Migrate(sqlDB *sql.DB, fs embed.FS, dir string) error {
	goose.SetBaseFS(fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDB, dir)
}
