package manifest

import (
	"slices"
	"testing"
)

func TestSlotsComeInTheOrderOfTheirNumbers(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[int]Player
		want      []int
	}{
		{"ten after two", map[int]Player{10: {}, 2: {}, 0: {}}, []int{0, 2, 10}},
		{"every slot", map[int]Player{23: {}, 9: {}, 19: {}, 1: {}, 20: {}}, []int{1, 9, 19, 20, 23}},
		{"none", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SortedSlots(tt.overrides); !slices.Equal(got, tt.want) {
				t.Errorf("Slots = %v, want %v", got, tt.want)
			}
		})
	}
}
