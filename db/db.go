package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kagadar/go-pipeline"
	"github.com/kagadar/go-set"
	_ "modernc.org/sqlite"
)

type db struct {
	db *sql.DB
}

type DB interface {
	DeleteSoundboard(ctx context.Context, guildID discordgo.Snowflake) error
	FindAllSoundboardRoles(ctx context.Context, filter set.Set[discordgo.Snowflake]) (map[discordgo.Snowflake]set.Set[discordgo.Snowflake], error)
	FindSoundboardRoles(ctx context.Context, guildID discordgo.Snowflake, filter set.Set[discordgo.Snowflake]) (set.Set[discordgo.Snowflake], error)
	InsertAutoRole(ctx context.Context, guildID, roleID discordgo.Snowflake, templateRoleName string) error
	ListGuilds(ctx context.Context) (set.Set[discordgo.Snowflake], error)
	ListSoundboards(ctx context.Context) (set.Set[discordgo.Snowflake], error)
	UpsertSoundboard(ctx context.Context, guildID discordgo.Snowflake, roles map[string]discordgo.Snowflake) error
}

func (db *db) DeleteSoundboard(ctx context.Context, guildID discordgo.Snowflake) error {
	if _, err := db.db.ExecContext(ctx, `DELETE FROM Soundboards WHERE GuildID = ?;`, guildID); err != nil {
		return fmt.Errorf("%w: failed to delete soundboard %q", err, guildID)
	}
	return nil
}

func (db *db) FindAllSoundboardRoles(ctx context.Context, filter set.Set[discordgo.Snowflake]) (map[discordgo.Snowflake]set.Set[discordgo.Snowflake], error) {
	rows, err := db.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT s.GuildID, s.RoleID
		FROM AutoRoles AS a
			INNER JOIN SoundboardRoles AS s
				USING (TemplateRoleName)
		WHERE a.RoleID IN (%s);
	`, strings.Join(pipeline.MapToSlice(filter, func(k discordgo.Snowflake, _ set.Empty) string { return string(k) }), ",")))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to find soundboard roles", err)
	}
	out := map[discordgo.Snowflake]set.Set[discordgo.Snowflake]{}
	for rows.Next() {
		var guildID, roleID discordgo.Snowflake
		if err := rows.Scan(&guildID, &roleID); err != nil {
			return nil, fmt.Errorf("%w: failed to scan soundboard role", err)
		}
		s := out[guildID]
		if s == nil {
			out[guildID] = set.New(roleID)
		} else {
			s.Put(roleID)
		}
	}
	return out, nil
}

func (db *db) FindSoundboardRoles(ctx context.Context, guildID discordgo.Snowflake, filter set.Set[discordgo.Snowflake]) (set.Set[discordgo.Snowflake], error) {
	rows, err := db.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT s.RoleID
		FROM AutoRoles AS a
			INNER JOIN SoundboardRoles AS s
				USING (TemplateRoleName)
		WHERE s.GuildID = %s AND a.RoleID IN (%s);
	`, guildID, strings.Join(pipeline.MapToSlice(filter, func(k discordgo.Snowflake, _ set.Empty) string { return string(k) }), ",")))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to find soundboard roles", err)
	}
	out := set.Set[discordgo.Snowflake]{}
	for rows.Next() {
		var roleID discordgo.Snowflake
		if err := rows.Scan(&roleID); err != nil {
			return nil, fmt.Errorf("%w: failed to scan soundboard role", err)
		}
		out.Put(roleID)
	}
	return out, nil
}

func (db *db) InsertAutoRole(ctx context.Context, guildID, roleID discordgo.Snowflake, templateRoleName string) error {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to start transaction to add autorole %q", err, guildID)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO Guilds VALUES(?);
	`, guildID); err != nil {
		return fmt.Errorf("%w: failed to save guild %q", err, guildID)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO AutoRoles VALUES(?, ?, ?);
	`, guildID, roleID, templateRoleName); err != nil {
		return fmt.Errorf("%w: failed to save autorole %q", err, guildID)
	}
	return tx.Commit()
}

func (db *db) ListGuilds(ctx context.Context) (set.Set[discordgo.Snowflake], error) {
	rows, err := db.db.QueryContext(ctx, `
		SELECT * FROM Guilds;
	`)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list guilds", err)
	}
	guilds := set.New[discordgo.Snowflake]()
	for rows.Next() {
		var guild discordgo.Snowflake
		if err := rows.Scan(&guild); err != nil {
			return nil, fmt.Errorf("%w: failed to scan guild", err)
		}
		guilds.Put(guild)
	}
	return guilds, nil
}

func (db *db) ListSoundboards(ctx context.Context) (set.Set[discordgo.Snowflake], error) {
	rows, err := db.db.QueryContext(ctx, `
		SELECT * FROM Soundboards;
	`)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list soundboards", err)
	}
	guilds := set.New[discordgo.Snowflake]()
	for rows.Next() {
		var guild discordgo.Snowflake
		if err := rows.Scan(&guild); err != nil {
			return nil, fmt.Errorf("%w: failed to scan soundboard", err)
		}
		guilds.Put(guild)
	}
	return guilds, nil
}

func (db *db) UpsertSoundboard(ctx context.Context, guildID discordgo.Snowflake, roles map[string]discordgo.Snowflake) error {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to start transaction to insert soundboard %q", err, guildID)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO Soundboards VALUES(?);
	`, guildID); err != nil {
		return fmt.Errorf("%w: failed to save soundboard %q", err, guildID)
	}
	for roleName, roleID := range roles {
		if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO SoundboardRoles VALUES(?, ?, ?);
	`, guildID, roleName, roleID); err != nil {
			return fmt.Errorf("%w: failed to save soundboard %q role %q", err, guildID, roleName)
		}
	}
	return tx.Commit()
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
		CREATE TABLE IF NOT EXISTS AutoRoles (GuildID TEXT, RoleID TEXT, TemplateRoleName TEXT, PRIMARY KEY(GuildID, RoleID, TemplateRoleName), FOREIGN KEY(GuildID) REFERENCES Guilds ON DELETE CASCADE) STRICT;
		CREATE TABLE IF NOT EXISTS Soundboards (GuildID TEXT, PRIMARY KEY(GuildID)) STRICT;
		CREATE TABLE IF NOT EXISTS SoundboardRoles (GuildID TEXT, TemplateRoleName TEXT, RoleID TEXT, PRIMARY KEY(GuildID, TemplateRoleName, RoleID), FOREIGN KEY(GuildID) REFERENCES Soundboards ON DELETE CASCADE) STRICT;
	`); err != nil {
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}
	return &db{db: d}, nil
}
