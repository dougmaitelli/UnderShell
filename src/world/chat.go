package world

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type chatSystem struct {
	history []ChatMessage
}

func newChatSystem() chatSystem {
	return chatSystem{history: make([]ChatMessage, 0, chatHistoryLimit)}
}

func (s *chatSystem) send(
	player *activePlayer,
	players map[int64]*activePlayer,
	value string,
) bool {
	message, ok := validateChatMessage(value)
	if player == nil || !ok {
		return false
	}
	chat := ChatMessage{
		PlayerID: player.ID, PlayerName: player.Name, Message: message,
	}
	s.history = append(s.history, chat)
	if len(s.history) > chatHistoryLimit {
		s.history = s.history[len(s.history)-chatHistoryLimit:]
	}
	for _, recipient := range players {
		select {
		case recipient.chats <- chat:
		default:
		}
	}
	return true
}

func validateChatMessage(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > chatMessageLimit {
		return "", false
	}
	for _, character := range value {
		if unicode.IsControl(character) || !unicode.IsPrint(character) {
			return "", false
		}
	}
	return value, true
}
