package fetcher

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"

	"email-demo/store"
)

// Config for IMAP connection.
type Config struct {
	Host          string
	Port          int
	Username      string
	Password      string
	TLS           bool
	Mailbox       string
	PollInterval  time.Duration
	AttachmentDir string
}

// Fetcher polls an IMAP mailbox and stores parsed emails.
type Fetcher struct {
	cfg   Config
	store *store.Store
	stop  chan struct{}
}

// New creates a new IMAP fetcher.
func New(cfg Config, st *store.Store) *Fetcher {
	return &Fetcher{cfg: cfg, store: st, stop: make(chan struct{})}
}

// Start begins polling in background.
func (f *Fetcher) Start() {
	go f.loop()
}

// Stop signals the fetcher to stop.
func (f *Fetcher) Stop() {
	close(f.stop)
}

func (f *Fetcher) loop() {
	// Fetch immediately on start
	f.fetchAll()

	ticker := time.NewTicker(f.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.fetchAll()
		case <-f.stop:
			log.Println("[IMAP] Fetcher stopped")
			return
		}
	}
}

func (f *Fetcher) connect() (*imapclient.Client, error) {
	addr := net.JoinHostPort(f.cfg.Host, fmt.Sprintf("%d", f.cfg.Port))

	var c *imapclient.Client
	var err error

	if f.cfg.TLS {
		c, err = imapclient.DialTLS(addr, &imapclient.Options{
			TLSConfig: &tls.Config{ServerName: f.cfg.Host},
		})
	} else {
		c, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("dial IMAP: %w", err)
	}

	if err := c.Login(f.cfg.Username, f.cfg.Password).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("login: %w", err)
	}

	return c, nil
}

func (f *Fetcher) fetchAll() {
	c, err := f.connect()
	if err != nil {
		log.Printf("[IMAP] Connection failed: %v", err)
		return
	}
	defer func() {
		c.Logout().Wait()
		c.Close()
	}()

	mbox, err := c.Select(f.cfg.Mailbox, nil).Wait()
	if err != nil {
		log.Printf("[IMAP] Select %s failed: %v", f.cfg.Mailbox, err)
		return
	}

	if mbox.NumMessages == 0 {
		log.Printf("[IMAP] %s is empty", f.cfg.Mailbox)
		return
	}

	log.Printf("[IMAP] %s has %d messages, fetching...", f.cfg.Mailbox, mbox.NumMessages)

	// Fetch all messages: envelope + full body
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(1, mbox.NumMessages)

	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		Envelope:    true,
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	fetchCmd := c.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	fetched := 0
	skipped := 0

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		// Collect all items from this message
		var envelope *imap.Envelope
		var bodyData []byte

		for {
			item := msg.Next()
			if item == nil {
				break
			}
			switch v := item.(type) {
			case imapclient.FetchItemDataEnvelope:
				envelope = v.Envelope
			case imapclient.FetchItemDataBodySection:
				bodyData, _ = io.ReadAll(v.Literal)
			}
		}

		if envelope == nil {
			continue
		}

		// Deduplicate by Message-ID
		messageID := envelope.MessageID
		if messageID != "" {
			exists, err := f.store.HasMessageID(messageID)
			if err == nil && exists {
				skipped++
				continue
			}
		}

		// Parse and store
		email := f.buildEmail(envelope, bodyData)
		emailID, err := f.store.SaveEmail(email)
		if err != nil {
			log.Printf("[IMAP] Failed to save email %q: %v", email.Subject, err)
			continue
		}

		// Parse body parts (text, html, attachments)
		f.parseBody(emailID, email, bodyData)
		fetched++
	}

	if err := fetchCmd.Close(); err != nil {
		log.Printf("[IMAP] Fetch error: %v", err)
	}

	log.Printf("[IMAP] Done: %d new, %d skipped", fetched, skipped)
}

func (f *Fetcher) buildEmail(env *imap.Envelope, _ []byte) *store.Email {
	e := &store.Email{
		MessageID: env.MessageID,
		Subject:   env.Subject,
		RecvDate:  env.Date,
	}

	if len(env.From) > 0 {
		e.FromAddr = formatAddr(env.From[0])
		e.FromName = env.From[0].Name
	}
	e.ToAddr = formatAddrs(env.To)
	e.CcAddr = formatAddrs(env.Cc)
	e.BccAddr = formatAddrs(env.Bcc)

	return e
}

func (f *Fetcher) parseBody(emailID int64, email *store.Email, bodyData []byte) {
	if len(bodyData) == 0 {
		return
	}

	// Store raw data
	email.RawData = string(bodyData)

	mr, err := mail.CreateReader(strings.NewReader(string(bodyData)))
	if err != nil {
		log.Printf("[IMAP] Failed to parse body for email %d: %v", emailID, err)
		return
	}

	var textBody, htmlBody string

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, err := io.ReadAll(p.Body)
			if err != nil {
				continue
			}
			if strings.HasPrefix(ct, "text/html") {
				htmlBody = string(b)
			} else {
				textBody = string(b)
			}

		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			if filename == "" {
				filename = fmt.Sprintf("attachment_%d_%d", emailID, time.Now().UnixNano())
			}
			// Sanitize filename
			filename = filepath.Base(filename)

			ct, _, _ := h.ContentType()

			// Save to disk
			dir := filepath.Join(f.cfg.AttachmentDir, fmt.Sprintf("%d", emailID))
			os.MkdirAll(dir, 0755)
			filePath := filepath.Join(dir, filename)

			data, err := io.ReadAll(p.Body)
			if err != nil {
				log.Printf("[IMAP] Failed to read attachment %s: %v", filename, err)
				continue
			}

			if err := os.WriteFile(filePath, data, 0644); err != nil {
				log.Printf("[IMAP] Failed to save attachment %s: %v", filePath, err)
				continue
			}

			f.store.SaveAttachment(&store.Attachment{
				EmailID:  emailID,
				Filename: filename,
				MimeType: ct,
				Size:     int64(len(data)),
				FilePath: filePath,
			})
		}
	}

	// Update email with parsed text/html bodies
	f.store.UpdateEmailBody(emailID, textBody, htmlBody, string(bodyData))
}

func formatAddr(a imap.Address) string {
	if a.Addr() != "" {
		return a.Addr()
	}
	return a.Mailbox + "@" + a.Host
}

func formatAddrs(addrs []imap.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, formatAddr(a))
	}
	return strings.Join(parts, ", ")
}

// init registers the mime type for common extensions
func init() {
	mime.AddExtensionType(".pdf", "application/pdf")
}
