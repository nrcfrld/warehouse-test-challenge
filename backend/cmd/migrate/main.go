// Command migrate applies embedded, versioned SQL migrations.
package main

import (
	"errors"
	"log"
	"os"

	"github.com/enrico/warehouse/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, url)
	if err != nil {
		return err
	}
	defer func() {
		s, d := m.Close()
		if s != nil {
			log.Print(s)
		}
		if d != nil {
			log.Print(d)
		}
	}()
	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
