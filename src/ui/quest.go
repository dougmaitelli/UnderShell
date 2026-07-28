package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
	npcconfig "sshrpg/src/npc"
	questconfig "sshrpg/src/quest"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

type questState struct {
	definitions     *questconfig.Quests
	progress        map[string]domain.CharacterQuest
	inFlight        bool
	dialogue        questDialogueState
	journalSelected int
}

type QuestView struct {
	Name        string
	Description string
	ItemName    string
	Current     int
	Required    int
	GiverID     string
	GiverName   string
	GiverArea   string
	RewardGold  int
}

type JournalView struct {
	Quests   []QuestView
	Selected int
}

type questDialogueKind uint8

const (
	questDialogueOffer questDialogueKind = iota
	questDialogueProgress
	questDialogueReady
	questDialogueCompleted
)

type questDialogueState struct {
	open       bool
	kind       questDialogueKind
	definition questconfig.Definition
	giverID    string
	giverName  string
}

type QuestDialogueView struct {
	NPCName     string
	Text        string
	CanAccept   bool
	CanComplete bool
}

type questInteractionKind uint8

const (
	questAccepted questInteractionKind = iota
	questCompleted
)

type questInteractionMsg struct {
	kind       questInteractionKind
	definition questconfig.Definition
	progress   domain.CharacterQuest
	completion repository.QuestCompletion
	err        error
}

func newQuestState(definitions *questconfig.Quests) questState {
	return questState{
		definitions: definitions,
		progress:    make(map[string]domain.CharacterQuest),
	}
}

func (s *questState) setProgress(progress []domain.CharacterQuest) {
	clear(s.progress)
	for _, entry := range progress {
		s.progress[entry.QuestID] = entry
	}
}

func (s *questState) views(inventory *domain.Inventory) []QuestView {
	if s.definitions == nil {
		return nil
	}
	views := make([]QuestView, 0, len(s.progress))
	for _, definition := range s.definitions.All() {
		progress, ok := s.progress[definition.ID]
		if !ok || progress.Status != domain.QuestActive {
			continue
		}
		itemName := definition.Objective.ItemID
		if definition.Objective.Item != nil {
			itemName = definition.Objective.Item.Name
		}
		views = append(views, QuestView{
			Name: definition.Name, Description: definition.Description,
			ItemName: itemName,
			Current: min(
				inventoryQuantity(inventory, definition.Objective.ItemID),
				definition.Objective.Quantity,
			),
			Required:   definition.Objective.Quantity,
			GiverID:    progress.GiverID,
			RewardGold: definition.Reward.Gold,
		})
	}
	return views
}

func (s *questState) journalView(
	inventory *domain.Inventory,
	resolveGiver func(string) (string, string),
) JournalView {
	quests := s.views(inventory)
	for index := range quests {
		quests[index].GiverName, quests[index].GiverArea =
			resolveGiver(quests[index].GiverID)
	}
	if len(quests) == 0 {
		s.journalSelected = 0
	} else if s.journalSelected >= len(quests) {
		s.journalSelected = len(quests) - 1
	}
	return JournalView{Quests: quests, Selected: s.journalSelected}
}

func (m *gameModel) questGiver(giverID string) (string, string) {
	if area := m.connection.snapshot.Area; area != nil {
		for _, definition := range area.NPCs {
			if definition.ID == giverID {
				return definition.Name, area.Name
			}
		}
	}
	if m.world != nil {
		if definition, area, ok := m.world.NPC(giverID); ok {
			return definition.Name, area.Name
		}
	}
	return giverID, ""
}

func inventoryQuantity(inventory *domain.Inventory, itemID string) int {
	if inventory == nil {
		return 0
	}
	quantity := 0
	for _, stack := range inventory.Items {
		if stack.ItemKey == itemID {
			quantity += stack.Quantity
		}
	}
	return quantity
}

func (m *gameModel) updateJournalInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "j", "esc":
		m.mode = inputModeGame
	case "up", "w":
		count := len(m.quests.views(m.inventory))
		if count > 0 {
			m.quests.journalSelected =
				(m.quests.journalSelected - 1 + count) % count
		}
	case "down", "s":
		count := len(m.quests.views(m.inventory))
		if count > 0 {
			m.quests.journalSelected =
				(m.quests.journalSelected + 1) % count
		}
	}
	return m, nil
}

func (s *questState) openDialogue(
	kind questDialogueKind,
	definition questconfig.Definition,
	giver *npcconfig.Definition,
) {
	s.dialogue = questDialogueState{
		open: true, kind: kind, definition: definition,
		giverID: giver.ID, giverName: giver.Name,
	}
}

func (s *questState) closeDialogue() {
	s.dialogue = questDialogueState{}
}

func (s *questState) dialogueView() QuestDialogueView {
	if !s.dialogue.open {
		return QuestDialogueView{}
	}
	dialogue := s.dialogue
	text := dialogue.definition.Dialogue.Offer
	switch dialogue.kind {
	case questDialogueProgress:
		text = dialogue.definition.Dialogue.InProgress
	case questDialogueReady:
		text = dialogue.definition.Dialogue.Ready
	case questDialogueCompleted:
		text = dialogue.definition.Dialogue.Completed
	}
	return QuestDialogueView{
		NPCName:     dialogue.giverName,
		Text:        text,
		CanAccept:   dialogue.kind == questDialogueOffer,
		CanComplete: dialogue.kind == questDialogueReady,
	}
}

func (m *gameModel) openQuestDialogue(
	kind questDialogueKind,
	definition questconfig.Definition,
	giver *npcconfig.Definition,
) (tea.Model, tea.Cmd) {
	m.quests.openDialogue(kind, definition, giver)
	m.mode = inputModeQuestDialogue
	m.movement.stop()
	return m, nil
}

func (m *gameModel) updateQuestDialogueInput(
	msg tea.KeyPressMsg,
) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if key == "esc" {
		m.quests.closeDialogue()
		m.mode = inputModeGame
		return m, nil
	}
	if key != "e" && key != "space" {
		return m, nil
	}

	dialogue := m.quests.dialogue
	m.quests.closeDialogue()
	m.mode = inputModeGame
	switch dialogue.kind {
	case questDialogueOffer:
		m.quests.inFlight = true
		return m, m.acceptQuest(
			dialogue.definition, dialogue.giverID,
		)
	case questDialogueReady:
		m.quests.inFlight = true
		return m, m.completeQuest(dialogue.definition)
	default:
		return m, nil
	}
}

func (m *gameModel) interactQuestGiver(
	giver *npcconfig.Definition,
) (tea.Model, tea.Cmd) {
	if m.quests.inFlight || m.quests.definitions == nil {
		return m, nil
	}
	for _, questID := range giver.QuestIDs {
		progress, tracked := m.quests.progress[questID]
		if !tracked || progress.Status != domain.QuestActive ||
			progress.GiverID != giver.ID {
			continue
		}
		definition, ok := m.quests.definitions.Quest(questID)
		if !ok {
			continue
		}
		current := inventoryQuantity(m.inventory, definition.Objective.ItemID)
		if current < definition.Objective.Quantity {
			return m.openQuestDialogue(
				questDialogueProgress, definition, giver,
			)
		}
		return m.openQuestDialogue(
			questDialogueReady, definition, giver,
		)
	}
	for _, questID := range giver.QuestIDs {
		if _, tracked := m.quests.progress[questID]; tracked {
			continue
		}
		definition, ok := m.quests.definitions.Quest(questID)
		if !ok {
			continue
		}
		return m.openQuestDialogue(
			questDialogueOffer, definition, giver,
		)
	}
	for _, questID := range giver.QuestIDs {
		progress, tracked := m.quests.progress[questID]
		if !tracked || progress.Status != domain.QuestCompleted ||
			progress.GiverID != giver.ID {
			continue
		}
		definition, ok := m.quests.definitions.Quest(questID)
		if !ok {
			continue
		}
		return m.openQuestDialogue(
			questDialogueCompleted, definition, giver,
		)
	}
	return m, m.addEvent(EventView{
		Kind:    world.EventQuest,
		Message: giver.Name + " has no quests available.",
	})
}

func (m *gameModel) acceptQuest(
	definition questconfig.Definition,
	giverID string,
) tea.Cmd {
	characterID := m.character.ID
	return func() tea.Msg {
		progress, err := m.repositories.Quests.Accept(context.Background(), repository.AcceptQuestParams{
			CharacterID: characterID, QuestID: definition.ID,
			GiverID: giverID,
		})
		return questInteractionMsg{
			kind: questAccepted, definition: definition,
			progress: progress, err: err,
		}
	}
}

func (m *gameModel) completeQuest(definition questconfig.Definition) tea.Cmd {
	characterID := m.character.ID
	return func() tea.Msg {
		completion, err := m.repositories.Quests.Complete(
			context.Background(), characterID, definition,
		)
		return questInteractionMsg{
			kind: questCompleted, definition: definition,
			completion: completion, err: err,
		}
	}
}

func (m *gameModel) updateQuestInteraction(
	msg questInteractionMsg,
) (tea.Model, tea.Cmd) {
	m.quests.inFlight = false
	if msg.err != nil {
		if errors.Is(msg.err, repository.ErrQuestItemsIncomplete) {
			return m, m.addEvent(EventView{
				Kind:    world.EventQuest,
				Message: "The required items are no longer available.",
			})
		}
		m.log.Error(
			"quest interaction",
			"character_id", m.character.ID,
			"quest_id", msg.definition.ID,
			"error", msg.err,
		)
		return m, m.addEvent(EventView{
			Kind: world.EventQuest, Message: "The quest could not be updated.",
		})
	}

	switch msg.kind {
	case questAccepted:
		m.quests.progress[msg.progress.QuestID] = msg.progress
		if msg.progress.Status == domain.QuestCompleted {
			return m, m.addEvent(EventView{
				Kind:    world.EventQuest,
				Message: "Quest already completed: " + msg.definition.Name,
			})
		}
		return m, m.addEvent(EventView{
			Kind:    world.EventQuest,
			Message: "Quest accepted: " + msg.definition.Name,
		})
	case questCompleted:
		m.quests.progress[msg.completion.Quest.QuestID] = msg.completion.Quest
		m.inventory = msg.completion.Inventory
		m.character.Gold = msg.completion.Gold
		message := "Quest completed: " + msg.definition.Name
		if msg.definition.Reward.Gold > 0 {
			message += fmt.Sprintf(" (+%d gold)", msg.definition.Reward.Gold)
		}
		return m, m.addEvent(EventView{
			Kind: world.EventQuest, Message: message,
		})
	default:
		return m, nil
	}
}

type QuestDialogueRenderer struct{}

func (QuestDialogueRenderer) RenderOver(
	game string,
	width, height int,
	dialogue QuestDialogueView,
) string {
	windowWidth := min(max(width-4, 36), 72)
	contentWidth := max(windowWidth-6, 1)
	text := strings.Join(wrapEventText(dialogue.Text, contentWidth), "\n")
	body := []string{
		questDialogueNameStyle.Render(dialogue.NPCName),
		"",
		text,
	}
	prompt := "E/Space: continue • Esc: close"
	if dialogue.CanAccept {
		prompt = "E/Space: accept quest • Esc: decline"
	} else if dialogue.CanComplete {
		prompt = "E/Space: hand over items • Esc: close"
	}
	body = append(body, "", mutedStyle.Render(prompt))

	window := questDialogueWindowStyle.
		Width(contentWidth).
		Render(
			lipgloss.JoinVertical(lipgloss.Left, body...),
		)
	renderedWidth, windowHeight := lipgloss.Size(window)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).
			X(max((width-renderedWidth)/2, 0)).
			Y(max(height-windowHeight, 0)).
			Z(1),
	).Render()
}

type JournalRenderer struct{}

func (JournalRenderer) RenderOver(
	game string,
	width, height int,
	journal JournalView,
) string {
	windowWidth := min(max(width-4, 36), 78)
	contentWidth := max(windowWidth-6, 30)
	listWidth := min(max(contentWidth/3, 14), 24)
	const detailGap = 2
	detailWidth := max(contentWidth-listWidth-1-detailGap, 12)

	listRows := []string{mutedStyle.Render("No active quests")}
	if len(journal.Quests) > 0 {
		listRows = make([]string, len(journal.Quests))
		for index, quest := range journal.Quests {
			name := truncateJournalText(quest.Name, max(listWidth-4, 1))
			if index == journal.Selected {
				listRows[index] = journalSelectedStyle.Render("> " + name)
			} else {
				listRows[index] = "  " + name
			}
		}
	}
	detailRows := []string{
		mutedStyle.Render("Select a quest to view its details."),
	}
	if len(journal.Quests) > 0 {
		selected := min(max(journal.Selected, 0), len(journal.Quests)-1)
		quest := journal.Quests[selected]
		status := fmt.Sprintf(
			"In progress — %d of %d", quest.Current, quest.Required,
		)
		if quest.Current >= quest.Required {
			status = "Ready to return"
		}
		detailRows = []string{
			journalQuestTitleStyle.Render(
				truncateJournalText(quest.Name, detailWidth),
			),
			"",
		}
		detailRows = append(
			detailRows,
			wrapEventText(quest.Description, detailWidth)...,
		)
		detailRows = append(detailRows, "")
		detailRows = append(
			detailRows,
			wrapEventText("Objective: "+quest.ItemName, detailWidth)...,
		)
		detailRows = append(
			detailRows,
			wrapEventText(status, detailWidth)...,
		)
		detailRows = append(
			detailRows,
			wrapEventText(
				"Return to: "+questGiverLocation(quest), detailWidth,
			)...,
		)
		if quest.RewardGold > 0 {
			detailRows = append(detailRows, wrapEventText(
				fmt.Sprintf("Reward: %d gold", quest.RewardGold),
				detailWidth,
			)...,
			)
		}
	}
	paneHeight := max(len(listRows), len(detailRows))
	paneRows := make([]string, paneHeight)
	for row := range paneRows {
		left, right := "", ""
		if row < len(listRows) {
			left = listRows[row]
		}
		if row < len(detailRows) {
			right = detailRows[row]
		}
		paneRows[row] = padJournalCell(left, listWidth) +
			journalDividerStyle.Render("│") +
			strings.Repeat(" ", detailGap) +
			padJournalCell(right, detailWidth)
	}

	controls := "↑/↓ select • J/Esc close"
	if contentWidth >= 46 {
		controls = "W/S or ↑/↓ to select • J or Esc to close"
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		journalTitleStyle.Render("QUEST JOURNAL"),
		"",
		strings.Join(paneRows, "\n"),
		"",
		mutedStyle.Render(controls),
	)
	window := journalWindowStyle.Width(windowWidth).Render(body)
	renderedWidth, windowHeight := lipgloss.Size(window)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).
			X(max((width-renderedWidth)/2, 0)).
			Y(max((height-windowHeight)/2, 0)).
			Z(1),
	).Render()
}

func questGiverLocation(quest QuestView) string {
	if quest.GiverArea == "" {
		return quest.GiverName
	}
	return quest.GiverName + " — " + quest.GiverArea
}

func padJournalCell(value string, width int) string {
	padding := max(width-lipgloss.Width(value), 0)
	return value + strings.Repeat(" ", padding)
}

func truncateJournalText(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

var (
	journalTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FACC15"))
	journalQuestTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FDE68A"))
	journalWindowStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#64748B"))
	journalSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FDE68A"))
	journalDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#475569"))
	questDialogueNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F59E0B"))
	questDialogueWindowStyle = lipgloss.NewStyle().
					Padding(0, 2).
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("#F59E0B"))
)
