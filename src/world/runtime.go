package world

import (
	"fmt"
	"time"
)

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
		enemies: newEnemySystem(manager.areas),
		loot:    newLootSystem(),
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
	case restorePickupRequest:
		request.reply <- s.loot.restore(
			s.players.authenticated(request.id, request.token),
			request.item, s.broadcast,
		)
	case useConsumableRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- ConsumableResult{}
			return
		}
		result := player.useConsumable(request.definition)
		if result.Applied {
			s.broadcast()
		}
		request.reply <- result
	case updateEquipmentRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- Player{}
			return
		}
		player.setEquipmentStats(request.stats)
		s.broadcast()
		request.reply <- player.Player
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
	case serverChatRequest:
		request.reply <- s.chat.sendServer(s.players.live, request.message)
	case adminAuthorizeRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- adminAuthorizeResult{}
			return
		}
		request.reply <- adminAuthorizeResult{role: player.Role, ok: true}
	case adminFindPlayerRequest:
		request.reply <- findAdminPlayer(&s.players, request.name)
	case adminGrantExperienceRequest:
		player := s.players.byName(request.name)
		if player == nil {
			request.reply <- adminPlayerResult{err: ErrPlayerNotOnline}
			return
		}
		previousLevel := player.Level
		grantExperience(&player.Player, request.amount)
		sendEvent(player, Event{
			Kind: EventAdmin,
			Message: fmt.Sprintf(
				"An administrator granted you %d XP", request.amount,
			),
		})
		if player.Level > previousLevel {
			sendEvent(player, Event{
				Kind:    EventProgression,
				Message: fmt.Sprintf("Reached level %d", player.Level),
			})
		}
		s.broadcast()
		request.reply <- adminPlayerResult{player: player.Player}
	case adminGrantLevelsRequest:
		player := s.players.byName(request.name)
		if player == nil {
			request.reply <- adminPlayerResult{err: ErrPlayerNotOnline}
			return
		}
		player.grantLevels(request.amount)
		sendEvent(player, Event{
			Kind: EventAdmin,
			Message: fmt.Sprintf(
				"An administrator granted you %d level(s)", request.amount,
			),
		})
		s.broadcast()
		request.reply <- adminPlayerResult{player: player.Player}
	case adminTeleportAreaRequest:
		player := s.players.byName(request.name)
		if player == nil {
			request.reply <- adminPlayerResult{err: ErrPlayerNotOnline}
			return
		}
		area, ok := s.players.areas.FindArea(request.area)
		if !ok {
			request.reply <- adminPlayerResult{
				err: fmt.Errorf("area %q not found", request.area),
			}
			return
		}
		player.AreaID = area.ID
		player.X, player.Y = area.Spawn.X, area.Spawn.Y
		sendEvent(player, Event{
			Kind:    EventAdmin,
			Message: fmt.Sprintf("Teleported to %s", area.Name),
		})
		s.broadcast()
		request.reply <- adminPlayerResult{player: player.Player}
	case adminTeleportPlayerRequest:
		player := s.players.byName(request.name)
		if player == nil {
			request.reply <- adminPlayerResult{err: ErrPlayerNotOnline}
			return
		}
		destination := s.players.byName(request.destination)
		if destination == nil {
			request.reply <- adminPlayerResult{
				err: fmt.Errorf(
					"destination player %q is not online",
					request.destination,
				),
			}
			return
		}
		player.AreaID = destination.AreaID
		player.X, player.Y = destination.X, destination.Y
		sendEvent(player, Event{
			Kind:    EventAdmin,
			Message: fmt.Sprintf("Teleported to %s", destination.Name),
		})
		s.broadcast()
		request.reply <- adminPlayerResult{player: player.Player}
	case adminNotifyRequest:
		player := s.players.live[request.id]
		if player == nil {
			request.reply <- false
			return
		}
		sendEvent(player, Event{
			Kind: EventAdmin, Message: request.message,
			InventoryChanged: request.inventoryChanged,
		})
		request.reply <- true
	case adminSetRoleRequest:
		player := s.players.byName(request.name)
		if player == nil {
			request.reply <- adminPlayerResult{err: ErrPlayerNotOnline}
			return
		}
		player.Role = request.role
		sendEvent(player, Event{
			Kind:    EventAdmin,
			Message: fmt.Sprintf("Your role is now %s", request.role),
		})
		s.broadcast()
		request.reply <- adminPlayerResult{player: player.Player}
	case adminKickRequest:
		player, err := s.players.kick(request.name, request.reason)
		if err == nil {
			s.broadcast()
		}
		request.reply <- adminPlayerResult{player: player, err: err}
	}
}

func findAdminPlayer(players *playerSystem, name string) adminPlayerResult {
	player := players.byName(name)
	if player == nil {
		return adminPlayerResult{err: ErrPlayerNotOnline}
	}
	return adminPlayerResult{player: player.Player}
}

func (s *runtimeState) snapshot(recipient *activePlayer) Snapshot {
	return s.players.snapshot(recipient, s.enemies.live, s.loot.live)
}

func (s *runtimeState) broadcast() {
	s.players.broadcast(s.snapshot)
}
