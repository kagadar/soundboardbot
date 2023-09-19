package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type db struct {
	db *sql.DB
}

func (db *db) SaveInvite(guildID, inviteCode string, expiry *time.Time) error {
	_, err := db.db.Exec(`
		INSERT INTO Soundboards VALUES(?, ?, ?);
	`, guildID, inviteCode, expiry.Unix())
	return err
}

type DB interface {
	SaveInvite(guildID string, inviteCode string, expiry *time.Time) error
}

func New() (DB, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user's home directory: %w", err)
	}
	datadir := filepath.Join(hd, ".soundboardbot")
	if err := os.MkdirAll(datadir, 0700); err != nil {
		return nil, fmt.Errorf("failed to make data directory: %w", err)
	}
	d, err := sql.Open("sqlite", filepath.Join(datadir, "db"))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if _, err := d.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS Guilds (GuildID TEXT, PRIMARY KEY(GuildID)) STRICT;
		CREATE TABLE IF NOT EXISTS AutoRoles (GuildID TEXT, RoleID TEXT, TemplateRoleName TEXT, PRIMARY KEY(GuildID, RoleID, TemplateRoleName), FOREIGN KEY(GuildID) REFERENCES Guilds) STRICT;
		CREATE TABLE IF NOT EXISTS Soundboards (GuildID TEXT, InviteCode TEXT, Expiry INTEGER, PRIMARY KEY(GuildID)) STRICT;
		CREATE TABLE IF NOT EXISTS PendingInvites (GuildID TEXT, Member TEXT, PRIMARY KEY(GuildID, Member), FOREIGN KEY(GuildID) REFERENCES Soundboards) STRICT;
	`); err != nil {
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}
	return &db{db: d}, nil
}
