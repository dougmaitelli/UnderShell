package sshserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"

	"sshrpg/src/ui"
)

type identityKey struct{}

type Server struct {
	server *ssh.Server
	log    *slog.Logger
}

func New(addr, hostKeyPath string, runner *ui.Runner, log *slog.Logger) (*Server, error) {
	signer, err := loadOrCreateHostKey(hostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("host key: %w", err)
	}

	s := &ssh.Server{
		Addr:        addr,
		IdleTimeout: 30 * time.Minute,
		MaxTimeout:  12 * time.Hour,
		Version:     "SSH-RPG",
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			identity := ui.Identity{
				Fingerprint: gossh.FingerprintSHA256(key),
				KeyType:     key.Type(),
				PublicKey:   base64.StdEncoding.EncodeToString(key.Marshal()),
			}
			ctx.SetValue(identityKey{}, identity)
			return true
		},
		Handler: func(session ssh.Session) {
			value := session.Context().Value(identityKey{})
			identity, ok := value.(ui.Identity)
			if !ok {
				_, _ = session.Write([]byte("Public-key authentication is required.\n"))
				return
			}
			runner.Run(session, identity)
			_, _ = session.Write([]byte("\x1b[?25h\x1b[?1049l"))
		},
		LocalPortForwardingCallback:   func(_ ssh.Context, _ string, _ uint32) bool { return false },
		ReversePortForwardingCallback: func(_ ssh.Context, _ string, _ uint32) bool { return false },
	}
	s.AddHostKey(signer)
	return &Server{server: s, log: log}, nil
}

func (s *Server) ListenAndServe() error {
	s.log.Info("SSH RPG listening", "addr", s.server.Addr)
	err := s.server.ListenAndServe()
	if errors.Is(err, ssh.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func loadOrCreateHostKey(path string) (gossh.Signer, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return gossh.ParsePrivateKey(data)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	data = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}
	return gossh.NewSignerFromKey(privateKey)
}
