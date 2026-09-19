package view

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"weight-calculator/internal/controller"
	"weight-calculator/internal/model"
)

func newInputWindow(
	a fyne.App,
	weightController *controller.WeightController,
	state model.State,
) {
	inputWindow := a.NewWindow("Ввод веса")
	inputWindow.Resize(fyne.NewSize(420, 190))

	weightEntry := widget.NewEntry()
	weightEntry.SetPlaceHolder("Например, 70")
	if state.HasValue {
		weightEntry.SetText(formatWeight(state.EarthWeight))
	}

	calculateButton := widget.NewButton("Рассчитать", func() {
		if err := weightController.Calculate(weightEntry.Text); err != nil {
			dialog.ShowError(err, inputWindow)
			return
		}
		a.Preferences().SetFloat(lastEarthWeightKey, weightController.CurrentState().EarthWeight)
		inputWindow.Close()
	})

	cancelButton := widget.NewButton("Отмена", inputWindow.Close)
	inputWindow.SetContent(container.NewVBox(
		widget.NewLabel("Введите вес человека на Земле (кг):"),
		weightEntry,
		container.NewHBox(calculateButton, cancelButton),
	))
	inputWindow.Show()
}
