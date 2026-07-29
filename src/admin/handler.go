// Package admin parses and executes privileged server commands.
package admin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

var (
	ErrPermissionDenied = errors.New("moderator or administrator role required")
	ErrAdminRequired    = errors.New("administrator role required for this command")
	ErrSlashRequired    = errors.New("commands must begin with /")
)

type Handler struct {
	characters  repository.CharacterRepository
	inventories repository.InventoryRepository
	items       *item.Items
	world       *world.Manager
	maintenance atomic.Bool
}

func New(
	characters repository.CharacterRepository,
	inventories repository.InventoryRepository,
	items *item.Items,
	worldManager *world.Manager,
) *Handler {
	return &Handler{
		characters: characters, inventories: inventories,
		items: items, world: worldManager,
	}
}

func (h *Handler) ExecuteChat(
	ctx context.Context,
	playerID int64,
	token string,
	playerName string,
	input string,
) (string, error) {
	role, ok := h.world.AuthenticatedRole(playerID, token)
	if !ok || (role != domain.CharacterRoleModerator &&
		role != domain.CharacterRoleAdmin) {
		return "", ErrPermissionDenied
	}
	return h.execute(
		ctx, input, strings.TrimSpace(playerName), false, role,
	)
}

func (h *Handler) ExecuteConsole(ctx context.Context, input string) (string, error) {
	return h.execute(ctx, input, "", true, domain.CharacterRoleAdmin)
}

func (h *Handler) execute(
	ctx context.Context,
	input string,
	defaultTarget string,
	console bool,
	role domain.CharacterRole,
) (string, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", ErrSlashRequired
	}
	fields, err := splitArguments(strings.TrimSpace(strings.TrimPrefix(input, "/")))
	if err != nil {
		return "", err
	}
	if len(fields) == 0 {
		return "", errors.New("command name is required")
	}

	command := strings.ToLower(fields[0])
	args := fields[1:]
	switch command {
	case "exp":
		return h.giveExperience(ctx, args, defaultTarget, console)
	case "lvl":
		return h.giveLevels(ctx, args, defaultTarget, console)
	case "item":
		return h.giveItem(ctx, args, defaultTarget, console)
	case "tp":
		return h.teleportArea(ctx, args, defaultTarget, console)
	case "tpto":
		return h.teleportPlayer(ctx, args, defaultTarget, console)
	case "promote":
		if role != domain.CharacterRoleAdmin {
			return "", ErrAdminRequired
		}
		return h.promote(ctx, args)
	case "kick":
		return h.kick(args, role)
	case "ban":
		if role != domain.CharacterRoleAdmin {
			return "", ErrAdminRequired
		}
		return h.ban(ctx, args)
	case "unban":
		if role != domain.CharacterRoleAdmin {
			return "", ErrAdminRequired
		}
		return h.unban(ctx, args)
	case "m":
		return h.serverMessage(args)
	case "maintenance":
		return h.setMaintenance(args)
	default:
		return "", fmt.Errorf("unknown command %q", fields[0])
	}
}

func (h *Handler) MaintenanceEnabled() bool {
	return h.maintenance.Load()
}

func (h *Handler) AllowsConnection(character *domain.Character) bool {
	if !h.MaintenanceEnabled() {
		return true
	}
	if character == nil {
		return false
	}
	return character.Role == domain.CharacterRoleModerator ||
		character.Role == domain.CharacterRoleAdmin
}

func (h *Handler) setMaintenance(args []string) (string, error) {
	if len(args) != 1 {
		return "", errors.New("usage: /maintenance <on|off>")
	}
	switch strings.ToLower(args[0]) {
	case "on":
		changed := !h.maintenance.Swap(true)
		if changed {
			h.world.ServerMessage(
				"Maintenance mode enabled. New player connections are paused.",
			)
		}
		return "Maintenance mode enabled.", nil
	case "off":
		changed := h.maintenance.Swap(false)
		if changed {
			h.world.ServerMessage(
				"Maintenance mode disabled. Player connections are open.",
			)
		}
		return "Maintenance mode disabled.", nil
	default:
		return "", errors.New("usage: /maintenance <on|off>")
	}
}

func (h *Handler) serverMessage(args []string) (string, error) {
	message := strings.TrimSpace(strings.Join(args, " "))
	if message == "" {
		return "", errors.New("usage: /m <message>")
	}
	if !h.world.ServerMessage(message) {
		return "", errors.New("server message must be printable and at most 200 characters")
	}
	return "Server message sent.", nil
}

func (h *Handler) kick(
	args []string,
	role domain.CharacterRole,
) (string, error) {
	target, err := requiredPlayerArgument(args, "/kick <player>")
	if err != nil {
		return "", err
	}
	player, err := h.world.FindOnlinePlayer(target)
	if err != nil {
		return "", playerError(target, err)
	}
	if role == domain.CharacterRoleModerator &&
		player.Role == domain.CharacterRoleAdmin {
		return "", errors.New("moderators cannot kick administrators")
	}
	kicked, err := h.world.KickPlayer(
		player.Name, "You were kicked from the game.",
	)
	if err != nil {
		return "", playerError(target, err)
	}
	return fmt.Sprintf("Kicked %s.", kicked.Name), nil
}

func (h *Handler) ban(ctx context.Context, args []string) (string, error) {
	target, err := requiredPlayerArgument(args, "/ban <player>")
	if err != nil {
		return "", err
	}
	character, err := h.characters.SetBanned(ctx, target, true)
	if errors.Is(err, repository.ErrCharacterNotFound) {
		return "", fmt.Errorf("player %q does not exist", target)
	}
	if err != nil {
		return "", fmt.Errorf("ban player: %w", err)
	}
	_, kickErr := h.world.KickPlayer(
		character.Name, "This account has been permanently banned.",
	)
	if kickErr != nil && !errors.Is(kickErr, world.ErrPlayerNotOnline) {
		return "", fmt.Errorf("disconnect banned player: %w", kickErr)
	}
	return fmt.Sprintf("Banned %s.", character.Name), nil
}

func (h *Handler) unban(ctx context.Context, args []string) (string, error) {
	target, err := requiredPlayerArgument(args, "/unban <player>")
	if err != nil {
		return "", err
	}
	character, err := h.characters.SetBanned(ctx, target, false)
	if errors.Is(err, repository.ErrCharacterNotFound) {
		return "", fmt.Errorf("player %q does not exist", target)
	}
	if err != nil {
		return "", fmt.Errorf("unban player: %w", err)
	}
	return fmt.Sprintf("Unbanned %s.", character.Name), nil
}

func requiredPlayerArgument(args []string, usage string) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf("usage: %s", usage)
	}
	return strings.TrimSpace(args[0]), nil
}

func (h *Handler) promote(ctx context.Context, args []string) (string, error) {
	target, err := requiredPlayerArgument(args, "/promote <player>")
	if err != nil {
		return "", err
	}
	player, err := h.world.FindOnlinePlayer(target)
	if err != nil {
		return "", playerError(target, err)
	}
	switch player.Role {
	case domain.CharacterRoleAdmin:
		return "", fmt.Errorf("%s is already an administrator", player.Name)
	case domain.CharacterRoleModerator:
		return "", fmt.Errorf("%s is already a moderator", player.Name)
	}
	if err := h.characters.UpdateRole(
		ctx, player.ID, domain.CharacterRoleModerator,
	); err != nil {
		return "", fmt.Errorf("persist moderator role: %w", err)
	}
	promoted, err := h.world.SetPlayerRole(
		player.Name, domain.CharacterRoleModerator,
	)
	if err != nil {
		return "", playerError(target, err)
	}
	return fmt.Sprintf("Promoted %s to moderator.", promoted.Name), nil
}

func (h *Handler) giveExperience(
	ctx context.Context,
	args []string,
	defaultTarget string,
	console bool,
) (string, error) {
	target, err := commandTarget(args, 1, defaultTarget, console,
		"/exp <amount> [player]")
	if err != nil {
		return "", err
	}
	amount, err := positiveInt64(args[0], "experience")
	if err != nil {
		return "", err
	}
	player, err := h.world.GrantExperience(target, amount)
	if err != nil {
		return "", playerError(target, err)
	}
	if err := h.saveProgress(ctx, player); err != nil {
		return "", err
	}
	return fmt.Sprintf("Granted %d XP to %s.", amount, player.Name), nil
}

func (h *Handler) giveLevels(
	ctx context.Context,
	args []string,
	defaultTarget string,
	console bool,
) (string, error) {
	target, err := commandTarget(args, 1, defaultTarget, console,
		"/lvl <amount> [player]")
	if err != nil {
		return "", err
	}
	amount, err := positiveInt(args[0], "levels")
	if err != nil {
		return "", err
	}
	player, err := h.world.GrantLevels(target, amount)
	if err != nil {
		return "", playerError(target, err)
	}
	if err := h.saveProgress(ctx, player); err != nil {
		return "", err
	}
	return fmt.Sprintf("Granted %d level(s) to %s.", amount, player.Name), nil
}

func (h *Handler) giveItem(
	ctx context.Context,
	args []string,
	defaultTarget string,
	console bool,
) (string, error) {
	target, err := commandTarget(args, 2, defaultTarget, console,
		"/item <item> <quantity> [player]")
	if err != nil {
		return "", err
	}
	definition, ok := h.findItem(args[0])
	if !ok {
		return "", fmt.Errorf("item %q not found", args[0])
	}
	quantity, err := positiveInt(args[1], "quantity")
	if err != nil {
		return "", err
	}
	player, err := h.world.FindOnlinePlayer(target)
	if err != nil {
		return "", playerError(target, err)
	}
	if _, err := h.inventories.AddItems(
		ctx, player.ID, definition.ID, definition.MaxStack, quantity,
	); err != nil {
		return "", fmt.Errorf("give item: %w", err)
	}
	h.world.NotifyPlayer(
		player.ID,
		fmt.Sprintf("Received %d × %s", quantity, definition.Name),
		true,
	)
	return fmt.Sprintf(
		"Granted %d × %s to %s.", quantity, definition.Name, player.Name,
	), nil
}

func (h *Handler) teleportArea(
	ctx context.Context,
	args []string,
	defaultTarget string,
	console bool,
) (string, error) {
	target, err := commandTarget(args, 1, defaultTarget, console,
		"/tp <area> [player]")
	if err != nil {
		return "", err
	}
	player, err := h.world.TeleportToArea(target, args[0])
	if err != nil {
		return "", playerError(target, err)
	}
	if err := h.characters.UpdateLocation(
		ctx, player.ID, player.AreaID, player.X, player.Y,
	); err != nil {
		return "", fmt.Errorf("save teleport: %w", err)
	}
	return fmt.Sprintf("Teleported %s to %s.", player.Name, args[0]), nil
}

func (h *Handler) teleportPlayer(
	ctx context.Context,
	args []string,
	defaultTarget string,
	console bool,
) (string, error) {
	target, err := commandTarget(args, 1, defaultTarget, console,
		"/tpTo <destination-player> [player]")
	if err != nil {
		return "", err
	}
	player, err := h.world.TeleportToPlayer(target, args[0])
	if err != nil {
		return "", playerError(target, err)
	}
	if err := h.characters.UpdateLocation(
		ctx, player.ID, player.AreaID, player.X, player.Y,
	); err != nil {
		return "", fmt.Errorf("save teleport: %w", err)
	}
	return fmt.Sprintf("Teleported %s to %s.", player.Name, args[0]), nil
}

func (h *Handler) saveProgress(ctx context.Context, player world.Player) error {
	if err := h.characters.UpdateProgress(
		ctx, player.ID, player.Level, player.Experience, player.SkillPoints,
		player.Attack, player.Defense, player.Vitality,
	); err != nil {
		return fmt.Errorf("save granted progress: %w", err)
	}
	return nil
}

func (h *Handler) findItem(value string) (*item.Definition, bool) {
	if definition, ok := h.items.Item(value); ok {
		return definition, true
	}
	for _, definition := range h.items.All() {
		if strings.EqualFold(definition.Name, value) {
			resolved, _ := h.items.Item(definition.ID)
			return resolved, true
		}
	}
	return nil, false
}

func commandTarget(
	args []string,
	required int,
	defaultTarget string,
	console bool,
	usage string,
) (string, error) {
	switch {
	case !console && len(args) == required:
		return defaultTarget, nil
	case len(args) == required+1:
		return strings.TrimSpace(args[required]), nil
	default:
		return "", fmt.Errorf("usage: %s", usage)
	}
}

func positiveInt64(value, label string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return parsed, nil
}

func positiveInt(value, label string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return parsed, nil
}

func playerError(name string, err error) error {
	if errors.Is(err, world.ErrPlayerNotOnline) {
		return fmt.Errorf("player %q is not online", name)
	}
	return err
}
