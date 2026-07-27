// Package config loads runtime configuration from the environment.
package config

import (
	"os"
	"strconv"
)

type Config struct {
	ListenAddr   string
	HostKeyPath  string
	DatabasePath string
	WorldWidth   int
	WorldHeight  int
}

func Load() Config {
	return Config{
		ListenAddr:   env("SSH_LISTEN_ADDR", ":2222"),
		HostKeyPath:  env("SSH_HOST_KEY_PATH", "./data/ssh_host_ed25519"),
		DatabasePath: env("DATABASE_PATH", "./data/game.db"),
		WorldWidth:   envInt("WORLD_WIDTH", 120),
		WorldHeight:  envInt("WORLD_HEIGHT", 60),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
