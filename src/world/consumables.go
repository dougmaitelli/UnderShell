package world

import "sshrpg/src/item"

func (p *activePlayer) useConsumable(
	definition *item.Definition,
) ConsumableResult {
	result := ConsumableResult{Player: p.Player}
	if definition == nil || definition.Type != item.TypeConsumable {
		return result
	}
	for _, effect := range definition.Effects {
		switch effect.Type {
		case item.EffectRestoreHealth:
			previous := p.Health
			p.Health = min(p.Health+effect.Amount, p.MaxHealth)
			result.HealthRestored += p.Health - previous
		}
	}
	result.Applied = result.HealthRestored > 0
	result.Player = p.Player
	return result
}
