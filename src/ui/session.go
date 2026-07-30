package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/ssh"

	"sshrpg/src/admin"
	"sshrpg/src/domain"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

type Identity struct {
	Fingerprint string
	KeyType     string
	PublicKey   string
}

const bannedAccountMessage = "This account has been permanently banned."
const maintenanceModeMessage = "The server is currently in maintenance mode. Please try again later."
const terminalRenderFPS = 20

type Repositories struct {
	Characters  repository.CharacterRepository
	Inventories repository.InventoryRepository
	Shops       repository.ShopRepository
	Quests      repository.QuestRepository
}

type Runner struct {
	repositories Repositories
	world        *world.Manager
	admin        *admin.Handler
	log          *slog.Logger
}

func New(
	repositories Repositories,
	worldManager *world.Manager,
	adminHandler *admin.Handler,
	log *slog.Logger,
) *Runner {
	return &Runner{
		repositories: repositories,
		world:        worldManager, admin: adminHandler, log: log,
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
	if char != nil && char.Banned {
		_, _ = io.WriteString(session, bannedAccountMessage+"\n")
		return
	}
	if r.admin != nil && !r.admin.AllowsConnection(char) {
		_, _ = io.WriteString(session, maintenanceModeMessage+"\n")
		return
	}
	var inventory *domain.Inventory
	var quests []domain.CharacterQuest
	if char != nil {
		inventory, err = r.repositories.Inventories.FindOrCreate(session.Context(), char.ID)
		if err != nil {
			r.log.Error("load inventory", "character_id", char.ID, "error", err)
			_, _ = io.WriteString(session, "The game could not load your inventory. Please try again.\n")
			return
		}
		quests, err = r.repositories.Quests.FindByCharacter(session.Context(), char.ID)
		if err != nil {
			r.log.Error("load quests", "character_id", char.ID, "error", err)
			_, _ = io.WriteString(session, "The game could not load your quests. Please try again.\n")
			return
		}
	}

	model := newGameModel(r.repositories, r.world, r.log, identity, char, inventory)
	model.admin = r.admin
	model.quests.setProgress(quests)
	program := tea.NewProgram(
		model,
		tea.WithContext(session.Context()),
		tea.WithInput(session),
		tea.WithOutput(session),
		tea.WithEnvironment(append(session.Environ(), "TERM="+pty.Term)),
		tea.WithColorProfile(colorprofile.TrueColor),
		tea.WithFPS(terminalRenderFPS),
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
