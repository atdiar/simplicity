// Package ui is a library of functions for simple, generic gui development.
package ui

type Event interface {
	Type() string
	Target() *Element
	CurrentTarget() *Element

	PreventDefault()
	StopPropagation()          // the phase is stil 1,2,or 3 but Stopped returns true
	StopImmediatePropagation() // sets the Phase to 0 and Stopped to true
	SetPhase(int)
	SetCurrentTarget(*Element)

	Phase() int
	Bubbles() bool
	DefaultPrevented() bool
	Stopped() bool

	Native() interface{} // returns the native event object
}

type EventListeners struct {
	list map[string]*eventHandlers
}

func NewEventListenerStore() EventListeners {
	return EventListeners{make(map[string]*eventHandlers)}
}

func (e EventListeners) AddEventListener(event string, handler *EventHandler) {
	eh, ok := e.list[event]
	if !ok {
		e.list[event] = newEventHandlers().Add(handler)
	}
	eh.Add(handler)
}

func (e EventListeners) RemoveEventListener(event string, handler *EventHandler) {
	eh, ok := e.list[event]
	if !ok {
		return
	}
	eh.Remove(handler)
}

func (e EventListeners) Handle(evt Event) bool {
	evh, ok := e.list[evt.Type()]
	if !ok {
		return true
	}
	switch evt.Phase() {
	// capture
	case 0:
		return true
	case 1:
		for _, h := range evh.List {
			if !h.Capture {
				continue
			}
			done := h.Handle(evt)
			if done {
				return done
			}
			if evt.Stopped() && (evt.Phase() == 0) {
				return true
			}
		}
	case 2:
		for _, h := range evh.List {
			done := h.Handle(evt)
			if done {
				return done
			}
		}
	case 3:
		if !evt.Bubbles() {
			return true
		}
		for _, h := range evh.List {
			if h.Capture {
				continue
			}
			done := h.Handle(evt)
			if done {
				return done
			}
			if evt.Stopped() && (evt.Phase() == 0) {
				return true
			}
		}
	}
	return false // not supposed to be reached anyway
}

type eventHandlers struct {
	List []*EventHandler
}

func newEventHandlers() *eventHandlers {
	return &eventHandlers{make([]*EventHandler, 0)}
}

func (e *eventHandlers) Add(h *EventHandler) *eventHandlers {
	e.List = append(e.List, h)
	return e
}

func (e *eventHandlers) Remove(h *EventHandler) *eventHandlers {
	var index int
	for k, v := range e.List {
		if v != h {
			continue
		}
		index = k
		break
	}
	e.List = append(e.List[:index], e.List[index+1:]...)
	return e
}

type EventHandler struct {
	Fn      func(Event) bool
	Capture bool // propagation mode: if false bubbles up, otherwise captured by the top most element and propagate down .

	Once bool
}

func (e EventHandler) Handle(evt Event) bool {
	return e.Fn(evt)
}

func NewEventHandler(fn func(Event) bool) *EventHandler {
	return &EventHandler{fn, false, false}
}
func (e *EventHandler) ForCapture() *EventHandler {
	e.Capture = true
	return e
}

func (e *EventHandler) TriggerOnce() *EventHandler {
	e.Once = true
	return e
}