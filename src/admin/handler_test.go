package admin

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/persistence"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

func TestChatCommandsRequireAdminAndDefaultTargetToSender(t *testing.T) {
	handler, manager, characters, _, adminPlayer, userPlayer := testHandler(t)

	adminSession := manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	userSession := manager.Join(world.Player{
		ID: userPlayer.ID, Name: userPlayer.Name, Role: domain.CharacterRoleUser,
		AreaID: "meadow", X: 2, Y: 1, Level: 1,
	})

	if _, err := handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		"/lvl 2",
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("non-admin command error = %v", err)
	}

	message, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		"/lvl 2",
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Granted 2 level(s) to Admin Player." {
		t.Fatalf("command response = %q", message)
	}
	persisted, err := characters.FindByFingerprint(
		context.Background(), "SHA256:admin-command",
	)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Level != 3 || persisted.SkillPoints != 2 {
		t.Fatalf("persisted admin progress = %#v", persisted)
	}
}

func TestConsoleCommandsRequireTargetsAndMutateNamedPlayers(t *testing.T) {
	handler, manager, characters, inventories, adminPlayer, userPlayer := testHandler(t)
	manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	manager.Join(world.Player{
		ID: userPlayer.ID, Name: userPlayer.Name, Role: domain.CharacterRoleUser,
		AreaID: "meadow", X: 2, Y: 1, Level: 1,
	})

	if _, err := handler.ExecuteConsole(
		context.Background(), "/exp 125",
	); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("missing console target error = %v", err)
	}
	if _, err := handler.ExecuteConsole(
		context.Background(), `/exp 125 "Target Player"`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handler.ExecuteConsole(
		context.Background(), `/item "Slime Gel" 5 "Target Player"`,
	); err != nil {
		t.Fatal(err)
	}
	inventory, err := inventories.FindOrCreate(context.Background(), userPlayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Items) != 3 ||
		inventory.Items[0].Quantity != 2 ||
		inventory.Items[1].Quantity != 2 ||
		inventory.Items[2].Quantity != 1 {
		t.Fatalf("granted inventory = %#v", inventory.Items)
	}

	if _, err := handler.ExecuteConsole(
		context.Background(), `/tp "Crystal Cave" "Target Player"`,
	); err != nil {
		t.Fatal(err)
	}
	target, err := manager.FindOnlinePlayer("target player")
	if err != nil {
		t.Fatal(err)
	}
	if target.AreaID != "cave" || target.X != 3 || target.Y != 1 {
		t.Fatalf("area teleport target = %#v", target)
	}
	if _, err := handler.ExecuteConsole(
		context.Background(), `/tpTo "Admin Player" "Target Player"`,
	); err != nil {
		t.Fatal(err)
	}
	target, err = manager.FindOnlinePlayer("Target Player")
	if err != nil {
		t.Fatal(err)
	}
	if target.AreaID != "meadow" || target.X != 1 || target.Y != 1 {
		t.Fatalf("player teleport target = %#v", target)
	}

	persisted, err := characters.FindByFingerprint(
		context.Background(), "SHA256:user-command",
	)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Level != 2 || persisted.Experience != 25 ||
		persisted.SkillPoints != 1 ||
		persisted.AreaID != "meadow" || persisted.X != 1 || persisted.Y != 1 {
		t.Fatalf("persisted target = %#v", persisted)
	}
}

func TestOnlyAdminsPromoteAndModeratorsCanRunOtherCommands(t *testing.T) {
	handler, manager, characters, _, adminPlayer, userPlayer := testHandler(t)
	adminSession := manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	userSession := manager.Join(world.Player{
		ID: userPlayer.ID, Name: userPlayer.Name, Role: domain.CharacterRoleUser,
		AreaID: "meadow", X: 2, Y: 1, Level: 1,
	})

	message, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		`/promote "Target Player"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Promoted Target Player to moderator." {
		t.Fatalf("promotion response = %q", message)
	}
	promoted, err := manager.FindOnlinePlayer("Target Player")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Role != domain.CharacterRoleModerator {
		t.Fatalf("live promoted role = %q", promoted.Role)
	}
	persisted, err := characters.FindByFingerprint(
		context.Background(), "SHA256:user-command",
	)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Role != domain.CharacterRoleModerator {
		t.Fatalf("persisted promoted role = %q", persisted.Role)
	}

	if _, err := handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		"/lvl 1",
	); err != nil {
		t.Fatalf("moderator command failed: %v", err)
	}
	message, err = handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		"/m Maintenance begins soon",
	)
	if err != nil {
		t.Fatalf("moderator server message failed: %v", err)
	}
	if message != "Server message sent." {
		t.Fatalf("server message response = %q", message)
	}
	for _, session := range []world.Session{adminSession, userSession} {
		announcement := <-session.Chats
		if announcement.Type != world.ChatMessageServer ||
			announcement.PlayerName != "Server" ||
			announcement.Message != "Maintenance begins soon" {
			t.Fatalf("server announcement = %#v", announcement)
		}
	}
	if _, err := handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		`/promote "Admin Player"`,
	); !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("moderator promotion error = %v", err)
	}
	for _, command := range []string{
		`/ban "Admin Player"`,
		`/unban "Admin Player"`,
	} {
		if _, err := handler.ExecuteChat(
			context.Background(),
			userPlayer.ID,
			userSession.Token,
			userPlayer.Name,
			command,
		); !errors.Is(err, ErrAdminRequired) {
			t.Fatalf("moderator command %q error = %v", command, err)
		}
	}
	if _, err := handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		`/kick "Admin Player"`,
	); err == nil || !strings.Contains(err.Error(), "cannot kick administrators") {
		t.Fatalf("moderator kicked admin: %v", err)
	}
	message, err = handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		`/kick "Target Player"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Kicked Target Player." {
		t.Fatalf("kick response = %q", message)
	}
	select {
	case reason := <-userSession.Kicked:
		if reason != "You were kicked from the game." {
			t.Fatalf("kick reason = %q", reason)
		}
	default:
		t.Fatal("kicked moderator did not receive a disconnect reason")
	}
	if _, err := manager.FindOnlinePlayer("Target Player"); !errors.Is(
		err, world.ErrPlayerNotOnline,
	) {
		t.Fatalf("kicked player remained online: %v", err)
	}
}

func TestAdminBanDisconnectsPersistsAndUnbanWorksOffline(t *testing.T) {
	handler, manager, characters, _, adminPlayer, userPlayer := testHandler(t)
	adminSession := manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	userSession := manager.Join(world.Player{
		ID: userPlayer.ID, Name: userPlayer.Name, Role: domain.CharacterRoleUser,
		AreaID: "meadow", X: 2, Y: 1, Level: 1,
	})

	message, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		`/ban "Target Player"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Banned Target Player." {
		t.Fatalf("ban response = %q", message)
	}
	select {
	case reason := <-userSession.Kicked:
		if reason != "This account has been permanently banned." {
			t.Fatalf("ban reason = %q", reason)
		}
	default:
		t.Fatal("banned player did not receive a disconnect reason")
	}
	persisted, err := characters.FindByFingerprint(
		context.Background(), "SHA256:user-command",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.Banned {
		t.Fatal("ban was not persisted")
	}

	message, err = handler.ExecuteConsole(
		context.Background(), `/unban "target player"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Unbanned Target Player." {
		t.Fatalf("unban response = %q", message)
	}
	persisted, err = characters.FindByFingerprint(
		context.Background(), "SHA256:user-command",
	)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Banned {
		t.Fatal("unban was not persisted")
	}
}

func TestAdminCanPromoteModeratorToAdministrator(t *testing.T) {
	handler, manager, characters, _, adminPlayer, userPlayer := testHandler(t)
	adminSession := manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	manager.Join(world.Player{
		ID: userPlayer.ID, Name: userPlayer.Name, Role: domain.CharacterRoleUser,
		AreaID: "meadow", X: 2, Y: 1, Level: 1,
	})

	if _, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		`/promote "Target Player"`,
	); err != nil {
		t.Fatal(err)
	}
	message, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		`/promote "Target Player" admin`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Promoted Target Player to admin." {
		t.Fatalf("admin promotion response = %q", message)
	}
	promoted, err := manager.FindOnlinePlayer("Target Player")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Role != domain.CharacterRoleAdmin {
		t.Fatalf("live promoted role = %q", promoted.Role)
	}
	persisted, err := characters.FindByFingerprint(
		context.Background(), "SHA256:user-command",
	)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Role != domain.CharacterRoleAdmin {
		t.Fatalf("persisted promoted role = %q", persisted.Role)
	}

	if _, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		`/promote "Target Player" owner`,
	); err == nil || !strings.Contains(err.Error(), "moderator or admin") {
		t.Fatalf("invalid promotion role error = %v", err)
	}
}

func TestMaintenanceModeAllowsOnlyStaffConnections(t *testing.T) {
	handler, manager, _, _, adminPlayer, userPlayer := testHandler(t)
	adminSession := manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	userSession := manager.Join(world.Player{
		ID: userPlayer.ID, Name: userPlayer.Name, Role: domain.CharacterRoleUser,
		AreaID: "meadow", X: 2, Y: 1, Level: 1,
	})

	if _, err := handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		"/maintenance on",
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("user maintenance command error = %v", err)
	}
	if _, err := handler.ExecuteChat(
		context.Background(),
		adminPlayer.ID,
		adminSession.Token,
		adminPlayer.Name,
		`/promote "Target Player"`,
	); err != nil {
		t.Fatal(err)
	}
	message, err := handler.ExecuteChat(
		context.Background(),
		userPlayer.ID,
		userSession.Token,
		userPlayer.Name,
		"/maintenance on",
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Maintenance mode enabled." || !handler.MaintenanceEnabled() {
		t.Fatalf(
			"maintenance response = %q, enabled = %v",
			message, handler.MaintenanceEnabled(),
		)
	}
	for _, session := range []world.Session{adminSession, userSession} {
		announcement := <-session.Chats
		if announcement.Type != world.ChatMessageServer ||
			announcement.Message !=
				"Maintenance mode enabled. New player connections are paused." {
			t.Fatalf("maintenance announcement = %#v", announcement)
		}
	}
	if handler.AllowsConnection(nil) {
		t.Fatal("maintenance allowed character creation")
	}
	if handler.AllowsConnection(&domain.Character{
		Role: domain.CharacterRoleUser,
	}) {
		t.Fatal("maintenance allowed a user connection")
	}
	for _, role := range []domain.CharacterRole{
		domain.CharacterRoleModerator,
		domain.CharacterRoleAdmin,
	} {
		if !handler.AllowsConnection(&domain.Character{Role: role}) {
			t.Fatalf("maintenance rejected %s connection", role)
		}
	}

	message, err = handler.ExecuteConsole(
		context.Background(), "/maintenance off",
	)
	if err != nil {
		t.Fatal(err)
	}
	if message != "Maintenance mode disabled." ||
		handler.MaintenanceEnabled() ||
		!handler.AllowsConnection(nil) {
		t.Fatalf(
			"disabled maintenance response = %q, enabled = %v",
			message, handler.MaintenanceEnabled(),
		)
	}
}

func TestConsoleReportsEachLineAndRequiresSlashPrefix(t *testing.T) {
	handler, manager, _, _, adminPlayer, _ := testHandler(t)
	manager.Join(world.Player{
		ID: adminPlayer.ID, Name: adminPlayer.Name, Role: domain.CharacterRoleAdmin,
		AreaID: "meadow", X: 1, Y: 1, Level: 1,
	})
	var output strings.Builder
	err := handler.RunConsole(
		context.Background(),
		strings.NewReader(
			"lvl 1 \"Admin Player\"\n"+
				"/lvl 1 \"Admin Player\"\n"+
				"/m Welcome to the realm\n",
		),
		&output,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "commands must begin with /") ||
		!strings.Contains(output.String(), "Granted 1 level(s) to Admin Player.") ||
		!strings.Contains(output.String(), "Server message sent.") {
		t.Fatalf("console output = %q", output.String())
	}
}

func testHandler(t *testing.T) (
	*Handler,
	*world.Manager,
	repository.CharacterRepository,
	repository.InventoryRepository,
	*domain.Character,
	*domain.Character,
) {
	t.Helper()
	database, err := persistence.Open(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	characters := repository.NewCharacterRepository(database.ORM())
	inventories := repository.NewInventoryRepository(database.ORM())
	ctx := context.Background()
	adminPlayer, err := characters.Create(ctx, repository.CreateCharacterParams{
		KeyFingerprint: "SHA256:admin-command",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "admin-key",
		Name:           "Admin Player",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := characters.UpdateRole(
		ctx, adminPlayer.ID, domain.CharacterRoleAdmin,
	); err != nil {
		t.Fatal(err)
	}
	adminPlayer.Role = domain.CharacterRoleAdmin
	userPlayer, err := characters.Create(ctx, repository.CreateCharacterParams{
		KeyFingerprint: "SHA256:user-command",
		PublicKeyType:  "ssh-ed25519",
		PublicKey:      "user-key",
		Name:           "Target Player",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inventories.FindOrCreate(ctx, adminPlayer.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := inventories.FindOrCreate(ctx, userPlayer.ID); err != nil {
		t.Fatal(err)
	}

	items, err := item.NewItems([]item.Definition{{
		ID: "slime_gel", Name: "Slime Gel",
		Type: item.TypeMaterial, MaxStack: 2,
	}})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := world.NewAreas([]world.AreaDefinition{
		{
			ID: "meadow", Name: "Meadow",
			Layout: []string{"#####", "#...#", "#####"},
			Spawn:  world.Point{X: 1, Y: 1},
		},
		{
			ID: "cave", Name: "Crystal Cave",
			Layout: []string{"#####", "#...#", "#####"},
			Spawn:  world.Point{X: 3, Y: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := world.New(areas, items, nil, nil)
	t.Cleanup(manager.Close)
	return New(characters, inventories, items, manager),
		manager, characters, inventories, adminPlayer, userPlayer
}
