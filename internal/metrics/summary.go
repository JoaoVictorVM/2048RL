package metrics

type Summary struct {
	Episodes   int
	AvgScore   float64
	AvgMaxTile float64
	WinRate    float64
}

func Summarize(records []Record) Summary {
	if len(records) == 0 {
		return Summary{}
	}

	totalScore, totalMaxTile, wins := 0, 0, 0
	for _, record := range records {
		totalScore += record.Score
		totalMaxTile += record.MaxTile
		if record.Won {
			wins++
		}
	}

	count := float64(len(records))
	return Summary{
		Episodes:   len(records),
		AvgScore:   float64(totalScore) / count,
		AvgMaxTile: float64(totalMaxTile) / count,
		WinRate:    float64(wins) / count,
	}
}
