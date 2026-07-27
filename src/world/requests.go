package world

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
