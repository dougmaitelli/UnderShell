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
	HostKeyPath  string
	DatabaseURL  string
	DatabasePath string
	GamePath     string
	AreasPath    string
	ObjectsPath  string
	ItemsPath    string
	EnemiesPath  string
	QuestsPath   string
}

func Load() Config {
	return Config{
		ListenAddr:   env("SSH_LISTEN_ADDR", ":2222"),
		HostKeyPath:  env("SSH_HOST_KEY_PATH", "./data/ssh_host_ed25519"),
		DatabaseURL:  env("DATABASE_URL"),
		DatabasePath: env("DATABASE_PATH", "./data/game.db"),
		GamePath:     env("GAME_CONFIG_PATH", "./content/game.json"),
		AreasPath:    env("AREAS_PATH", "./content/areas"),
		ObjectsPath:  env("OBJECTS_PATH", "./content/objects"),
		ItemsPath:    env("ITEMS_PATH", "./content/items"),
		EnemiesPath:  env("ENEMIES_PATH", "./content/enemies"),
		QuestsPath:   env("QUESTS_PATH", "./content/quests"),
	}
}

// DatabaseSource returns the PostgreSQL URL when configured, otherwise the
// local SQLite path. DATABASE_URL intentionally takes precedence so production
// deployments cannot accidentally open the fallback file.
func (c Config) DatabaseSource() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return c.DatabasePath
}

type Spawn struct {
	AreaID string `json:"area_id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

type Game struct {
	DefaultSpawn Spawn `json:"default_spawn"`
}

func LoadGame(path string) (game Game, err error) {
	file, err := os.Open(path)
	if err != nil {
		return Game{}, fmt.Errorf("open game config %s: %w", path, err)
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

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

func env(key string, fallback ...string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
