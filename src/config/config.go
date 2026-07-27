// Package config loads runtime configuration from the environment.
package config

import (
	"os"
)

type Config struct {
	ListenAddr   string
	HostKeyPath  string
	DatabasePath string
	MapsPath     string
	ItemsPath    string
	EnemiesPath  string
}

func Load() Config {
	return Config{
		ListenAddr:   env("SSH_LISTEN_ADDR", ":2222"),
		HostKeyPath:  env("SSH_HOST_KEY_PATH", "./data/ssh_host_ed25519"),
		DatabasePath: env("DATABASE_PATH", "./data/game.db"),
		MapsPath:     env("MAPS_PATH", "./maps"),
		ItemsPath:    env("ITEMS_PATH", "./items/items.json"),
		EnemiesPath:  env("ENEMIES_PATH", "./enemies/enemies.json"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
