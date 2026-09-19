package view

import (
	"fmt"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"weight-calculator/internal/controller"
	"weight-calculator/internal/model"
)

const (
	applicationID      = "by.bsuir.weight-calculator"
	lastEarthWeightKey = "last_earth_weight"
)

// MainWindow is the main screen and an observer of the active model.
type MainWindow struct {
	app        fyne.App
	window     fyne.Window
	controller *controller.WeightController

	earthWeightLabel *widget.Label
	results          *fyne.Container
	state            model.State
}

func newMainWindow(weightModel *model.WeightModel, weightController *controller.WeightController) *MainWindow {
	mainView := &MainWindow{
		app:              app.NewWithID(applicationID),
		controller:       weightController,
		earthWeightLabel: widget.NewLabel("Вес на Земле: —"),
		results:          container.NewVBox(widget.NewLabel("Сначала введите вес на Земле.")),
	}
	mainView.window = mainView.app.NewWindow("Калькулятор веса на планетах")
	weightModel.Subscribe(mainView)
	mainView.restoreLastWeight()

	mainView.window.Resize(fyne.NewSize(520, 520))
	mainView.window.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("Вариант 4", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Калькулятор веса на планетах солнечной системы"),
		widget.NewButton("Ввести данные", mainView.openInputWindow),
		mainView.earthWeightLabel,
		widget.NewSeparator(),
		mainView.results,
	))
	return mainView
}

// restoreLastWeight loads the last valid value saved by Fyne preferences.
func (v *MainWindow) restoreLastWeight() {
	weight := v.app.Preferences().FloatWithFallback(lastEarthWeightKey, 0)
	if weight > 0 && !math.IsNaN(weight) && !math.IsInf(weight, 0) {
		v.state = model.State{HasValue: true, EarthWeight: weight}
	}
}

// ModelUpdated is called by WeightModel after a successful calculation.
func (v *MainWindow) ModelUpdated(state model.State) {
	v.state = state
	if !state.HasValue {
		return
	}

	v.earthWeightLabel.SetText(fmt.Sprintf("Вес на Земле: %.2f кг", state.EarthWeight))
	v.results.RemoveAll()
	for _, result := range state.PlanetWeight {
		v.results.Add(widget.NewLabel(fmt.Sprintf("%s: %.2f кг", result.Planet.Name, result.Weight)))
	}
	v.results.Refresh()
}

func (v *MainWindow) openInputWindow() {
	newInputWindow(v.app, v.controller, v.state)
}

// Run creates and displays the application window.
func Run(weightModel *model.WeightModel, weightController *controller.WeightController) {
	mainView := newMainWindow(weightModel, weightController)
	mainView.window.ShowAndRun()
}
