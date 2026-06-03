package main

import (
	"database/sql"
	"embed"
	"flag"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DB_URL)")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = os.Getenv("DB_URL")
	}
	if *dbURL == "" {
		log.Fatal("Missing -db flag or DB_URL env variable")
	}

	db, err := sql.Open("pgx", *dbURL)
	if err != nil {
		log.Fatal("Failed to open db: ", err)
	}
	defer db.Close()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}
}
