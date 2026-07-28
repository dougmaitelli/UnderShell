package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/ssh"

	"sshrpg/src/domain"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

type Identity struct {
	Fingerprint string
	KeyType     string
	PublicKey   string
}

type Repositories struct {
	Characters  repository.CharacterRepository
	Inventories repository.InventoryRepository
	Shops       repository.ShopRepository
}

type Runner struct {
	repositories Repositories
	world        *world.Manager
	log          *slog.Logger
}

func New(
	repositories Repositories,
	worldManager *world.Manager,
	log *slog.Logger,
) *Runner {
	return &Runner{
		repositories: repositories,
		world:        worldManager, log: log,
	}
}

func (r *Runner) Run(session ssh.Session, identity Identity) {
	pty, resize, ok := session.Pty()
	if !ok {
		_, _ = io.WriteString(session, "An interactive terminal is required. Try: ssh -t <host>\n")
		return
	}

	char, err := r.repositories.Characters.FindByFingerprint(
		session.Context(), identity.Fingerprint,
	)
	if err != nil {
		r.log.Error("load character", "error", err)
		_, _ = io.WriteString(session, "The game could not load your character. Please try again.\n")
		return
	}
	var inventory *domain.Inventory
	if char != nil {
		inventory, err = r.repositories.Inventories.FindOrCreate(session.Context(), char.ID)
		if err != nil {
			r.log.Error("load inventory", "character_id", char.ID, "error", err)
			_, _ = io.WriteString(session, "The game could not load your inventory. Please try again.\n")
			return
		}
	}

	model := newGameModel(r.repositories, r.world, r.log, identity, char, inventory)
	program := tea.NewProgram(
		model,
		tea.WithContext(session.Context()),
		tea.WithInput(session),
		tea.WithOutput(session),
		tea.WithEnvironment(append(session.Environ(), "TERM="+pty.Term)),
		tea.WithColorProfile(colorprofile.TrueColor),
		tea.WithoutSignalHandler(),
	)

	go forwardWindowSizes(program, resize, pty.Window)
	finalModel, runErr := program.Run()
	if final, ok := finalModel.(*gameModel); ok {
		final.leaveWorld()
	}
	if runErr != nil &&
		!errors.Is(runErr, context.Canceled) &&
		!errors.Is(runErr, tea.ErrProgramKilled) &&
		!errors.Is(runErr, tea.ErrInterrupted) {
		r.log.Error("terminal program failed", "error", runErr)
	}
}

func forwardWindowSizes(program *tea.Program, resize <-chan ssh.Window, initial ssh.Window) {
	program.Send(tea.WindowSizeMsg{Width: initial.Width, Height: initial.Height})
	for window := range resize {
		program.Send(tea.WindowSizeMsg{Width: window.Width, Height: window.Height})
	}
}
