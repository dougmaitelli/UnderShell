// Package config loads runtime configuration from the environment.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type Config struct {
	ListenAddr   string
	GamePath     string
	HostKeyPath  string
	DatabasePath string
	MapsPath     string
	ItemsPath    string
	EnemiesPath  string
	QuestsPath   string
}

func Load() Config {
	return Config{
		ListenAddr:   env("SSH_LISTEN_ADDR", ":2222"),
		GamePath:     env("GAME_CONFIG_PATH", "./config/game.json"),
		HostKeyPath:  env("SSH_HOST_KEY_PATH", "./data/ssh_host_ed25519"),
		DatabasePath: env("DATABASE_PATH", "./data/game.db"),
		MapsPath:     env("MAPS_PATH", "./maps"),
		ItemsPath:    env("ITEMS_PATH", "./items/items.json"),
		EnemiesPath:  env("ENEMIES_PATH", "./enemies/enemies.json"),
		QuestsPath:   env("QUESTS_PATH", "./quests/quests.json"),
	}
}

type Spawn struct {
	AreaID string `json:"area_id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

type Game struct {
	DefaultSpawn Spawn `json:"default_spawn"`
}

func LoadGame(path string) (Game, error) {
	file, err := os.Open(path)
	if err != nil {
		return Game{}, fmt.Errorf("open game config %s: %w", path, err)
	}
	defer file.Close()

	var game Game
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&game); err != nil {
		return Game{}, fmt.Errorf("decode game config %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return Game{}, fmt.Errorf("decode game config %s: %w", path, err)
	}
	if game.DefaultSpawn.AreaID == "" {
		return Game{}, errors.New("default_spawn.area_id is required")
	}
	return game, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
