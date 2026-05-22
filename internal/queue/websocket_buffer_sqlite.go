package queue

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO)
)

const (
	sqliteCreateTable = `
CREATE TABLE IF NOT EXISTS ws_outbox (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    topic      TEXT    NOT NULL,
    payload    BLOB    NOT NULL,
    properties TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ws_outbox_expires ON ws_outbox(expires_at);
`
	sqliteInsert      = `INSERT INTO ws_outbox (topic, payload, properties, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`
	sqliteSelectAll   = `SELECT id, topic, payload, properties FROM ws_outbox WHERE expires_at > ? ORDER BY id ASC`
	sqliteDeleteByID  = `DELETE FROM ws_outbox WHERE id = ?`
	sqliteCount       = `SELECT COUNT(*) FROM ws_outbox WHERE expires_at > ?`
	sqlitePruneExpired = `DELETE FROM ws_outbox WHERE expires_at <= ?`
)

// bufferedMessage is a message stored in the SQLite offline buffer
type bufferedMessage struct {
	ID         int64
	Topic      string
	Payload    []byte
	Properties MessageHeaders
}

// wsOfflineBuffer is a durable store-and-forward buffer for the WebSocket ant client.
// Messages are persisted here when the ant is disconnected from the queen; they are
// drained and forwarded after reconnection.
type wsOfflineBuffer struct {
	db           *sql.DB
	maxSize      int64
	ttl          time.Duration
	mu           sync.Mutex
}

// newWSOfflineBuffer opens (or creates) the SQLite buffer database at the given path.
func newWSOfflineBuffer(dbPath string, maxSize int64, ttl time.Duration) (*wsOfflineBuffer, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite buffer %s: %w", dbPath, err)
	}

	// Single-writer mode to avoid SQLITE_BUSY contention
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(sqliteCreateTable); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create ws_outbox table: %w", err)
	}

	return &wsOfflineBuffer{
		db:      db,
		maxSize: maxSize,
		ttl:     ttl,
	}, nil
}

// Enqueue persists a message for later delivery.
// Returns an error if the buffer is full.
func (b *wsOfflineBuffer) Enqueue(topic string, payload []byte, props MessageHeaders) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().Unix()
	count, err := b.countLocked(now)
	if err != nil {
		return fmt.Errorf("buffer count: %w", err)
	}
	if count >= b.maxSize {
		return fmt.Errorf("offline buffer full (%d/%d messages)", count, b.maxSize)
	}

	propsJSON, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("marshal properties: %w", err)
	}

	expiresAt := time.Now().Add(b.ttl).Unix()
	_, err = b.db.Exec(sqliteInsert, topic, payload, string(propsJSON), now, expiresAt)
	return err
}

// DequeueAll returns all non-expired buffered messages in insertion order.
// The caller must call Remove for each message after successful delivery.
func (b *wsOfflineBuffer) DequeueAll() ([]*bufferedMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Prune expired messages first
	now := time.Now().Unix()
	if _, err := b.db.Exec(sqlitePruneExpired, now); err != nil {
		return nil, fmt.Errorf("prune expired messages: %w", err)
	}

	rows, err := b.db.Query(sqliteSelectAll, now)
	if err != nil {
		return nil, fmt.Errorf("query outbox: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var msgs []*bufferedMessage
	for rows.Next() {
		var m bufferedMessage
		var propsJSON string
		if err := rows.Scan(&m.ID, &m.Topic, &m.Payload, &propsJSON); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &m.Properties); err != nil {
			return nil, fmt.Errorf("unmarshal properties for id %d: %w", m.ID, err)
		}
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

// Remove deletes a message from the buffer by its row ID.
func (b *wsOfflineBuffer) Remove(id int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.db.Exec(sqliteDeleteByID, id)
	return err
}

// Count returns the number of non-expired messages currently buffered.
func (b *wsOfflineBuffer) Count() (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.countLocked(time.Now().Unix())
}

func (b *wsOfflineBuffer) countLocked(nowUnix int64) (int64, error) {
	var n int64
	err := b.db.QueryRow(sqliteCount, nowUnix).Scan(&n)
	return n, err
}

// Close releases the database handle.
func (b *wsOfflineBuffer) Close() error {
	return b.db.Close()
}
