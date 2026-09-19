package service

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"

	"weight-calculator/internal/model"
)

var (
	errEmptyWeight        = errors.New("введите вес")
	errNegativeWeight     = errors.New("вес не может быть отрицательным")
	errMultipleSeparators = errors.New("в числе может быть не более одной точки или запятой")
	errLettersInWeight    = errors.New("вес не должен содержать буквы")
	errInvalidWeight      = errors.New("введите положительное число, например 70")
	errNonPositiveWeight  = errors.New("вес должен быть больше нуля")
	errMustBeLessThanMax  = errors.New("вес не может превышать 300 кг")
)

// maxWeightKg is the maximum allowed human weight in kilograms.
const maxWeightKg = 300

// Calculator contains the application's business logic.
type Calculator struct {
	model *model.WeightModel
}

func NewCalculator(weightModel *model.WeightModel) *Calculator {
	return &Calculator{model: weightModel}
}

// Calculate validates the text from the input field and updates the model.
func (c *Calculator) Calculate(rawWeight string) error {
	value := strings.TrimSpace(rawWeight)
	if value == "" {
		return errEmptyWeight
	}
	if strings.HasPrefix(value, "-") {
		return errNegativeWeight
	}
	if strings.Count(value, ".")+strings.Count(value, ",") > 1 {
		return errMultipleSeparators
	}
	if strings.IndexFunc(value, unicode.IsLetter) >= 0 {
		return errLettersInWeight
	}

	value = strings.ReplaceAll(value, ",", ".")
	weight, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(weight) || math.IsInf(weight, 0) {
		return errInvalidWeight
	}
	if weight < 0 {
		return errNegativeWeight
	}
	if weight == 0 {
		return errNonPositiveWeight
	}
	if weight > maxWeightKg {
		return errMustBeLessThanMax
	}

	planetWeights := make([]model.PlanetWeight, 0, len(model.Planets()))
	for _, planet := range model.Planets() {
		planetWeights = append(planetWeights, model.PlanetWeight{
			Planet: planet,
			Weight: weight * planet.Gravity,
		})
	}

	c.model.SetState(model.State{
		HasValue:     true,
		EarthWeight:  weight,
		PlanetWeight: planetWeights,
	})
	return nil
}

func (c *Calculator) CurrentState() model.State {
	return c.model.State()
}
