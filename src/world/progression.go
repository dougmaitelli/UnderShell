package world

import (
	"fmt"
	"math"
)

func (p *activePlayer) grantExperience(reward int64) {
	sendEvent(p, Event{
		Kind: EventProgression, Message: fmt.Sprintf("Gained %d XP", reward),
	})
	previousLevel := p.Level
	grantExperience(&p.Player, reward)
	for level := previousLevel + 1; level <= p.Level; level++ {
		sendEvent(p, Event{
			Kind: EventProgression, Message: fmt.Sprintf("Level up! Reached level %d", level),
		})
		sendEvent(p, Event{
			Kind: EventProgression, Message: "Gained 1 skill point",
		})
	}
}

func (p *activePlayer) spendSkillPoint(skill string) bool {
	if p.SkillPoints < 1 {
		return false
	}
	switch skill {
	case "attack":
		p.Attack++
	case "defense":
		p.Defense++
	case "vitality":
		p.Vitality++
		p.MaxHealth += vitalityHealthPerRank
		p.Health += vitalityHealthPerRank
	default:
		return false
	}
	p.SkillPoints--
	return true
}

func (p *activePlayer) grantLevels(levels int) {
	if levels <= 0 {
		return
	}
	if p.Level < 1 {
		p.Level = 1
	}
	if levels > math.MaxInt-p.Level {
		levels = math.MaxInt - p.Level
	}
	if levels > math.MaxInt-p.SkillPoints {
		levels = math.MaxInt - p.SkillPoints
	}
	p.Level += levels
	p.SkillPoints += levels
}

// ExperienceToNextLevel returns the XP required to advance from level to level+1.
// The quadratic requirement is 100*level² and saturates only at integer capacity.
func ExperienceToNextLevel(level int) int64 {
	const maxExperience = int64(1<<63 - 1)
	if level < 1 {
		level = 1
	}
	const largestSafeLevel = 303700049
	if level > largestSafeLevel {
		return maxExperience
	}
	value := int64(level)
	return 100 * value * value
}

func grantExperience(player *Player, reward int64) {
	const maxExperience = int64(1<<63 - 1)
	if reward <= 0 {
		return
	}
	if player.Level < 1 {
		player.Level = 1
	}
	if reward > maxExperience-player.Experience {
		player.Experience = maxExperience
	} else {
		player.Experience += reward
	}
	for {
		requirement := ExperienceToNextLevel(player.Level)
		if player.Experience < requirement {
			return
		}
		player.Experience -= requirement
		player.Level++
		player.SkillPoints++
	}
}
