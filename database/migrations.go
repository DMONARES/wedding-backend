package database

import (
	"database/sql"
	"log"
)

func RunMigrations(db *sql.DB) {
	groupTable := `
	CREATE TABLE IF NOT EXISTS guest_groups (
		id SERIAL PRIMARY KEY,
		main_guest_name TEXT NOT NULL,
		comment TEXT,
		guest_count INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	guestsTable := `
	CREATE TABLE IF NOT EXISTS guests (
		id SERIAL PRIMARY KEY,
		group_id INTEGER NOT NULL REFERENCES guest_groups(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		is_main BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(groupTable)
	if err != nil {
		log.Fatal("Migration failed (guest_groups): ", err)
	}

	_, err = db.Exec(guestsTable)
	if err != nil {
		log.Fatal("Migration failed (guests): ", err)
	}

	log.Println("✅ Migrations completed")
}
