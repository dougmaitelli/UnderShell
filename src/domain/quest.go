package domain

type QuestStatus string

const (
	QuestActive    QuestStatus = "active"
	QuestCompleted QuestStatus = "completed"
)

type CharacterQuest struct {
	QuestID     string
	GiverID     string
	Status      QuestStatus
	AcceptedAt  string
	CompletedAt string
}
