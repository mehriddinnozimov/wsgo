package eventemitter

import (
	"errors"
	"fmt"
)

type Listener func(args ...interface{})

type EventEmitter struct {
	events map[string][]Listener
}

func (ee *EventEmitter) On(key string, _listeners ...Listener) {
	listeners, ok := ee.events[key]
	if ok {
		ee.events[key] = append(listeners, _listeners...)
	} else {
		ee.events[key] = _listeners
	}
}

func (ee *EventEmitter) indexOf(listeners []Listener, _listener Listener) int {
	_pointer := fmt.Sprintf("%v", _listener)

	for index, listener := range listeners {
		pointer := fmt.Sprintf("%v", listener)
		if pointer == _pointer {
			return index
		}
	}

	return -1
}

func (ee *EventEmitter) Clear() {
	for k := range ee.events {
		delete(ee.events, k)
	}
}

func (ee *EventEmitter) Off(key string, _listener Listener) error {
	listeners, err := ee.listeners(key)
	if err != nil {
		return err
	}

	index := ee.indexOf(listeners, _listener)
	if index == -1 {
		return errors.New("No listener found")
	}

	listeners = append(listeners[:index], listeners[index+1:]...)
	ee.events[key] = listeners

	return nil

}

func (ee *EventEmitter) listeners(key string) ([]Listener, error) {
	listeners, ok := ee.events[key]
	if ok && len(listeners) > 0 {
		return listeners, nil
	} else {
		return listeners, errors.New("Not listener found")
	}
}

func (ee *EventEmitter) Emit(key string, _args ...interface{}) error {
	listeners, err := ee.listeners(key)
	if err != nil {
		return err
	}

	for _, listener := range listeners {
		listener(_args...)
	}

	return nil
}

func New() EventEmitter {
	ee := EventEmitter{make(map[string][]Listener)}
	return ee
}
