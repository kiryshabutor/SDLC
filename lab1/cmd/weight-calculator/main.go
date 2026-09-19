package main

import (
	"weight-calculator/internal/controller"
	"weight-calculator/internal/model"
	"weight-calculator/internal/service"
	"weight-calculator/internal/view"
)

func main() {
	weightModel := model.NewWeightModel()
	calculator := service.NewCalculator(weightModel)
	weightController := controller.NewWeightController(calculator)

	view.Run(weightModel, weightController)
}
