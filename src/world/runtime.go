package world

import (
	"fmt"
	"time"
)

const (
	enemyTickInterval      = 750 * time.Millisecond
	enemyBroadcastInterval = time.Second
)

type runtimeState struct {
	players playerSystem
	enemies enemySystem
	loot    lootSystem
	chat    chatSystem
	combat  combatSystem

	pendingAreas map[string]bool
}

func newRuntimeState(manager *Manager) *runtimeState {
	state := &runtimeState{
		players: playerSystem{
			areas: manager.areas,
			live:  make(map[int64]*activePlayer),
		},
		enemies:      newEnemySystem(manager.areas),
		loot:         newLootSystem(),
		chat:         newChatSystem(),
		pendingAreas: make(map[string]bool),
	}
	state.enemies.populate()
	return state
}

func (s *runtimeState) run(events <-chan any, done <-chan struct{}) {
	enemyTicker := time.NewTicker(enemyTickInterval)
	defer enemyTicker.Stop()
	broadcastTicker := time.NewTicker(enemyBroadcastInterval)
	defer broadcastTicker.Stop()
	for {
		select {
		case event := <-events:
			s.handle(event)
		case now := <-enemyTicker.C:
			for areaID := range s.enemies.tick(&s.players, now) {
				s.pendingAreas[areaID] = true
			}
		case <-broadcastTicker.C:
			if len(s.pendingAreas) > 0 {
				s.broadcastAreas(s.pendingAreas)
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
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- Player{}
			return
		}
		previousAreaID := player.AreaID
		request.reply <- s.players.move(
			request.id, request.token, request.dx, request.dy, time.Now(),
			func() {
				s.broadcastAreas(map[string]bool{
					previousAreaID: true,
					player.AreaID:  true,
				})
			},
		)
	case leaveRequest:
		player := s.players.authenticated(request.id, request.token)
		if s.players.leave(request.id, request.token) {
			s.broadcastArea(player.AreaID)
		}
	case defeatEnemyRequest:
		target := s.enemies.enemy(request.id)
		if target == nil {
			request.reply <- false
			return
		}
		areaID := target.AreaID
		s.combat.defeatEnemy(&s.enemies, &s.loot, target)
		s.broadcastArea(areaID)
		request.reply <- true
	case attackRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- AttackResult{}
			return
		}
		request.reply <- s.combat.attack(
			&s.players, &s.enemies, &s.loot,
			request.id, request.token,
			func() { s.broadcastArea(player.AreaID) },
		)
	case pickupRequest:
		player := s.players.authenticated(request.id, request.token)
		request.reply <- s.loot.pickup(
			player, func() {
				if player != nil {
					s.broadcastArea(player.AreaID)
				}
			},
		)
	case restorePickupRequest:
		request.reply <- s.loot.restore(
			s.players.authenticated(request.id, request.token),
			request.item, func() { s.broadcastArea(request.item.AreaID) },
		)
	case useConsumableRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- ConsumableResult{}
			return
		}
		result := player.useConsumable(request.definition)
		if result.Applied {
			s.broadcastArea(player.AreaID)
		}
		request.reply <- result
	case updateEquipmentRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil {
			request.reply <- Player{}
			return
		}
		player.setEquipmentStats(request.stats)
		s.broadcastArea(player.AreaID)
		request.reply <- player.Player
	case spendSkillRequest:
		player := s.players.authenticated(request.id, request.token)
		if player == nil || !player.spendSkillPoint(request.skill) {
			request.reply <- Player{}
			return
		}
		s.broadcastArea(player.AreaID)
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
		s.broadcastArea(player.AreaID)
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
		s.broadcastArea(player.AreaID)
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
		previousAreaID := player.AreaID
		player.AreaID = area.ID
		player.X, player.Y = area.Spawn.X, area.Spawn.Y
		sendEvent(player, Event{
			Kind:    EventAdmin,
			Message: fmt.Sprintf("Teleported to %s", area.Name),
		})
		s.broadcastAreas(map[string]bool{
			previousAreaID: true,
			player.AreaID:  true,
		})
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
		previousAreaID := player.AreaID
		player.AreaID = destination.AreaID
		player.X, player.Y = destination.X, destination.Y
		sendEvent(player, Event{
			Kind:    EventAdmin,
			Message: fmt.Sprintf("Teleported to %s", destination.Name),
		})
		s.broadcastAreas(map[string]bool{
			previousAreaID: true,
			player.AreaID:  true,
		})
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
		s.broadcastArea(player.AreaID)
		request.reply <- adminPlayerResult{player: player.Player}
	case adminKickRequest:
		player, err := s.players.kick(request.name, request.reason)
		if err == nil {
			s.broadcastArea(player.AreaID)
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

func (s *runtimeState) broadcastArea(areaID string) {
	s.players.broadcastArea(s.snapshot, areaID)
	delete(s.pendingAreas, areaID)
}

func (s *runtimeState) broadcastAreas(areaIDs map[string]bool) {
	s.players.broadcastAreas(s.snapshot, areaIDs)
	for areaID := range areaIDs {
		delete(s.pendingAreas, areaID)
	}
}
