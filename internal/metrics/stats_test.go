package metrics

import "testing"

func TestFloat64sSum(t *testing.T) {
	tests := []struct {
		name   string
		values Float64s
		want   float64
	}{
		{"empty", nil, 0},
		{"single", Float64s{42}, 42},
		{"several", Float64s{10, 20, 30}, 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.values.Sum(); got != tt.want {
				t.Errorf("Float64s%v.Sum() = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestFloat64sMean(t *testing.T) {
	tests := []struct {
		name   string
		values Float64s
		want   float64
	}{
		{"empty", nil, 0},
		{"single", Float64s{42}, 42},
		{"several", Float64s{10, 20, 30}, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.values.Mean(); got != tt.want {
				t.Errorf("Float64s%v.Mean() = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}
