package domain

type Inventory struct {
	CharacterID int64
	Items       []InventoryItem
}

type InventoryItem struct {
	Slot     int
	ItemKey  string
	Quantity int
}
