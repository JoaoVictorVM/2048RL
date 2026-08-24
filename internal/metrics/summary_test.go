package metrics

import "testing"

func TestSummarize(t *testing.T) {
	if got := Summarize(nil); got != (Summary{}) {
		t.Errorf("resumo de fatia vazia deveria ser zero, obtido %+v", got)
	}

	got := Summarize([]Record{
		{Score: 100, MaxTile: 128, Won: false},
		{Score: 300, MaxTile: 256, Won: true},
		{Score: 200, MaxTile: 512, Won: true},
	})

	if got.Episodes != 3 {
		t.Errorf("episódios %d, esperado 3", got.Episodes)
	}
	if got.AvgScore != 200 {
		t.Errorf("score médio %v, esperado 200", got.AvgScore)
	}
	if want := (128.0 + 256 + 512) / 3; got.AvgMaxTile != want {
		t.Errorf("max tile médio %v, esperado %v", got.AvgMaxTile, want)
	}
	if want := 2.0 / 3; got.WinRate != want {
		t.Errorf("win rate %v, esperado %v", got.WinRate, want)
	}
}
