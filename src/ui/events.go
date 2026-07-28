package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshrpg/src/world"
)

const eventLineLimit = 10
const eventTextWidth = 28

type EventView struct {
	Kind    world.EventKind
	Message string
}

type eventFeed struct {
	events []timedEvent
	nextID uint64
}

type timedEvent struct {
	id uint64
	EventView
}

const eventLifetime = 6 * time.Second

func (f *eventFeed) add(event EventView) tea.Cmd {
	f.nextID++
	id := f.nextID
	f.events = append(f.events, timedEvent{id: id, EventView: event})
	return tea.Tick(eventLifetime, func(time.Time) tea.Msg {
		return eventExpiredMsg{id: id}
	})
}

func (f *eventFeed) expire(id uint64) {
	for index, event := range f.events {
		if event.id == id {
			f.events = append(f.events[:index], f.events[index+1:]...)
			return
		}
	}
}

func (f *eventFeed) views() []EventView {
	events := make([]EventView, len(f.events))
	for index, event := range f.events {
		events[index] = event.EventView
	}
	return events
}

func (m *gameModel) addEvent(event EventView) tea.Cmd {
	return m.eventFeed.add(event)
}

func (m *gameModel) eventViews() []EventView {
	return m.eventFeed.views()
}

type EventRenderer struct{}

func (EventRenderer) RenderOver(game string, width, height int, events []EventView) string {
	if len(events) == 0 {
		return game
	}
	type styledLine struct {
		kind world.EventKind
		text string
	}
	lines := make([]styledLine, 0, len(events))
	for _, event := range events {
		wrapped := wrapEventText("• "+event.Message, eventTextWidth)
		for _, line := range wrapped {
			lines = append(lines, styledLine{kind: event.Kind, text: line})
		}
	}
	lineLimit := min(eventLineLimit, max(height-5, 1))
	if len(lines) > lineLimit {
		lines = lines[len(lines)-lineLimit:]
	}
	rendered := make([]string, len(lines))
	for index, line := range lines {
		rendered[index] = eventStyle(line.kind).Render(line.text)
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		eventTitleStyle.Render("EVENTS"),
		strings.Join(rendered, "\n"),
	)
	window := eventWindowStyle.Render(body)
	windowWidth, windowHeight := lipgloss.Size(window)
	x := max(width-windowWidth-1, 0)
	y := max(height-windowHeight-3, 1)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).X(x).Y(y).Z(1),
	).Render()
}

func wrapEventText(value string, width int) []string {
	if width < 1 {
		return nil
	}
	words := strings.Fields(value)
	lines := make([]string, 0, 2)
	current := ""
	for _, word := range words {
		for len([]rune(word)) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			runes := []rune(word)
			lines = append(lines, string(runes[:width]))
			word = string(runes[width:])
		}
		if current == "" {
			current = word
		} else if len([]rune(current))+1+len([]rune(word)) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func eventStyle(kind world.EventKind) lipgloss.Style {
	switch kind {
	case world.EventPickup:
		return pickupEventStyle
	case world.EventProgression:
		return progressionEventStyle
	case world.EventDamage, world.EventDeath:
		return damageEventStyle
	case world.EventCombat:
		return combatEventStyle
	case world.EventQuest:
		return questEventStyle
	default:
		return respawnEventStyle
	}
}

var (
	eventTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0"))
	eventWindowStyle = lipgloss.NewStyle().
				Width(eventTextWidth).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#475569"))
	pickupEventStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	progressionEventStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC"))
	damageEventStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185"))
	combatEventStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	questEventStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE047"))
	respawnEventStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
)
