package entity

import "github.com/uptrace/bun"

// CharacterEquipment assigns an owned inventory slot to one equipment slot.
type CharacterEquipment struct {
	bun.BaseModel `bun:"table:character_equipment,alias:character_equipment"`

	CharacterID   int64  `bun:"character_id,pk,unique:character_equipment_inventory"`
	EquipmentSlot string `bun:"equipment_slot,pk"`
	InventorySlot int    `bun:"inventory_slot,notnull,unique:character_equipment_inventory"`
}
