package db

import (
	"database/sql"
	"fmt"

	// SQLite driver
	_ "modernc.org/sqlite"

	"github.com/meganerd/commons-votes/internal/model"
)

// Database wraps the SQL connection pool.
type Database struct {
	db *sql.DB
}

// NewDatabase opens or creates the SQLite database at the given path.
func NewDatabase(path string) (*Database, error) {

	dbConn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &Database{db: dbConn}, nil
}

// Init initializes tables for members and votes.
func (d *Database) Init() error {
	schema := `
CREATE TABLE IF NOT EXISTS members (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	party TEXT
);
CREATE TABLE IF NOT EXISTS bills (
	id INTEGER PRIMARY KEY,
	number TEXT NOT NULL,
	description TEXT,
	full_text_url TEXT
);
CREATE TABLE IF NOT EXISTS votes (
	id INTEGER PRIMARY KEY,
	bill_id INTEGER,
	member_id INTEGER,
	result TEXT,
	vote_date TEXT
);
`
	_, err := d.db.Exec(schema)
	return err
}

// InsertBill ensures the bill exists, then returns its true ID.
func (d *Database) InsertBill(b model.Bill) (int, error) {
	// 1) Upsert the bill (ignore if already present)
	if _, err := d.db.Exec(
		"INSERT OR IGNORE INTO bills(number, description, full_text_url) VALUES(?, ?, ?)",
		b.Number, b.Description, b.FullTextURL,
	); err != nil {
		return 0, err
	}
	// 2) Now fetch the canonical ID for that bill
	var id int
	err := d.db.QueryRow(
		"SELECT id FROM bills WHERE number = ? AND description = ?",
		b.Number, b.Description,
	).Scan(&id)
	return id, err
}

// InsertMember inserts a Member, ignoring if the ID already exists.
func (d *Database) InsertMember(m model.Member) error {
	// Insert member by name & party only; let SQLite assign the PRIMARY KEY (id)
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO members(name, party) VALUES(?, ?)",
		m.Name, m.Party,
	)
	return err
}

// InsertVote inserts a Vote record into the database.
func (d *Database) InsertVote(v model.Vote) error {
	// Insert vote; omit the id to let SQLite assign INTEGER PRIMARY KEY automatically
	_, err := d.db.Exec(
		"INSERT INTO votes(bill_id, member_id, result, vote_date) VALUES(?, ?, ?, ?)",
		v.BillID, v.MemberID, v.Result, v.VoteDate,
	)
	return err
}

// QueryRow wraps the underlying sql.DB.QueryRow for custom queries.
func (d *Database) QueryRow(query string, args ...interface{}) *sql.Row {
	return d.db.QueryRow(query, args...)
}

// Query wraps the underlying sql.DB.Query for custom queries.
func (d *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.Query(query, args...)
}

// Close shuts down the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// Exec runs an arbitrary SQL statement (used to BEGIN/COMMIT transactions and PRAGMAs).
func (d *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.db.Exec(query, args...)
}
