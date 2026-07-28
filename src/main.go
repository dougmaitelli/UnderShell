// Command sshrpg starts the SSH game server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sshrpg/src/config"
	"sshrpg/src/enemy"
	"sshrpg/src/item"
	"sshrpg/src/persistence"
	"sshrpg/src/quest"
	"sshrpg/src/repository"
	"sshrpg/src/sshserver"
	"sshrpg/src/ui"
	"sshrpg/src/world"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0700); err != nil {
		log.Error("create data directory", "error", err)
		os.Exit(1)
	}
	database, err := persistence.Open(cfg.DatabasePath)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	characters := repository.NewCharacterRepository(database.ORM())
	inventories := repository.NewInventoryRepository(database.ORM())
	shops := repository.NewShopRepository(database.ORM())
	questProgress := repository.NewQuestRepository(database.ORM())

	areas, err := world.LoadAreas(cfg.MapsPath)
	if err != nil {
		log.Error("load maps", "path", cfg.MapsPath, "error", err)
		os.Exit(1)
	}
	game, err := config.LoadGame(cfg.GamePath)
	if err != nil {
		log.Error("load game config", "path", cfg.GamePath, "error", err)
		os.Exit(1)
	}
	if err := areas.SetDefaultSpawn(
		game.DefaultSpawn.AreaID,
		world.Point{X: game.DefaultSpawn.X, Y: game.DefaultSpawn.Y},
	); err != nil {
		log.Error("validate default spawn", "error", err)
		os.Exit(1)
	}
	items, err := item.LoadItems(cfg.ItemsPath)
	if err != nil {
		log.Error("load items", "path", cfg.ItemsPath, "error", err)
		os.Exit(1)
	}
	enemies, err := enemy.LoadEnemies(cfg.EnemiesPath)
	if err != nil {
		log.Error("load enemies", "path", cfg.EnemiesPath, "error", err)
		os.Exit(1)
	}
	if err := areas.ValidateEnemySpawns(enemies); err != nil {
		log.Error("validate enemy spawns", "error", err)
		os.Exit(1)
	}
	if err := enemies.ValidateDrops(items); err != nil {
		log.Error("validate enemy drops", "error", err)
		os.Exit(1)
	}
	quests, err := quest.LoadQuests(cfg.QuestsPath)
	if err != nil {
		log.Error("load quests", "path", cfg.QuestsPath, "error", err)
		os.Exit(1)
	}
	if err := quests.ResolveItems(items); err != nil {
		log.Error("validate quest objectives", "error", err)
		os.Exit(1)
	}
	if err := quests.ValidateObjectives(enemies); err != nil {
		log.Error("validate quest enemy drops", "error", err)
		os.Exit(1)
	}
	if err := areas.ValidateNPCs(items, quests); err != nil {
		log.Error("validate NPCs", "error", err)
		os.Exit(1)
	}
	worldManager := world.New(areas, items, enemies, quests)
	defer worldManager.Close()

	runner := ui.New(ui.Repositories{
		Characters: characters, Inventories: inventories, Shops: shops,
		Quests: questProgress,
	}, worldManager, log)
	server, err := sshserver.New(cfg.ListenAddr, cfg.HostKeyPath, runner, log)
	if err != nil {
		log.Error("configure SSH server", "error", err)
		os.Exit(1)
	}

	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-signals:
		log.Info("shutting down", "signal", sig)
	case err := <-errs:
		if err != nil {
			log.Error("SSH server stopped", "error", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown", "error", err)
	}
}
