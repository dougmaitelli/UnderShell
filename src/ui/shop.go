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
	"sshrpg/src/repository"
)

const shopInteractionRange = 2

type shopTab uint8

const (
	shopTabBuy shopTab = iota
	shopTabSell
)

type shopState struct {
	npc      *npcconfig.Definition
	tab      shopTab
	selected int
	inFlight bool
	message  string
}

type shopSellEntry struct {
	Item      domain.InventoryItem
	Name      string
	SellPrice int
}

type ShopView struct {
	NPCName  string
	Tab      shopTab
	Selected int
	Gold     int
	Stock    []npcconfig.ShopItem
	Sell     []shopSellEntry
	Message  string
	InFlight bool
}

type shopTradeMsg struct {
	result   repository.TradeResult
	itemName string
	buying   bool
	err      error
}

func (s *shopState) open(npc *npcconfig.Definition) {
	s.npc = npc
	s.tab = shopTabBuy
	s.selected = 0
	s.inFlight = false
	s.message = ""
}

func (s *shopState) close() {
	*s = shopState{}
}

func (s *shopState) sellEntries(inventory *domain.Inventory) []shopSellEntry {
	if s.npc == nil || inventory == nil {
		return nil
	}
	prices := make(map[string]npcconfig.ShopItem, len(s.npc.Stock))
	for _, stock := range s.npc.Stock {
		prices[stock.ItemID] = stock
	}
	entries := make([]shopSellEntry, 0, len(inventory.Items))
	for _, inventoryItem := range inventory.Items {
		stock, ok := prices[inventoryItem.ItemKey]
		if !ok {
			continue
		}
		name := stock.Name
		if name == "" {
			name = inventoryItem.ItemKey
		}
		entries = append(entries, shopSellEntry{
			Item: inventoryItem, Name: name, SellPrice: stock.SellPrice,
		})
	}
	return entries
}

func (s *shopState) view(character *domain.Character, inventory *domain.Inventory) ShopView {
	if s.npc == nil {
		return ShopView{}
	}
	gold := 0
	if character != nil {
		gold = character.Gold
	}
	return ShopView{
		NPCName: s.npc.Name, Tab: s.tab, Selected: s.selected, Gold: gold,
		Stock: append([]npcconfig.ShopItem(nil), s.npc.Stock...),
		Sell:  s.sellEntries(inventory), Message: s.message, InFlight: s.inFlight,
	}
}

func (m *gameModel) nearbyShop() *npcconfig.Definition {
	if m.character == nil || m.connection.snapshot.Area == nil {
		return nil
	}
	area := m.connection.snapshot.Area
	if area.ID != m.character.AreaID {
		return nil
	}
	for index := range area.NPCs {
		npc := &area.NPCs[index]
		if npc.Type == npcconfig.TypeShop &&
			absolute(npc.X-m.character.X) <= shopInteractionRange &&
			absolute(npc.Y-m.character.Y) <= shopInteractionRange {
			return npc
		}
	}
	return nil
}

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (m *gameModel) openShop(npc *npcconfig.Definition) (tea.Model, tea.Cmd) {
	m.shop.open(npc)
	m.mode = inputModeShop
	m.movement.stop()
	return m, nil
}

func (m *gameModel) updateShopInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if key == "esc" {
		m.shop.close()
		m.mode = inputModeGame
		return m, nil
	}
	if m.shop.inFlight {
		return m, nil
	}
	if key == "tab" {
		if m.shop.tab == shopTabBuy {
			m.shop.tab = shopTabSell
		} else {
			m.shop.tab = shopTabBuy
		}
		m.shop.selected = 0
		m.shop.message = ""
		return m, nil
	}
	count := len(m.shop.npc.Stock)
	if m.shop.tab == shopTabSell {
		count = len(m.shop.sellEntries(m.inventory))
	}
	switch key {
	case "up", "w":
		if count > 0 {
			m.shop.selected = (m.shop.selected - 1 + count) % count
		}
	case "down", "s":
		if count > 0 {
			m.shop.selected = (m.shop.selected + 1) % count
		}
	case "e", "space":
		if count == 0 {
			return m, nil
		}
		m.shop.inFlight = true
		m.shop.message = ""
		if m.shop.tab == shopTabBuy {
			return m, m.buyShopItem(m.shop.npc.Stock[m.shop.selected])
		}
		return m, m.sellShopItem(m.shop.sellEntries(m.inventory)[m.shop.selected])
	}
	return m, nil
}

func (m *gameModel) buyShopItem(stock npcconfig.ShopItem) tea.Cmd {
	return func() tea.Msg {
		result, err := m.repositories.Shops.BuyItem(
			context.Background(), m.character.ID,
			stock.ItemID, stock.MaxStack, stock.BuyPrice,
		)
		return shopTradeMsg{result: result, itemName: stock.Name, buying: true, err: err}
	}
}

func (m *gameModel) sellShopItem(entry shopSellEntry) tea.Cmd {
	return func() tea.Msg {
		result, err := m.repositories.Shops.SellItem(
			context.Background(), m.character.ID,
			entry.Item.Slot, entry.Item.ItemKey, entry.SellPrice,
		)
		return shopTradeMsg{result: result, itemName: entry.Name, err: err}
	}
}

func (m *gameModel) updateShopTrade(msg shopTradeMsg) (tea.Model, tea.Cmd) {
	m.shop.inFlight = false
	if msg.err != nil {
		if errors.Is(msg.err, repository.ErrInsufficientGold) {
			m.shop.message = "Not enough gold."
		} else if errors.Is(msg.err, repository.ErrItemNotOwned) {
			m.shop.message = "That item is no longer available."
		} else {
			m.shop.message = "The trade could not be completed."
			m.log.Error("shop trade", "character_id", m.character.ID, "error", msg.err)
		}
		return m, nil
	}
	m.inventory = msg.result.Inventory
	m.character.Gold = msg.result.Gold
	if msg.buying {
		m.shop.message = "Bought " + msg.itemName + "."
	} else {
		m.shop.message = "Sold " + msg.itemName + "."
	}
	if m.shop.tab == shopTabSell {
		count := len(m.shop.sellEntries(m.inventory))
		if count == 0 {
			m.shop.selected = 0
		} else if m.shop.selected >= count {
			m.shop.selected = count - 1
		}
	}
	return m, nil
}

type ShopRenderer struct{}

func (ShopRenderer) RenderOver(game string, width, height int, view ShopView) string {
	tab := "BUY"
	rows := make([]string, 0, len(view.Stock))
	if view.Tab == shopTabSell {
		tab = "SELL"
		for index, entry := range view.Sell {
			rows = append(rows, shopRow(
				index == view.Selected,
				fmt.Sprintf("%s ×%d", entry.Name, entry.Item.Quantity),
				entry.SellPrice,
			))
		}
	} else {
		for index, stock := range view.Stock {
			name := stock.Name
			if name == "" {
				name = stock.ItemID
			}
			rows = append(rows, shopRow(index == view.Selected, name, stock.BuyPrice))
		}
	}
	if len(rows) == 0 {
		rows = append(rows, mutedStyle.Render("Nothing available."))
	}
	status := view.Message
	if view.InFlight {
		status = "Trading…"
	}
	body := []string{
		shopTitleStyle.Render(view.NPCName + "'s Shop"),
		fmt.Sprintf("[%s]  Gold: %d", tab, view.Gold),
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		mutedStyle.Render("Tab: buy/sell • ↑/↓: select"),
		mutedStyle.Render("E/Space: trade • Esc: close"),
	}
	if status != "" {
		body = append(body, "", shopStatusStyle.Render(status))
	}
	window := shopWindowStyle.Render(lipgloss.JoinVertical(lipgloss.Left, body...))
	windowWidth, windowHeight := lipgloss.Size(window)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(game).X(0).Y(0).Z(0),
		lipgloss.NewLayer(window).
			X(max((width-windowWidth)/2, 0)).
			Y(max((height-windowHeight)/2, 0)).
			Z(1),
	).Render()
}

func shopRow(selected bool, name string, price int) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	return fmt.Sprintf("%s%-24s %3d gold", prefix, name, price)
}

var (
	shopTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FBBF24"))
	shopWindowStyle = lipgloss.NewStyle().
			Width(46).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#D97706"))
	shopStatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FDE68A"))
)
