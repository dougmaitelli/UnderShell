package world

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
