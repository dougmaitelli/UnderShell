package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshrpg/src/domain"
	"sshrpg/src/item"
	"sshrpg/src/repository"
	"sshrpg/src/world"
)

type inventoryState struct {
	selected int
	inFlight bool
	message  string
}

type InventoryItemView struct {
	Item       domain.InventoryItem
	Definition *item.Definition
	Equipped   bool
}

type EquipmentSlotView struct {
	Slot     item.EquipmentSlot
	ItemName string
}

type InventoryView struct {
	Items     []InventoryItemView
	Equipment []EquipmentSlotView
	Selected  int
	InFlight  bool
	Message   string
}

type inventoryEquipmentMsg struct {
	inventory *domain.Inventory
	itemName  string
	equipped  bool
	err       error
}

type inventoryConsumableMsg struct {
	inventory     *domain.Inventory
	result        world.ConsumableResult
	itemName      string
	notApplicable bool
	err           error
}

var equipmentSlotOrder = []item.EquipmentSlot{
	item.SlotHelmet,
	item.SlotWeapon,
	item.SlotArmor,
	item.SlotGloves,
	item.SlotLegs,
	item.SlotBoots,
}

func (s *inventoryState) view(
	inventory *domain.Inventory,
	definitions *item.Items,
) InventoryView {
	view := InventoryView{
		Selected: s.selected, InFlight: s.inFlight, Message: s.message,
		Equipment: make([]EquipmentSlotView, len(equipmentSlotOrder)),
	}
	for index, slot := range equipmentSlotOrder {
		view.Equipment[index].Slot = slot
	}
	if inventory == nil {
		return view
	}
	view.Items = make([]InventoryItemView, 0, len(inventory.Items))
	namesBySlot := make(map[int]string, len(inventory.Items))
	for _, owned := range inventory.Items {
		var definition *item.Definition
		if definitions != nil {
			definition, _ = definitions.Item(owned.ItemKey)
		}
		name := owned.ItemKey
		if definition != nil {
			name = definition.Name
		}
		namesBySlot[owned.Slot] = name
		view.Items = append(view.Items, InventoryItemView{
			Item: owned, Definition: definition,
			Equipped: inventory.IsEquipped(owned.Slot),
		})
	}
	for index, slot := range equipmentSlotOrder {
		inventorySlot, equipped := inventory.EquippedInventorySlot(string(slot))
		if equipped {
			view.Equipment[index].ItemName = namesBySlot[inventorySlot]
		}
	}
	if len(view.Items) == 0 {
		view.Selected = 0
	} else if view.Selected >= len(view.Items) {
		view.Selected = len(view.Items) - 1
	}
	return view
}

func (m *gameModel) inventoryView() InventoryView {
	var definitions *item.Items
	if m.world != nil {
		definitions = m.world.Items()
	}
	return m.inventoryMenu.view(m.inventory, definitions)
}

func (m *gameModel) updateInventoryInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if key == "i" || key == "esc" {
		m.mode = inputModeGame
		m.inventoryMenu.message = ""
		return m, nil
	}
	if m.inventoryMenu.inFlight || m.inventory == nil {
		return m, nil
	}
	count := len(m.inventory.Items)
	switch key {
	case "up", "w":
		if count > 0 {
			m.inventoryMenu.selected =
				(m.inventoryMenu.selected - 1 + count) % count
		}
	case "down", "s":
		if count > 0 {
			m.inventoryMenu.selected = (m.inventoryMenu.selected + 1) % count
		}
	case "e", "space":
		view := m.inventoryView()
		if len(view.Items) == 0 {
			return m, nil
		}
		selected := view.Items[view.Selected]
		if selected.Definition == nil {
			m.inventoryMenu.message = "That item cannot be used."
			return m, nil
		}
		switch selected.Definition.Type {
		case item.TypeEquipment:
			m.inventoryMenu.inFlight = true
			m.inventoryMenu.message = ""
			return m, m.toggleEquipment(selected)
		case item.TypeConsumable:
			if !m.consumableCanApply(selected.Definition) {
				m.inventoryMenu.message = "Health is already full."
				return m, nil
			}
			m.inventoryMenu.inFlight = true
			m.inventoryMenu.message = ""
			return m, m.useConsumable(selected)
		default:
			m.inventoryMenu.message = "That item cannot be used."
		}
	}
	return m, nil
}

func (m *gameModel) consumableCanApply(definition *item.Definition) bool {
	for _, effect := range definition.Effects {
		if effect.Type != item.EffectRestoreHealth {
			return true
		}
		for _, player := range m.connection.snapshot.Players {
			if player.ID == m.character.ID {
				return player.Health < player.MaxHealth
			}
		}
	}
	return true
}

func (m *gameModel) useConsumable(selected InventoryItemView) tea.Cmd {
	return func() tea.Msg {
		definition := selected.Definition
		inventory, err := m.repositories.Inventories.ConsumeItem(
			context.Background(), m.character.ID,
			selected.Item.Slot, selected.Item.ItemKey,
		)
		if err != nil {
			return inventoryConsumableMsg{
				itemName: definition.Name, err: err,
			}
		}
		result := m.world.UseConsumable(
			m.character.ID, m.connection.session.Token, definition.ID,
		)
		if result.Applied {
			return inventoryConsumableMsg{
				inventory: inventory, result: result,
				itemName: definition.Name,
			}
		}
		refunded, refundErr := m.repositories.Inventories.AddItem(
			context.Background(), m.character.ID,
			definition.ID, definition.MaxStack,
		)
		if refundErr != nil {
			return inventoryConsumableMsg{
				inventory: inventory,
				itemName:  definition.Name,
				err: fmt.Errorf(
					"restore unapplied consumable: %w",
					refundErr,
				),
			}
		}
		return inventoryConsumableMsg{
			inventory: refunded, itemName: definition.Name,
			notApplicable: true,
		}
	}
}

func (m *gameModel) toggleEquipment(selected InventoryItemView) tea.Cmd {
	return func() tea.Msg {
		definition := selected.Definition
		var (
			inventory *domain.Inventory
			err       error
			equipped  bool
		)
		if selected.Equipped {
			inventory, err = m.repositories.Inventories.Unequip(
				context.Background(), m.character.ID,
				string(definition.EquipmentSlot),
			)
		} else {
			equipped = true
			inventory, err = m.repositories.Inventories.Equip(
				context.Background(), m.character.ID,
				selected.Item.Slot, selected.Item.ItemKey,
				string(definition.EquipmentSlot),
			)
		}
		return inventoryEquipmentMsg{
			inventory: inventory, itemName: definition.Name,
			equipped: equipped, err: err,
		}
	}
}

func (m *gameModel) updateInventoryEquipment(
	msg inventoryEquipmentMsg,
) (tea.Model, tea.Cmd) {
	m.inventoryMenu.inFlight = false
	if msg.err != nil {
		if errors.Is(msg.err, repository.ErrItemNotOwned) {
			m.inventoryMenu.message = "That item is no longer available."
		} else {
			m.inventoryMenu.message = "Equipment could not be updated."
			if m.log != nil {
				m.log.Error(
					"update equipment",
					"character_id", m.character.ID,
					"error", msg.err,
				)
			}
		}
		return m, nil
	}
	m.inventory = msg.inventory
	action := "Unequipped "
	if msg.equipped {
		action = "Equipped "
	}
	m.inventoryMenu.message = action + msg.itemName + "."
	return m, nil
}

func (m *gameModel) updateInventoryConsumable(
	msg inventoryConsumableMsg,
) (tea.Model, tea.Cmd) {
	m.inventoryMenu.inFlight = false
	if msg.inventory != nil {
		m.inventory = msg.inventory
	}
	if msg.err != nil {
		if errors.Is(msg.err, repository.ErrItemNotOwned) {
			m.inventoryMenu.message = "That item is no longer available."
		} else {
			m.inventoryMenu.message = "That item could not be used."
			if m.log != nil {
				m.log.Error(
					"use consumable",
					"character_id", m.character.ID,
					"error", msg.err,
				)
			}
		}
		return m, nil
	}
	if msg.notApplicable {
		m.inventoryMenu.message = "That item has no effect right now."
		return m, nil
	}
	for index := range m.connection.snapshot.Players {
		if m.connection.snapshot.Players[index].ID == msg.result.Player.ID {
			m.connection.snapshot.Players[index] = msg.result.Player
			break
		}
	}
	m.inventoryMenu.message = fmt.Sprintf(
		"Used %s. Restored %d health.",
		msg.itemName, msg.result.HealthRestored,
	)
	return m, m.addEvent(EventView{
		Kind: world.EventConsumable,
		Message: fmt.Sprintf(
			"Used %s (+%d health)",
			msg.itemName, msg.result.HealthRestored,
		),
	})
}

type InventoryRenderer struct{}

// RenderOver accepts an optional resolved view so older callers can still
// render a raw inventory.
func (InventoryRenderer) RenderOver(
	game string,
	width, height int,
	inventory *domain.Inventory,
	resolved ...InventoryView,
) string {
	view := InventoryView{
		Equipment: make([]EquipmentSlotView, len(equipmentSlotOrder)),
	}
	for index, slot := range equipmentSlotOrder {
		view.Equipment[index].Slot = slot
	}
	if len(resolved) > 0 {
		view = resolved[0]
	} else if inventory != nil {
		for _, owned := range inventory.Items {
			view.Items = append(view.Items, InventoryItemView{
				Item: owned, Equipped: inventory.IsEquipped(owned.Slot),
			})
		}
	}

	windowWidth := min(max(width-4, 42), 76)
	contentWidth := max(windowWidth-6, 30)
	listWidth := min(max(contentWidth/3, 18), 27)
	const detailGap = 2
	detailWidth := max(contentWidth-listWidth-1-detailGap, 12)

	listRows := []string{mutedStyle.Render("Your inventory is empty.")}
	if len(view.Items) > 0 {
		listRows = make([]string, len(view.Items))
		for index, entry := range view.Items {
			name := entry.Item.ItemKey
			if entry.Definition != nil {
				name = entry.Definition.Name
			}
			marker := "  "
			if index == view.Selected {
				marker = "> "
			}
			equipped := ""
			if entry.Equipped {
				equipped = " ◆"
			}
			quantity := ""
			if entry.Item.Quantity > 1 {
				quantity = fmt.Sprintf(" ×%d", entry.Item.Quantity)
			}
			row := marker + truncateJournalText(
				name+quantity+equipped,
				max(listWidth-lipgloss.Width(marker), 1),
			)
			if index == view.Selected {
				row = inventorySelectedStyle.Render(row)
			}
			listRows[index] = row
		}
	}

	detailRows := []string{mutedStyle.Render("Select an item to view details.")}
	if len(view.Items) > 0 {
		selected := min(max(view.Selected, 0), len(view.Items)-1)
		entry := view.Items[selected]
		name := entry.Item.ItemKey
		itemType := "Unknown"
		description := "No item definition is available."
		if entry.Definition != nil {
			name = entry.Definition.Name
			itemType = titleInventoryValue(string(entry.Definition.Type))
			description = entry.Definition.Description
		}
		detailRows = []string{
			inventoryItemTitleStyle.Render(
				truncateJournalText(name, detailWidth),
			),
			"",
			"Type: " + itemType,
		}
		if entry.Definition != nil &&
			entry.Definition.Type == item.TypeEquipment {
			detailRows = append(
				detailRows,
				"Slot: "+titleInventoryValue(
					string(entry.Definition.EquipmentSlot),
				),
			)
			if entry.Equipped {
				detailRows = append(detailRows, "Status: Equipped")
			}
		} else if entry.Definition != nil &&
			entry.Definition.Type == item.TypeConsumable {
			for _, effect := range entry.Definition.Effects {
				detailRows = append(
					detailRows,
					inventoryEffectDescription(effect),
				)
			}
		}
		detailRows = append(detailRows, "")
		detailRows = append(
			detailRows,
			wrapEventText(description, detailWidth)...,
		)
	}
	status := view.Message
	if view.InFlight {
		status = "Updating item…"
	}
	paneOverhead := 14
	if height < 18 {
		paneOverhead = 8
	}
	if status != "" {
		paneOverhead++
	}
	maxPaneHeight := max(height-paneOverhead, 1)
	listRows = inventoryVisibleRows(
		listRows, view.Selected, maxPaneHeight,
	)
	if len(detailRows) > maxPaneHeight {
		detailRows = detailRows[:maxPaneHeight]
	}
	paneRows := inventoryPaneRows(listRows, detailRows, listWidth, detailWidth)

	body := []string{
		inventoryTitleStyle.Render("INVENTORY"),
		"",
		strings.Join(paneRows, "\n"),
	}
	if height >= 18 {
		body = append(body, "", mutedStyle.Render("EQUIPMENT"))
		body = append(body, equipmentRows(view.Equipment, contentWidth)...)
	}
	action := inventoryAction(view)
	controls := "W/S or ↑/↓: select • E/Space: " + action + " • I/Esc: close"
	if contentWidth < lipgloss.Width(controls) {
		controls = "↑/↓ select • E " + action + " • I/Esc close"
	}
	body = append(
		body, "",
		mutedStyle.Render(controls),
	)
	if status != "" {
		body = append(body, inventoryStatusStyle.Render(status))
	}
	window := inventoryWindowStyle.
		Width(windowWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, body...))
	windowWidthRendered, windowHeight := lipgloss.Size(window)

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).
			X(max((width-windowWidthRendered)/2, 0)).
			Y(max((height-windowHeight)/2, 0)).
			Z(1),
	).Render()
}

func inventoryAction(view InventoryView) string {
	if len(view.Items) == 0 {
		return "use"
	}
	selected := min(max(view.Selected, 0), len(view.Items)-1)
	definition := view.Items[selected].Definition
	if definition != nil && definition.Type == item.TypeConsumable {
		return "use"
	}
	return "equip"
}

func inventoryEffectDescription(effect item.Effect) string {
	switch effect.Type {
	case item.EffectRestoreHealth:
		return fmt.Sprintf("Effect: Restore %d health", effect.Amount)
	default:
		return "Effect: Unknown"
	}
}

func inventoryVisibleRows(rows []string, selected, limit int) []string {
	if len(rows) <= limit {
		return rows
	}
	start := selected - limit/2
	start = min(max(start, 0), len(rows)-limit)
	return rows[start : start+limit]
}

func inventoryPaneRows(
	leftRows, rightRows []string,
	leftWidth, rightWidth int,
) []string {
	const detailGap = 2
	height := max(len(leftRows), len(rightRows))
	rows := make([]string, height)
	for row := range rows {
		left, right := "", ""
		if row < len(leftRows) {
			left = leftRows[row]
		}
		if row < len(rightRows) {
			right = rightRows[row]
		}
		rows[row] = padJournalCell(left, leftWidth) +
			inventoryDividerStyle.Render("│") +
			strings.Repeat(" ", detailGap) +
			padJournalCell(right, rightWidth)
	}
	return rows
}

func equipmentRows(slots []EquipmentSlotView, width int) []string {
	cellWidth := max((width-2)/2, 1)
	rows := make([]string, 0, (len(slots)+1)/2)
	for index := 0; index < len(slots); index += 2 {
		cells := make([]string, 0, 2)
		for offset := 0; offset < 2 && index+offset < len(slots); offset++ {
			slot := slots[index+offset]
			name := slot.ItemName
			if name == "" {
				name = "Empty"
			}
			value := fmt.Sprintf(
				"%-7s %s",
				titleInventoryValue(string(slot.Slot))+":",
				name,
			)
			cells = append(cells, padJournalCell(
				truncateJournalText(value, cellWidth),
				cellWidth,
			))
		}
		rows = append(rows, strings.Join(cells, "  "))
	}
	return rows
}

func titleInventoryValue(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

var (
	inventoryTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FBBF24"))
	inventoryItemTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FDE68A"))
	inventoryWindowStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#64748B"))
	inventorySelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FDE68A"))
	inventoryDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#475569"))
	inventoryStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FDE68A"))
)
