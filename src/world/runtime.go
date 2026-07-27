package world

import "time"

type runtimeState struct {
	players playerSystem
	enemies enemySystem
	loot    lootSystem
	chat    chatSystem
	combat  combatSystem
}

func newRuntimeState(manager *Manager) *runtimeState {
	state := &runtimeState{
		players: playerSystem{
			areas: manager.areas,
			live:  make(map[int64]*activePlayer),
		},
		enemies: newEnemySystem(manager.areas, manager.enemies),
		loot:    newLootSystem(manager.items),
		chat:    newChatSystem(),
	}
	state.enemies.populate()
	return state
}

func (s *runtimeState) run(events <-chan any, done <-chan struct{}) {
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case event := <-events:
			s.handle(event)
		case now := <-ticker.C:
			if s.enemies.tick(&s.players, now) {
				s.broadcast()
			}
		case <-done:
			s.players.closeAll()
			return
		}
	}
}

func (s *runtimeState) handle(event any) {
	switch request := event.(type) {
	case joinRequest:
		request.reply <- s.players.join(request.player, s.chat.history, s.snapshot)
	case moveRequest:
		request.reply <- s.players.move(request.id, request.token, request.dx, request.dy, s.broadcast)
	case leaveRequest:
		if s.players.leave(request.id, request.token) {
			s.broadcast()
		}
	case defeatEnemyRequest:
		target := s.enemies.enemy(request.id)
		if target == nil {
			request.reply <- false
			return
		}
		s.combat.defeatEnemy(&s.enemies, &s.loot, target)
		s.broadcast()
		request.reply <- true
	case attackRequest:
		request.reply <- s.combat.attack(
			&s.players, &s.enemies, &s.loot,
			request.id, request.token, s.broadcast,
		)
	case pickupRequest:
		request.reply <- s.loot.pickup(
			s.players.authenticated(request.id, request.token), s.broadcast,
		)
	case spendSkillRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil || !player.spendSkillPoint(request.skill) {
			request.reply <- Player{}
			return
		}
		s.broadcast()
		request.reply <- player.Player
	case chatRequest:
		request.reply <- s.chat.send(
			s.players.authenticated(request.id, request.token), s.players.live, request.message,
		)
	}
}

func (s *runtimeState) snapshot(recipient *activePlayer) Snapshot {
	return s.players.snapshot(recipient, s.enemies.live, s.loot.live)
}

func (s *runtimeState) broadcast() {
	s.players.broadcast(s.snapshot)
}
