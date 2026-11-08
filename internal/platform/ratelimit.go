package platform

import (
	"sync"
	"time"
)

// Window — ограничитель «не более N запросов за любые W» со скользящим окном
// (бюджет запросов к Steam, ADR-0006, DV-012). Потокобезопасен.
type Window struct {
	mu    sync.Mutex
	limit int
	span  time.Duration
	now   func() time.Time
	times []time.Time
	pause time.Time
}

// NewWindow создаёт ограничитель на limit запросов за span.
func NewWindow(limit int, span time.Duration) *Window {
	return &Window{limit: limit, span: span, now: time.Now}
}

// Allow занимает место в окне, если оно есть. Используется API-сервером:
// при исчерпании бюджета пользователь сразу получает 503, а не ждёт.
func (w *Window) Allow() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	if now.Before(w.pause) {
		return false
	}
	cut := now.Add(-w.span)
	i := 0
	for i < len(w.times) && !w.times[i].After(cut) {
		i++
	}
	w.times = w.times[i:]
	if len(w.times) >= w.limit {
		return false
	}
	w.times = append(w.times, now)
	return true
}

// Pause запрещает запросы на d (после ответа 429/403 Steam, DV-012 п. 3).
func (w *Window) Pause(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if until := w.now().Add(d); until.After(w.pause) {
		w.pause = until
	}
}
