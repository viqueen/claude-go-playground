package migrate

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

func Run(db *sql.DB, migrations embed.FS, dir string) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
