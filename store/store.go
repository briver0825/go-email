package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Email represents a parsed email stored in the database.
type Email struct {
	ID        int64
	MessageID string
	Subject   string
	FromAddr  string
	FromName  string
	ToAddr    string
	CcAddr    string
	BccAddr   string
	TextBody  string
	HTMLBody  string
	RawData   string
	RecvDate  time.Time
	CreatedAt time.Time
}

// Attachment represents a saved email attachment.
type Attachment struct {
	ID        int64
	EmailID   int64
	Filename  string
	MimeType  string
	Size      int64
	FilePath  string
	CreatedAt time.Time
}

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// New opens the database and creates tables.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS emails (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT UNIQUE,
		subject    TEXT NOT NULL DEFAULT '',
		from_addr  TEXT NOT NULL DEFAULT '',
		from_name  TEXT NOT NULL DEFAULT '',
		to_addr    TEXT NOT NULL DEFAULT '',
		cc_addr    TEXT NOT NULL DEFAULT '',
		bcc_addr   TEXT NOT NULL DEFAULT '',
		text_body  TEXT NOT NULL DEFAULT '',
		html_body  TEXT NOT NULL DEFAULT '',
		raw_data   TEXT NOT NULL DEFAULT '',
		recv_date  DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_emails_message_id ON emails(message_id);
	CREATE INDEX IF NOT EXISTS idx_emails_recv_date ON emails(recv_date);

	CREATE TABLE IF NOT EXISTS attachments (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		email_id   INTEGER NOT NULL,
		filename   TEXT NOT NULL DEFAULT '',
		mime_type  TEXT NOT NULL DEFAULT '',
		size       INTEGER NOT NULL DEFAULT 0,
		file_path  TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (email_id) REFERENCES emails(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_attachments_email_id ON attachments(email_id);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// HasMessageID checks if an email with the given Message-ID already exists.
func (s *Store) HasMessageID(messageID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM emails WHERE message_id = ?", messageID).Scan(&count)
	return count > 0, err
}

// SaveEmail inserts a parsed email and returns its ID.
func (s *Store) SaveEmail(e *Email) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO emails (message_id, subject, from_addr, from_name, to_addr, cc_addr, bcc_addr, text_body, html_body, raw_data, recv_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.MessageID, e.Subject, e.FromAddr, e.FromName, e.ToAddr, e.CcAddr, e.BccAddr,
		e.TextBody, e.HTMLBody, e.RawData, e.RecvDate,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SaveAttachment inserts an attachment record.
func (s *Store) SaveAttachment(a *Attachment) error {
	_, err := s.db.Exec(`
		INSERT INTO attachments (email_id, filename, mime_type, size, file_path)
		VALUES (?, ?, ?, ?, ?)`,
		a.EmailID, a.Filename, a.MimeType, a.Size, a.FilePath,
	)
	return err
}

// GetEmails returns emails ordered by date descending with pagination.
func (s *Store) GetEmails(limit, offset int) ([]Email, error) {
	rows, err := s.db.Query(`
		SELECT id, message_id, subject, from_addr, from_name, to_addr, cc_addr, bcc_addr,
		       text_body, html_body, recv_date, created_at
		FROM emails ORDER BY recv_date DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []Email
	for rows.Next() {
		var e Email
		if err := rows.Scan(&e.ID, &e.MessageID, &e.Subject, &e.FromAddr, &e.FromName,
			&e.ToAddr, &e.CcAddr, &e.BccAddr, &e.TextBody, &e.HTMLBody,
			&e.RecvDate, &e.CreatedAt); err != nil {
			return nil, err
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// GetEmail returns a single email by ID.
func (s *Store) GetEmail(id int64) (*Email, error) {
	var e Email
	err := s.db.QueryRow(`
		SELECT id, message_id, subject, from_addr, from_name, to_addr, cc_addr, bcc_addr,
		       text_body, html_body, raw_data, recv_date, created_at
		FROM emails WHERE id = ?`, id).Scan(
		&e.ID, &e.MessageID, &e.Subject, &e.FromAddr, &e.FromName,
		&e.ToAddr, &e.CcAddr, &e.BccAddr, &e.TextBody, &e.HTMLBody,
		&e.RawData, &e.RecvDate, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetAttachments returns all attachments for an email.
func (s *Store) GetAttachments(emailID int64) ([]Attachment, error) {
	rows, err := s.db.Query(`
		SELECT id, email_id, filename, mime_type, size, file_path, created_at
		FROM attachments WHERE email_id = ?`, emailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.EmailID, &a.Filename, &a.MimeType, &a.Size, &a.FilePath, &a.CreatedAt); err != nil {
			return nil, err
		}
		atts = append(atts, a)
	}
	return atts, rows.Err()
}

// GetAttachment returns a single attachment by ID.
func (s *Store) GetAttachment(id int64) (Attachment, error) {
	var a Attachment
	err := s.db.QueryRow(`
		SELECT id, email_id, filename, mime_type, size, file_path, created_at
		FROM attachments WHERE id = ?`, id).Scan(
		&a.ID, &a.EmailID, &a.Filename, &a.MimeType, &a.Size, &a.FilePath, &a.CreatedAt,
	)
	return a, err
}

// UpdateEmailBody updates the text/html bodies and raw data after parsing.
func (s *Store) UpdateEmailBody(id int64, textBody, htmlBody, rawData string) error {
	_, err := s.db.Exec(
		"UPDATE emails SET text_body = ?, html_body = ?, raw_data = ? WHERE id = ?",
		textBody, htmlBody, rawData, id,
	)
	return err
}

// CountEmails returns total number of emails.
func (s *Store) CountEmails() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM emails").Scan(&count)
	return count, err
}
