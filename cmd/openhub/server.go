package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jeremytregunna/openhub/internal/auth"
	"github.com/jeremytregunna/openhub/internal/config"
	"github.com/jeremytregunna/openhub/internal/git"
	"github.com/jeremytregunna/openhub/internal/instance"
	"github.com/jeremytregunna/openhub/internal/replication"
	"github.com/jeremytregunna/openhub/internal/server"
	"github.com/jeremytregunna/openhub/internal/storage"
	"golang.org/x/crypto/ssh"
)

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	sshPort := fs.Int("ssh-port", 2222, "SSH server port")
	httpPort := fs.Int("http-port", 3000, "HTTP server port")
	fs.Parse(args)

	cfg := config.Default()
	cfg.SSHPort = *sshPort
	cfg.HTTPPort = *httpPort

	if storagePath := os.Getenv("OPENHUB_STORAGE"); storagePath != "" {
		cfg.StoragePath = storagePath
	}

	store, err := storage.New(cfg.StoragePath)
	if err != nil {
		log.Fatalf("storage init: %v", err)
	}

	authStore, err := auth.NewAuthStore(cfg.StoragePath)
	if err != nil {
		log.Fatalf("auth store init: %v", err)
	}

	inst, err := instance.LoadOrCreate(cfg.StoragePath)
	if err != nil {
		log.Fatalf("instance init: %v", err)
	}
	log.Printf("instance ID: %s", inst.ID)

	replManager := replication.NewManager(store, inst.ID)
	replManager.Start(3)
	log.Printf("started replication workers")
	replManager.StartPeriodicSync(5 * time.Minute)
	log.Printf("started periodic sync (every 5 minutes)")

	hostKey, err := loadOrGenerateHostKey(cfg.StoragePath)
	if err != nil {
		log.Fatalf("load host key: %v", err)
	}

	sshServer := git.NewSSHServer(cfg.SSHPort, store, authStore, hostKey, replManager)
	go func() {
		log.Printf("starting SSH server on port %d", cfg.SSHPort)
		if err := sshServer.Start(); err != nil {
			log.Fatalf("SSH server: %v", err)
		}
	}()

	apiServer := server.New(store, authStore)
	gitHTTPServer := git.NewHTTPServer(store, authStore)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer)
	mux.Handle("/", gitHTTPServer)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	log.Printf("starting HTTP server on port %d", cfg.HTTPPort)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("HTTP server: %v", err)
	}
}

func loadOrGenerateHostKey(storagePath string) (ssh.Signer, error) {
	keyPath := filepath.Join(storagePath, "ssh_host_key")

	// Try to load existing key
	if keyData, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(keyData)
		if block == nil {
			return nil, fmt.Errorf("invalid PEM data in host key")
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
		return ssh.NewSignerFromKey(key)
	}

	// Generate new key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	// Save key to disk
	keyData := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		return nil, fmt.Errorf("write host key: %w", err)
	}

	log.Printf("generated new SSH host key at %s", keyPath)
	return ssh.NewSignerFromKey(key)
}
