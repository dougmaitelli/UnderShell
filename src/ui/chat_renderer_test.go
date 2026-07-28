package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"sshrpg/src/domain"
	"sshrpg/src/world"
)

func TestChatOverlayShowsLatestTenMessages(t *testing.T) {
	messages := make([]world.ChatMessage, 12)
	for index := range messages {
		messages[index] = world.ChatMessage{
			PlayerName: "Aria", Message: fmt.Sprintf("message-%02d", index),
		}
	}
	game := strings.Repeat(strings.Repeat(" ", 80)+"\n", 20)
	plain := ansi.Strip(ChatRenderer{}.RenderOver(game, 80, 24, messages, false, ""))
	if strings.Contains(plain, "message-00") || strings.Contains(plain, "message-01") {
		t.Fatalf("old chat messages remain visible: %q", plain)
	}
	for index := 2; index < 12; index++ {
		if !strings.Contains(plain, fmt.Sprintf("message-%02d", index)) {
			t.Fatalf("recent chat message %d is missing: %q", index, plain)
		}
	}
}

func TestFocusedChatShowsInputWithoutMessages(t *testing.T) {
	game := strings.Repeat(strings.Repeat(" ", 80)+"\n", 20)
	plain := ansi.Strip(ChatRenderer{}.RenderOver(game, 80, 24, nil, true, "hello"))
	if !strings.Contains(plain, "CHAT") || !strings.Contains(plain, "> hello") {
		t.Fatalf("focused chat input is missing: %q", plain)
	}
}

func TestChatNamesUsePlayerRoleColors(t *testing.T) {
	user := world.ChatMessage{
		PlayerName: "Aria",
		PlayerRole: domain.CharacterRoleUser,
		Message:    "hello",
	}
	if rendered := renderChatMessage(user, 0); !strings.Contains(
		rendered,
		renderPlayerName("Aria", domain.CharacterRoleUser, 0),
	) {
		t.Fatalf("user chat name does not use the shared player color: %q", rendered)
	}

	admin := world.ChatMessage{
		PlayerName: "DougM",
		PlayerRole: domain.CharacterRoleAdmin,
		Message:    "hello",
	}
	first := renderChatMessage(admin, 0)
	second := renderChatMessage(admin, 1)
	if ansi.Strip(first) != "[DougM] hello" {
		t.Fatalf("admin chat message = %q", ansi.Strip(first))
	}
	if first == second {
		t.Fatal("admin chat name shimmer did not advance")
	}
}
