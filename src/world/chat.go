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
		Type:     ChatMessagePlayer,
		PlayerID: player.ID, PlayerName: player.Name,
		PlayerRole: player.Role, Message: message,
	}
	s.broadcast(chat, players)
	return true
}

func (s *chatSystem) sendServer(
	players map[int64]*activePlayer,
	value string,
) bool {
	message, ok := validateChatMessage(value)
	if !ok {
		return false
	}
	s.broadcast(ChatMessage{
		Type: ChatMessageServer, PlayerName: "Server", Message: message,
	}, players)
	return true
}

func (s *chatSystem) broadcast(
	chat ChatMessage,
	players map[int64]*activePlayer,
) {
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
