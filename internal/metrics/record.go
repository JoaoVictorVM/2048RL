package metrics

const (
	DirName  = "metrics"
	FileName = "episodes.jsonl"
)

type Record struct {
	Episode int  `json:"episode"`
	Score   int  `json:"score"`
	MaxTile int  `json:"max_tile"`
	Won     bool `json:"won"`
	Moves   int  `json:"moves"`
}
