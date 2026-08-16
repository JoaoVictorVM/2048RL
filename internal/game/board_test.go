package game

import "testing"

func TestApplyMove_SlidesAndMergesEachDirection(t *testing.T) {
	start := Board{
		{2, 2, 0, 4},
		{0, 4, 4, 0},
		{2, 0, 0, 2},
		{8, 0, 8, 0},
	}

	tests := []struct {
		name string
		dir  Direction
		want Board
	}{
		{
			name: "left",
			dir:  Left,
			want: Board{
				{4, 4, 0, 0},
				{8, 0, 0, 0},
				{4, 0, 0, 0},
				{16, 0, 0, 0},
			},
		},
		{
			name: "right",
			dir:  Right,
			want: Board{
				{0, 0, 4, 4},
				{0, 0, 0, 8},
				{0, 0, 0, 4},
				{0, 0, 0, 16},
			},
		},
		{
			name: "up",
			dir:  Up,
			want: Board{
				{4, 2, 4, 4},
				{8, 4, 8, 2},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
			},
		},
		{
			name: "down",
			dir:  Down,
			want: Board{
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{4, 2, 4, 4},
				{8, 4, 8, 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, moved := applyMoveToBoard(start, tt.dir)
			if !moved {
				t.Fatalf("expected move %s to change the board", tt.dir)
			}
			if got != tt.want {
				t.Errorf("move %s:\ngot  %v\nwant %v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestApplyMove_SingleMergePerTilePerMove(t *testing.T) {
	start := Board{
		{2, 2, 4, 4},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	want := [Size]int{4, 8, 0, 0}

	got, _, moved := applyMoveToBoard(start, Left)
	if !moved {
		t.Fatal("expected the board to change")
	}
	if got[0] != want {
		t.Errorf("got %v, want %v (no tile may merge twice in one move)", got[0], want)
	}
}

func TestApplyMove_ScoreAccumulatesMergedValues(t *testing.T) {
	start := Board{
		{2, 2, 4, 4},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}

	_, gained, _ := applyMoveToBoard(start, Left)
	if want := 4 + 8; gained != want {
		t.Errorf("score gained = %d, want %d", gained, want)
	}
}

func TestApplyMove_NoOpMoveDetected(t *testing.T) {
	start := Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}

	got, gained, moved := applyMoveToBoard(start, Left)
	if moved {
		t.Error("expected the move to be reported as a no-op")
	}
	if gained != 0 {
		t.Errorf("score gained = %d, want 0", gained)
	}
	if got != start {
		t.Error("board changed on a no-op move")
	}
}
