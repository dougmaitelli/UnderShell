package world

import (
	"time"

	"sshrpg/src/domain"
	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

type Player struct {
	ID             int64
	Name           string
	Role           domain.CharacterRole
	AreaID         string
	X              int
	Y              int
	Health         int
	MaxHealth      int
	Level          int
	Experience     int64
	SkillPoints    int
	Attack         int
	Defense        int
	Vitality       int
	EquipmentStats item.EquipmentStats
}

type Snapshot struct {
	Area    *Area
	Players []Player
	Enemies []Enemy
	Drops   []GroundItem
}

type Enemy struct {
	ID         uint64
	Definition *enemy.Definition
	Health     int
	AreaID     string
	X          int
	Y          int
	spawnIndex int
	nextAttack time.Time
}

type GroundItem struct {
	ID     uint64
	Item   *item.Definition
	AreaID string
	X      int
	Y      int
}

type Session struct {
	Token   string
	Updates <-chan Snapshot
	Events  <-chan Event
	Chats   <-chan ChatMessage
	Kicked  <-chan string
}

type ChatMessage struct {
	Type       ChatMessageType
	PlayerID   int64
	PlayerName string
	PlayerRole domain.CharacterRole
	Message    string
}

type ChatMessageType string

const (
	ChatMessagePlayer ChatMessageType = ""
	ChatMessageServer ChatMessageType = "server"
)

type EventKind string

const (
	EventPickup      EventKind = "pickup"
	EventProgression EventKind = "progression"
	EventCombat      EventKind = "combat"
	EventDamage      EventKind = "damage"
	EventDeath       EventKind = "death"
	EventRespawn     EventKind = "respawn"
	EventQuest       EventKind = "quest"
	EventConsumable  EventKind = "consumable"
	EventTrade       EventKind = "trade"
	EventAdmin       EventKind = "admin"
)

type Event struct {
	Kind             EventKind
	Message          string
	InventoryChanged bool
}

type AttackResult struct {
	HitIDs      []uint64
	DefeatedIDs []uint64
}

type PickupResult struct {
	Item  GroundItem
	Found bool
}

type ConsumableResult struct {
	Player         Player
	Applied        bool
	HealthRestored int
}

const (
	attackRange            = 2
	pickupRange            = 2
	enemyAggroRange        = 8
	playerMaxHealth        = 10
	enemyAttackInterval    = 1500 * time.Millisecond
	horizontalMoveInterval = 75 * time.Millisecond
	verticalMoveInterval   = 100 * time.Millisecond
	vitalityHealthPerRank  = 5
	chatHistoryLimit       = 10
	chatMessageLimit       = 200
)
