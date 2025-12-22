//go:build !darwin || !cgo

package calendar

import "time"

// AuthStatus represents the current authorization status
type AuthStatus int

const (
	AuthNotDetermined AuthStatus = iota
	AuthRestricted
	AuthDenied
	AuthAuthorized
)

// GetAuthStatus returns the current calendar authorization status
func GetAuthStatus() AuthStatus {
	return AuthDenied
}

type stubClient struct{}

func NewClient() (Client, error) {
	return nil, ErrNotSupported
}

func (c *stubClient) ListCalendars() ([]Calendar, error) {
	return nil, ErrNotSupported
}

func (c *stubClient) ListEvents(start, end time.Time) ([]Event, error) {
	return nil, ErrNotSupported
}

func (c *stubClient) CreateEvent(event Event) (string, error) {
	return "", ErrNotSupported
}

func (c *stubClient) DeleteEvent(id string) error {
	return ErrNotSupported
}
