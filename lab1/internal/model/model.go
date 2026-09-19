package model

// Planet describes a planet and its gravity relative to Earth's gravity.
type Planet struct {
	Name    string
	Gravity float64
}

// PlanetWeight is a calculated weight for one planet.
type PlanetWeight struct {
	Planet Planet
	Weight float64
}

// State is the data displayed by the view.
type State struct {
	HasValue     bool
	EarthWeight  float64
	PlanetWeight []PlanetWeight
}

// Observer receives a notification when the model changes.
type Observer interface {
	ModelUpdated(State)
}

// WeightModel stores the current calculator state and notifies its observers.
// The notification is what makes this an active model in MVC.
type WeightModel struct {
	state     State
	observers []Observer
}

func NewWeightModel() *WeightModel {
	return &WeightModel{}
}

func (m *WeightModel) Subscribe(observer Observer) {
	m.observers = append(m.observers, observer)
}

func (m *WeightModel) State() State {
	return copyState(m.state)
}

// SetState changes the model and actively informs all subscribed views.
func (m *WeightModel) SetState(state State) {
	m.state = copyState(state)
	for _, observer := range m.observers {
		observer.ModelUpdated(copyState(m.state))
	}
}

func copyState(state State) State {
	state.PlanetWeight = append([]PlanetWeight(nil), state.PlanetWeight...)
	return state
}

// Planets returns the gravity constants used by the calculator.
func Planets() []Planet {
	return []Planet{
		{Name: "Меркурий", Gravity: 0.38},
		{Name: "Венера", Gravity: 0.91},
		{Name: "Земля", Gravity: 1.00},
		{Name: "Марс", Gravity: 0.38},
		{Name: "Юпитер", Gravity: 2.36},
		{Name: "Сатурн", Gravity: 0.92},
		{Name: "Уран", Gravity: 0.89},
		{Name: "Нептун", Gravity: 1.12},
	}
}
