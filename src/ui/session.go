package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

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
const registrationRateLimitMessage = "Too many new-player registrations from this address. Please try again later."
const terminalRenderFPS = 20
const initialDatabaseOperationTimeout = 5 * time.Second

type Repositories struct {
	Characters  repository.CharacterRepository
	Inventories repository.InventoryRepository
	Shops       repository.ShopRepository
	Quests      repository.QuestRepository
}

type Runner struct {
	repositories  Repositories
	world         *world.Manager
	admin         *admin.Handler
	log           *slog.Logger
	registrations RegistrationLimiter
}

type RegistrationLimiter interface {
	AllowRegistration(string) bool
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

func (r *Runner) SetRegistrationLimiter(limiter RegistrationLimiter) {
	r.registrations = limiter
}

func (r *Runner) Run(session ssh.Session, identity Identity) {
	pty, resize, ok := session.Pty()
	if !ok {
		_, _ = io.WriteString(session, "An interactive terminal is required. Try: ssh -t <host>\n")
		return
	}

	loadContext, cancelLoad := context.WithTimeout(
		session.Context(), initialDatabaseOperationTimeout,
	)
	char, err := r.repositories.Characters.FindByFingerprint(
		loadContext, identity.Fingerprint,
	)
	cancelLoad()
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
	if char == nil && r.registrations != nil &&
		!r.registrations.AllowRegistration(sessionRemoteIP(session.RemoteAddr())) {
		_, _ = io.WriteString(session, registrationRateLimitMessage+"\n")
		return
	}
	var inventory *domain.Inventory
	var quests []domain.CharacterQuest
	if char != nil {
		loadContext, cancelLoad = context.WithTimeout(
			session.Context(), initialDatabaseOperationTimeout,
		)
		inventory, err = r.repositories.Inventories.FindOrCreate(loadContext, char.ID)
		cancelLoad()
		if err != nil {
			r.log.Error("load inventory", "character_id", char.ID, "error", err)
			_, _ = io.WriteString(session, "The game could not load your inventory. Please try again.\n")
			return
		}
		loadContext, cancelLoad = context.WithTimeout(
			session.Context(), initialDatabaseOperationTimeout,
		)
		quests, err = r.repositories.Quests.FindByCharacter(loadContext, char.ID)
		cancelLoad()
		if err != nil {
			r.log.Error("load quests", "character_id", char.ID, "error", err)
			_, _ = io.WriteString(session, "The game could not load your quests. Please try again.\n")
			return
		}
	}

	model := newGameModel(
		r.repositories, r.world, r.log, identity, char, inventory,
		session.Context(),
	)
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

func sessionRemoteIP(address net.Addr) string {
	if address == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(address.String())
	if err == nil {
		return host
	}
	return address.String()
}

func forwardWindowSizes(program *tea.Program, resize <-chan ssh.Window, initial ssh.Window) {
	program.Send(tea.WindowSizeMsg{Width: initial.Width, Height: initial.Height})
	for window := range resize {
		program.Send(tea.WindowSizeMsg{Width: window.Width, Height: window.Height})
	}
}
