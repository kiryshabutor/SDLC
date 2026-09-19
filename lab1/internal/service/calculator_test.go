package service

import (
	"math"
	"testing"

	"weight-calculator/internal/model"
)

func TestCalculateUpdatesModel(t *testing.T) {
	weightModel := model.NewWeightModel()
	calculator := NewCalculator(weightModel)

	if err := calculator.Calculate("70,5"); err != nil {
		t.Fatalf("Calculate returned error: %v", err)
	}

	state := calculator.CurrentState()
	if !state.HasValue || state.EarthWeight != 70.5 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if got := state.PlanetWeight[0].Weight; math.Abs(got-26.79) > 0.000001 {
		t.Fatalf("Mercury weight = %v, want 26.79", got)
	}
}

func TestCalculateRejectsInvalidWeight(t *testing.T) {
	weightModel := model.NewWeightModel()
	calculator := NewCalculator(weightModel)

	for _, input := range []string{"", "abc", "0", "-10", "NaN", "+Inf"} {
		if err := calculator.Calculate(input); err == nil {
			t.Errorf("Calculate(%q) returned nil error", input)
		}
	}
	if calculator.CurrentState().HasValue {
		t.Fatal("invalid input changed the model")
	}
}

func TestCalculateExplainsInputErrors(t *testing.T) {
	calculator := NewCalculator(model.NewWeightModel())

	tests := []struct {
		input string
		want  error
	}{
		{"-10", errNegativeWeight},
		{"10.5.2", errMultipleSeparators},
		{"10,5,2", errMultipleSeparators},
		{"10,5.2", errMultipleSeparators},
		{"десять", errLettersInWeight},
		{"10kg", errLettersInWeight},
		{"300.5", errMustBeLessThanMax},
		{"500", errMustBeLessThanMax},
	}

	for _, test := range tests {
		if err := calculator.Calculate(test.input); err != test.want {
			t.Errorf("Calculate(%q) error = %v, want %v", test.input, err, test.want)
		}
	}
}
