package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"email-demo/api"
	"email-demo/fetcher"
	"email-demo/store"
)

func main() {
	cfg := LoadConfig()

	// Init store — create parent directory of DB file
	os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)
	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer st.Close()

	// Init attachment dir
	os.MkdirAll(cfg.AttachmentDir, 0755)

	// Start IMAP fetcher
	f := fetcher.New(fetcher.Config{
		Host:          cfg.IMAP.Host,
		Port:          cfg.IMAP.Port,
		Username:      cfg.IMAP.Username,
		Password:      cfg.IMAP.Password,
		TLS:           cfg.IMAP.TLS,
		Mailbox:       cfg.IMAP.Mailbox,
		PollInterval:  cfg.PollInterval,
		AttachmentDir: cfg.AttachmentDir,
	}, st)
	f.Start()
	log.Printf("IMAP fetcher started (polling every %s)", cfg.PollInterval)

	// Start HTTP server
	srv := api.NewServer(cfg.HTTPAddr, st)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	f.Stop()
	srv.Shutdown()
	log.Println("Bye.")
}
