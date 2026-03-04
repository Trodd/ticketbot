package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// TicketType represents different types of tickets
type TicketType string

const (
	TicketTypeSupport     TicketType = "support"
	TicketTypeAppeal      TicketType = "appeal"
	TicketTypeReport      TicketType = "report"
	TicketTypeApplication TicketType = "application"
)

// TicketStatus represents the status of a ticket
type TicketStatus string

const (
	StatusOpen     TicketStatus = "open"
	StatusClaimed  TicketStatus = "claimed"
	StatusPending  TicketStatus = "pending"
	StatusApproved TicketStatus = "approved"
	StatusDenied   TicketStatus = "denied"
	StatusClosed   TicketStatus = "closed"
)

// Ticket represents a support ticket
type Ticket struct {
	ID           int64
	TicketNumber int64 // Per-guild sequential number starting from 1
	ChannelID    string
	UserID       string
	GuildID      string
	Type         TicketType
	Subject      string
	Description  string
	Status       TicketStatus
	Priority     string
	ClaimedBy    *string
	ClaimedAt    *time.Time
	FormData     map[string]string
	CreatedAt    time.Time
	ClosedAt     *time.Time
	ClosedBy     *string
	Resolution   *string
}

// Blacklist represents a blacklisted user
type Blacklist struct {
	ID        int64
	UserID    string
	GuildID   string
	Reason    string
	AddedBy   string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// Transcript represents a saved ticket transcript
type Transcript struct {
	ID        int64
	TicketID  int64
	ChannelID string
	GuildID   string
	UserID    string
	Content   string
	CreatedAt time.Time
}

// StaffNote represents an internal note on a ticket
type StaffNote struct {
	ID        int64
	TicketID  int64
	AuthorID  string
	Content   string
	CreatedAt time.Time
}

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes tables
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// migrate creates the necessary tables
func (db *DB) migrate() error {
	// Create tables
	tableQueries := []string{
		`CREATE TABLE IF NOT EXISTS tickets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id TEXT UNIQUE NOT NULL,
			user_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			type TEXT DEFAULT 'support',
			subject TEXT DEFAULT '',
			description TEXT DEFAULT '',
			status TEXT DEFAULT 'open',
			priority TEXT DEFAULT 'normal',
			claimed_by TEXT,
			claimed_at DATETIME,
			form_data TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME,
			closed_by TEXT,
			resolution TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS blacklist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			reason TEXT DEFAULT '',
			added_by TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			UNIQUE(user_id, guild_id)
		)`,
		`CREATE TABLE IF NOT EXISTS transcripts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			channel_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS staff_notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			author_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, q := range tableQueries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
		}
	}

	// Add missing columns to existing tickets table (migrations for existing databases)
	alterQueries := []string{
		`ALTER TABLE tickets ADD COLUMN type TEXT DEFAULT 'support'`,
		`ALTER TABLE tickets ADD COLUMN description TEXT DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN priority TEXT DEFAULT 'normal'`,
		`ALTER TABLE tickets ADD COLUMN claimed_by TEXT`,
		`ALTER TABLE tickets ADD COLUMN claimed_at DATETIME`,
		`ALTER TABLE tickets ADD COLUMN form_data TEXT DEFAULT '{}'`,
		`ALTER TABLE tickets ADD COLUMN closed_by TEXT`,
		`ALTER TABLE tickets ADD COLUMN resolution TEXT`,
		`ALTER TABLE tickets ADD COLUMN ticket_number INTEGER DEFAULT 0`,
	}

	for _, q := range alterQueries {
		// Ignore errors - columns may already exist
		db.conn.Exec(q)
	}

	// Create indexes after columns exist
	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tickets_channel_id ON tickets(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tickets_type ON tickets(type)`,
		`CREATE INDEX IF NOT EXISTS idx_blacklist_user ON blacklist(user_id, guild_id)`,
	}

	for _, q := range indexQueries {
		db.conn.Exec(q) // Ignore errors for existing indexes
	}

	return nil
}

// GetNextTicketNumberByType returns the next ticket number for a specific type in a guild (starts from 1)
func (db *DB) GetNextTicketNumberByType(guildID string, ticketType TicketType) int64 {
	var maxNum int64
	err := db.conn.QueryRow("SELECT COALESCE(MAX(ticket_number), 0) FROM tickets WHERE guild_id = ? AND type = ?", guildID, ticketType).Scan(&maxNum)
	if err != nil {
		maxNum = 0
	}
	return maxNum + 1
}

// CreateTicket inserts a new ticket record
func (db *DB) CreateTicket(channelID, userID, guildID string, ticketType TicketType, subject, description string, formData map[string]string) (*Ticket, error) {
	formJSON, _ := json.Marshal(formData)

	// Get next ticket number for this type in this guild (starts from 1)
	var maxNum int64
	err := db.conn.QueryRow("SELECT COALESCE(MAX(ticket_number), 0) FROM tickets WHERE guild_id = ? AND type = ?", guildID, ticketType).Scan(&maxNum)
	if err != nil {
		maxNum = 0
	}
	ticketNumber := maxNum + 1

	result, err := db.conn.Exec(
		"INSERT INTO tickets (channel_id, user_id, guild_id, type, subject, description, form_data, ticket_number) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		channelID, userID, guildID, ticketType, subject, description, string(formJSON), ticketNumber,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Ticket{
		ID:           id,
		TicketNumber: ticketNumber,
		ChannelID:    channelID,
		UserID:       userID,
		GuildID:      guildID,
		Type:         ticketType,
		Subject:      subject,
		Description:  description,
		Status:       StatusOpen,
		Priority:     "normal",
		FormData:     formData,
		CreatedAt:    time.Now(),
	}, nil
}

// GetTicketByChannel retrieves a ticket by its channel ID
func (db *DB) GetTicketByChannel(channelID string) (*Ticket, error) {
	ticket := &Ticket{}
	var formJSON string
	err := db.conn.QueryRow(
		`SELECT id, COALESCE(ticket_number, id) as ticket_number, channel_id, user_id, guild_id, type, subject, description, status, priority, 
		claimed_by, claimed_at, form_data, created_at, closed_at, closed_by, resolution 
		FROM tickets WHERE channel_id = ?`,
		channelID,
	).Scan(&ticket.ID, &ticket.TicketNumber, &ticket.ChannelID, &ticket.UserID, &ticket.GuildID, &ticket.Type,
		&ticket.Subject, &ticket.Description, &ticket.Status, &ticket.Priority,
		&ticket.ClaimedBy, &ticket.ClaimedAt, &formJSON, &ticket.CreatedAt,
		&ticket.ClosedAt, &ticket.ClosedBy, &ticket.Resolution)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ticket.FormData = make(map[string]string)
	json.Unmarshal([]byte(formJSON), &ticket.FormData)
	return ticket, nil
}

// GetTicketByID retrieves a ticket by its ID
func (db *DB) GetTicketByID(id int64) (*Ticket, error) {
	ticket := &Ticket{}
	var formJSON string
	err := db.conn.QueryRow(
		`SELECT id, channel_id, user_id, guild_id, type, subject, description, status, priority, 
		claimed_by, claimed_at, form_data, created_at, closed_at, closed_by, resolution 
		FROM tickets WHERE id = ?`,
		id,
	).Scan(&ticket.ID, &ticket.ChannelID, &ticket.UserID, &ticket.GuildID, &ticket.Type,
		&ticket.Subject, &ticket.Description, &ticket.Status, &ticket.Priority,
		&ticket.ClaimedBy, &ticket.ClaimedAt, &formJSON, &ticket.CreatedAt,
		&ticket.ClosedAt, &ticket.ClosedBy, &ticket.Resolution)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ticket.FormData = make(map[string]string)
	json.Unmarshal([]byte(formJSON), &ticket.FormData)
	return ticket, nil
}

// GetOpenTicketsByUser retrieves all open tickets for a user
func (db *DB) GetOpenTicketsByUser(userID, guildID string) ([]*Ticket, error) {
	rows, err := db.conn.Query(
		`SELECT id, channel_id, user_id, guild_id, type, subject, status, created_at 
		FROM tickets WHERE user_id = ? AND guild_id = ? AND status NOT IN ('closed', 'approved', 'denied')`,
		userID, guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*Ticket
	for rows.Next() {
		ticket := &Ticket{}
		if err := rows.Scan(&ticket.ID, &ticket.ChannelID, &ticket.UserID, &ticket.GuildID,
			&ticket.Type, &ticket.Subject, &ticket.Status, &ticket.CreatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// GetUserTicketHistory retrieves all tickets for a user
func (db *DB) GetUserTicketHistory(userID, guildID string, limit int) ([]*Ticket, error) {
	rows, err := db.conn.Query(
		`SELECT id, COALESCE(ticket_number, id) as ticket_number, channel_id, user_id, guild_id, type, subject, status, resolution, created_at, closed_at 
		FROM tickets WHERE user_id = ? AND guild_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, guildID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*Ticket
	for rows.Next() {
		ticket := &Ticket{}
		if err := rows.Scan(&ticket.ID, &ticket.TicketNumber, &ticket.ChannelID, &ticket.UserID, &ticket.GuildID,
			&ticket.Type, &ticket.Subject, &ticket.Status, &ticket.Resolution, &ticket.CreatedAt, &ticket.ClosedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// SetTicketPriority updates the priority of a ticket
func (db *DB) SetTicketPriority(channelID, priority string) error {
	_, err := db.conn.Exec(
		"UPDATE tickets SET priority = ? WHERE channel_id = ?",
		priority, channelID,
	)
	return err
}

// CloseTicket marks a ticket as closed
func (db *DB) CloseTicket(channelID, closedBy string, resolution string) error {
	now := time.Now()
	_, err := db.conn.Exec(
		"UPDATE tickets SET status = 'closed', closed_at = ?, closed_by = ?, resolution = ? WHERE channel_id = ?",
		now, closedBy, resolution, channelID,
	)
	return err
}

// ReopenTicket reopens a closed ticket
func (db *DB) ReopenTicket(channelID string) error {
	_, err := db.conn.Exec(
		"UPDATE tickets SET status = 'open', closed_at = NULL, closed_by = NULL WHERE channel_id = ?",
		channelID,
	)
	return err
}

// ApproveTicket marks a ticket as approved (for appeals/applications) but keeps it open
func (db *DB) ApproveTicket(channelID, staffID string) error {
	_, err := db.conn.Exec(
		"UPDATE tickets SET status = 'approved', resolution = 'approved' WHERE channel_id = ?",
		channelID,
	)
	return err
}

// DenyTicket marks a ticket as denied (for appeals/applications) but keeps it open
func (db *DB) DenyTicket(channelID, staffID string) error {
	_, err := db.conn.Exec(
		"UPDATE tickets SET status = 'denied', resolution = 'denied' WHERE channel_id = ?",
		channelID,
	)
	return err
}

// GetOpenTickets retrieves all non-closed tickets for a guild
func (db *DB) GetOpenTickets(guildID string) ([]*Ticket, error) {
	rows, err := db.conn.Query(
		`SELECT id, COALESCE(ticket_number, id) as ticket_number, channel_id, user_id, type, subject, priority, created_at 
		FROM tickets WHERE guild_id = ? AND status NOT IN ('closed')
		ORDER BY created_at ASC`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*Ticket
	for rows.Next() {
		ticket := &Ticket{}
		if err := rows.Scan(&ticket.ID, &ticket.TicketNumber, &ticket.ChannelID, &ticket.UserID, &ticket.Type,
			&ticket.Subject, &ticket.Priority, &ticket.CreatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// AddToBlacklist adds a user to the blacklist
func (db *DB) AddToBlacklist(userID, guildID, reason, addedBy string, expiresAt *time.Time) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO blacklist (user_id, guild_id, reason, added_by, expires_at) VALUES (?, ?, ?, ?, ?)",
		userID, guildID, reason, addedBy, expiresAt,
	)
	return err
}

// RemoveFromBlacklist removes a user from the blacklist
func (db *DB) RemoveFromBlacklist(userID, guildID string) error {
	_, err := db.conn.Exec(
		"DELETE FROM blacklist WHERE user_id = ? AND guild_id = ?",
		userID, guildID,
	)
	return err
}

// IsBlacklisted checks if a user is blacklisted
func (db *DB) IsBlacklisted(userID, guildID string) (*Blacklist, error) {
	bl := &Blacklist{}
	err := db.conn.QueryRow(
		`SELECT id, user_id, guild_id, reason, added_by, created_at, expires_at 
		FROM blacklist WHERE user_id = ? AND guild_id = ? 
		AND (expires_at IS NULL OR expires_at > datetime('now'))`,
		userID, guildID,
	).Scan(&bl.ID, &bl.UserID, &bl.GuildID, &bl.Reason, &bl.AddedBy, &bl.CreatedAt, &bl.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return bl, nil
}

// SaveTranscript saves a ticket transcript
func (db *DB) SaveTranscript(ticketID int64, channelID, guildID, userID, content string) (*Transcript, error) {
	result, err := db.conn.Exec(
		"INSERT INTO transcripts (ticket_id, channel_id, guild_id, user_id, content) VALUES (?, ?, ?, ?, ?)",
		ticketID, channelID, guildID, userID, content,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Transcript{
		ID:        id,
		TicketID:  ticketID,
		ChannelID: channelID,
		GuildID:   guildID,
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
	}, nil
}

// GetTranscript retrieves a transcript by ticket ID
func (db *DB) GetTranscript(ticketID int64) (*Transcript, error) {
	tr := &Transcript{}
	err := db.conn.QueryRow(
		"SELECT id, ticket_id, channel_id, guild_id, user_id, content, created_at FROM transcripts WHERE ticket_id = ?",
		ticketID,
	).Scan(&tr.ID, &tr.TicketID, &tr.ChannelID, &tr.GuildID, &tr.UserID, &tr.Content, &tr.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return tr, nil
}

// AddStaffNote adds an internal note to a ticket
func (db *DB) AddStaffNote(ticketID int64, authorID, content string) (*StaffNote, error) {
	result, err := db.conn.Exec(
		"INSERT INTO staff_notes (ticket_id, author_id, content) VALUES (?, ?, ?)",
		ticketID, authorID, content,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &StaffNote{
		ID:        id,
		TicketID:  ticketID,
		AuthorID:  authorID,
		Content:   content,
		CreatedAt: time.Now(),
	}, nil
}

// GetStaffNotes retrieves all notes for a ticket
func (db *DB) GetStaffNotes(ticketID int64) ([]*StaffNote, error) {
	rows, err := db.conn.Query(
		"SELECT id, ticket_id, author_id, content, created_at FROM staff_notes WHERE ticket_id = ? ORDER BY created_at ASC",
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*StaffNote
	for rows.Next() {
		note := &StaffNote{}
		if err := rows.Scan(&note.ID, &note.TicketID, &note.AuthorID, &note.Content, &note.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// GetTicketStats returns ticket statistics for a guild
func (db *DB) GetTicketStats(guildID string) (total, open, closed int, err error) {
	err = db.conn.QueryRow(
		"SELECT COUNT(*) FROM tickets WHERE guild_id = ?", guildID,
	).Scan(&total)
	if err != nil {
		return
	}

	err = db.conn.QueryRow(
		"SELECT COUNT(*) FROM tickets WHERE guild_id = ? AND status NOT IN ('closed', 'approved', 'denied')", guildID,
	).Scan(&open)
	if err != nil {
		return
	}

	closed = total - open
	return
}

// GetDetailedStats returns detailed statistics by type
func (db *DB) GetDetailedStats(guildID string) (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := db.conn.Query(
		"SELECT type, COUNT(*) FROM tickets WHERE guild_id = ? GROUP BY type", guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ticketType string
		var count int
		if err := rows.Scan(&ticketType, &count); err != nil {
			return nil, err
		}
		stats[ticketType] = count
	}

	return stats, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}
