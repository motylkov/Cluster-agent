// Package agents provides shared types and peer database management for cloud-agent.
package agents

import (
	"cloud-agent/internal/config"
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
)

// PeersDB wraps a sql.DB for peer management in SQLite.
type PeersDB struct {
	db *sql.DB
}

// InitPeersDB opens the SQLite database and creates the peers table if it does not exist.
func InitPeersDB(dbPath string) (*PeersDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", dbPath, err)
	}
	createTable := `CREATE TABLE IF NOT EXISTS peers (
		name TEXT PRIMARY KEY,
		addr TEXT NOT NULL,
		master BOOLEAN NOT NULL DEFAULT 0
	);`
	if _, err := db.ExecContext(context.Background(), createTable); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, fmt.Errorf("failed to create peers table: %v; also failed to close db: %w", err, cerr)
		}
		return nil, fmt.Errorf("failed to create peers table: %w", err)
	}

	return &PeersDB{db: db}, nil
}

// UpsertPeer inserts or updates a peer in the database.
func (d *PeersDB) UpsertPeer(peer config.PeerInfo) error {
	_, err := d.db.ExecContext(context.Background(), `INSERT INTO peers (name, addr, master) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET addr=excluded.addr, master=excluded.master;`,
		peer.Name, peer.Addr, peer.Master)
	if err != nil {
		return fmt.Errorf("failed to upsert peer: %w", err)
	}
	return nil
}

// RemovePeer deletes a peer from the database by name.
func (d *PeersDB) RemovePeer(name string) error {
	_, err := d.db.ExecContext(context.Background(), `DELETE FROM peers WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}
	return nil
}

// LoadPeersFromDB loads all peers from the database.
func (d *PeersDB) LoadPeersFromDB() (peers []config.PeerInfo, err error) {
	rows, err := d.db.QueryContext(context.Background(), `SELECT name, addr, master FROM peers`)
	if err != nil {
		return nil, fmt.Errorf("failed to query peers: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			fmt.Printf("failed to close rows: %v\n", cerr)
		}
	}()

	for rows.Next() {
		var p config.PeerInfo
		if err := rows.Scan(&p.Name, &p.Addr, &p.Master); err != nil {
			return nil, fmt.Errorf("failed to scan peer: %w", err)
		}
		peers = append(peers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error: %w", err)
	}
	return peers, nil
}
