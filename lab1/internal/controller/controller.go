package controller

import (
	"weight-calculator/internal/model"
	"weight-calculator/internal/service"
)

// WeightController handles user actions and delegates business work to the service.
type WeightController struct {
	calculator *service.Calculator
}

func NewWeightController(calculator *service.Calculator) *WeightController {
	return &WeightController{calculator: calculator}
}

func (c *WeightController) Calculate(rawWeight string) error {
	return c.calculator.Calculate(rawWeight)
}

func (c *WeightController) CurrentState() model.State {
	return c.calculator.CurrentState()
}
