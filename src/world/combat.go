package world

type combatSystem struct{}

func (p *activePlayer) attackDamage() int {
	return 1 + p.Attack
}

func (p *activePlayer) takeDamage(rawDamage int) int {
	damage := max(rawDamage-p.Defense, 0)
	previousHealth := p.Health
	p.Health = max(p.Health-damage, 0)
	return previousHealth - p.Health
}

func (p *activePlayer) isWithin(areaID string, x, y, distance int) bool {
	return p.AreaID == areaID &&
		abs(p.X-x) <= distance &&
		abs(p.Y-y) <= distance
}

func (e *Enemy) takeDamage(damage int) bool {
	e.Health -= damage
	return e.Health <= 0
}

func (e *Enemy) isWithinRangeOf(player *activePlayer, distance int) bool {
	return player.isWithin(e.AreaID, e.X, e.Y, distance)
}

func (combatSystem) attack(
	players *playerSystem,
	enemies *enemySystem,
	loot *lootSystem,
	playerID int64,
	token string,
	broadcast func(),
) AttackResult {
	result := AttackResult{}
	player := players.authenticated(playerID, token)
	if player == nil {
		return result
	}
	for _, target := range enemies.live {
		if !target.isWithinRangeOf(player, attackRange) {
			continue
		}
		result.HitIDs = append(result.HitIDs, target.ID)
		if !target.takeDamage(player.attackDamage()) {
			continue
		}
		result.DefeatedIDs = append(result.DefeatedIDs, target.ID)
		sendEvent(player, Event{Kind: EventCombat, Message: "Defeated " + target.Name})
		player.grantExperience(target.Experience)
		combatSystem{}.defeatEnemy(enemies, loot, target)
	}
	if len(result.HitIDs) > 0 {
		broadcast()
	}
	return result
}

func (combatSystem) defeatEnemy(enemies *enemySystem, loot *lootSystem, target *Enemy) {
	if definition, ok := enemies.defs.Enemy(target.DefinitionID); ok {
		loot.rollDrops(target, definition)
	}
	enemies.remove(target)
}
