// Command sshrpg starts the SSH game server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sshrpg/src/admin"
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
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	database, err := persistence.Open(cfg.DatabaseSource())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Error("close database", "error", err)
		}
	}()
	characters := repository.NewCharacterRepository(database.ORM())
	inventories := repository.NewInventoryRepository(database.ORM())
	shops := repository.NewShopRepository(database.ORM())
	questProgress := repository.NewQuestRepository(database.ORM())

	game, err := config.LoadGame(cfg.GamePath)
	if err != nil {
		return fmt.Errorf("load game config %s: %w", cfg.GamePath, err)
	}
	items, err := item.LoadItems(cfg.ItemsPath)
	if err != nil {
		return fmt.Errorf("load items %s: %w", cfg.ItemsPath, err)
	}
	enemies, err := enemy.LoadEnemies(cfg.EnemiesPath, items)
	if err != nil {
		return fmt.Errorf("load enemies %s: %w", cfg.EnemiesPath, err)
	}
	quests, err := quest.LoadQuests(cfg.QuestsPath, items)
	if err != nil {
		return fmt.Errorf("load quests %s: %w", cfg.QuestsPath, err)
	}
	if err := quests.ValidateObjectives(enemies); err != nil {
		return fmt.Errorf("validate quest enemy drops: %w", err)
	}
	objects, err := world.LoadMapObjects(cfg.ObjectsPath)
	if err != nil {
		return fmt.Errorf("load map objects %s: %w", cfg.ObjectsPath, err)
	}
	areas, err := world.LoadAreas(
		cfg.AreasPath, world.References{
			Items: items, Enemies: enemies, Quests: quests, Objects: objects,
		},
	)
	if err != nil {
		return fmt.Errorf("load areas %s: %w", cfg.AreasPath, err)
	}
	if err := areas.SetDefaultSpawn(
		game.DefaultSpawn.AreaID,
		world.Point{X: game.DefaultSpawn.X, Y: game.DefaultSpawn.Y},
	); err != nil {
		return fmt.Errorf("validate default spawn: %w", err)
	}
	worldManager := world.New(areas, items, enemies, quests)
	defer worldManager.Close()
	adminCommands := admin.New(characters, inventories, items, worldManager)

	runner := ui.New(ui.Repositories{
		Characters: characters, Inventories: inventories, Shops: shops,
		Quests: questProgress,
	}, worldManager, adminCommands, log)
	server, err := sshserver.New(
		cfg.ListenAddr, cfg.HostKeyPath, runner, log,
		sshserver.AdmissionConfig{
			MaxConnections:       cfg.SSHMaxConnections,
			MaxConnectionsPerIP:  cfg.SSHMaxConnectionsPerIP,
			MaxSessions:          cfg.SSHMaxSessions,
			HandshakesPerMinute:  cfg.SSHHandshakesPerMinute,
			RegistrationsPerHour: cfg.SSHRegistrationsPerHour,
		},
	)
	if err != nil {
		return fmt.Errorf("configure SSH server: %w", err)
	}

	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	go func() {
		if err := adminCommands.RunConsole(context.Background(), os.Stdin, os.Stdout); err != nil {
			log.Error("admin console stopped", "error", err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		log.Info("shutting down", "signal", sig)
	case err := <-errs:
		if err != nil {
			return fmt.Errorf("serve SSH: %w", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful SSH shutdown: %w", err)
	}
	return nil
}
