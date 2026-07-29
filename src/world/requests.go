package world

import (
	"sshrpg/src/domain"
	"sshrpg/src/item"
)

type joinRequest struct {
	player Player
	reply  chan Session
}

type moveRequest struct {
	id    int64
	token string
	dx    int
	dy    int
	reply chan Player
}

type leaveRequest struct {
	id    int64
	token string
}

type defeatEnemyRequest struct {
	id    uint64
	reply chan bool
}

type attackRequest struct {
	id    int64
	token string
	reply chan AttackResult
}

type pickupRequest struct {
	id    int64
	token string
	reply chan PickupResult
}

type useConsumableRequest struct {
	id         int64
	token      string
	definition *item.Definition
	reply      chan ConsumableResult
}

type updateEquipmentRequest struct {
	id    int64
	token string
	stats item.EquipmentStats
	reply chan Player
}

type spendSkillRequest struct {
	id    int64
	token string
	skill string
	reply chan Player
}

type chatRequest struct {
	id      int64
	token   string
	message string
	reply   chan bool
}

type adminAuthorizeRequest struct {
	id    int64
	token string
	reply chan adminAuthorizeResult
}

type adminAuthorizeResult struct {
	role domain.CharacterRole
	ok   bool
}

type adminFindPlayerRequest struct {
	name  string
	reply chan adminPlayerResult
}

type adminPlayerResult struct {
	player Player
	err    error
}

type adminGrantExperienceRequest struct {
	name   string
	amount int64
	reply  chan adminPlayerResult
}

type adminGrantLevelsRequest struct {
	name   string
	amount int
	reply  chan adminPlayerResult
}

type adminTeleportAreaRequest struct {
	name  string
	area  string
	reply chan adminPlayerResult
}

type adminTeleportPlayerRequest struct {
	name        string
	destination string
	reply       chan adminPlayerResult
}

type adminNotifyRequest struct {
	id               int64
	message          string
	inventoryChanged bool
	reply            chan bool
}

type adminSetRoleRequest struct {
	name  string
	role  domain.CharacterRole
	reply chan adminPlayerResult
}

type adminKickRequest struct {
	name   string
	reason string
	reply  chan adminPlayerResult
}
