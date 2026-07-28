package domain

type Inventory struct {
	CharacterID int64
	Items       []InventoryItem
	Equipment   []EquippedItem
}

type InventoryItem struct {
	Slot     int
	ItemKey  string
	Quantity int
}

// EquippedItem assigns one owned inventory slot to an equipment slot.
type EquippedItem struct {
	EquipmentSlot string
	InventorySlot int
}

func (i *Inventory) EquippedInventorySlot(equipmentSlot string) (int, bool) {
	if i == nil {
		return 0, false
	}
	for _, equipped := range i.Equipment {
		if equipped.EquipmentSlot == equipmentSlot {
			return equipped.InventorySlot, true
		}
	}
	return 0, false
}

func (i *Inventory) IsEquipped(inventorySlot int) bool {
	if i == nil {
		return false
	}
	for _, equipped := range i.Equipment {
		if equipped.InventorySlot == inventorySlot {
			return true
		}
	}
	return false
}
