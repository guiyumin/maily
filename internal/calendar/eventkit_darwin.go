//go:build darwin && cgo

package calendar

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework EventKit -framework AppKit
#include "eventkit_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"time"
	"unsafe"
)

type eventKitClient struct{}

// AuthStatus represents the current authorization status
type AuthStatus int

const (
	AuthNotDetermined AuthStatus = iota
	AuthRestricted
	AuthDenied
	AuthAuthorized
)

// GetAuthStatus returns the current calendar authorization status without prompting
func GetAuthStatus() AuthStatus {
	status := C.GetAuthorizationStatus()
	switch status {
	case C.EK_AUTH_NOT_DETERMINED:
		return AuthNotDetermined
	case C.EK_AUTH_RESTRICTED:
		return AuthRestricted
	case C.EK_AUTH_DENIED:
		return AuthDenied
	case C.EK_AUTH_AUTHORIZED:
		return AuthAuthorized
	default:
		return AuthDenied
	}
}

// NewClient creates a new EventKit-based calendar client
func NewClient() (Client, error) {
	result := C.RequestCalendarAccess()
	if result != C.EK_SUCCESS {
		return nil, ErrAccessDenied
	}
	return &eventKitClient{}, nil
}

func (c *eventKitClient) ListCalendars() ([]Calendar, error) {
	cStr := C.ListCalendars()
	if cStr == nil {
		return nil, ErrFailed
	}
	defer C.FreeString(cStr)

	jsonStr := C.GoString(cStr)

	var rawCalendars []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Color string `json:"color"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawCalendars); err != nil {
		return nil, err
	}

	calendars := make([]Calendar, len(rawCalendars))
	for i, rc := range rawCalendars {
		calendars[i] = Calendar{
			ID:    rc.ID,
			Title: rc.Title,
			Color: rc.Color,
		}
	}

	return calendars, nil
}

func (c *eventKitClient) ListEvents(start, end time.Time) ([]Event, error) {
	startTs := C.longlong(start.Unix())
	endTs := C.longlong(end.Unix())

	cStr := C.ListEvents(startTs, endTs)
	if cStr == nil {
		return nil, ErrFailed
	}
	defer C.FreeString(cStr)

	jsonStr := C.GoString(cStr)

	var rawEvents []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		StartTime  int64  `json:"startTime"`
		EndTime    int64  `json:"endTime"`
		Location   string `json:"location"`
		Notes      string `json:"notes"`
		Calendar   string `json:"calendar"`
		CalendarID string `json:"calendarID"`
		AllDay     bool   `json:"allDay"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawEvents); err != nil {
		return nil, err
	}

	events := make([]Event, len(rawEvents))
	for i, re := range rawEvents {
		events[i] = Event{
			ID:        re.ID,
			Title:     re.Title,
			StartTime: time.Unix(re.StartTime, 0),
			EndTime:   time.Unix(re.EndTime, 0),
			Location:  re.Location,
			Notes:     re.Notes,
			Calendar:  re.Calendar,
			AllDay:    re.AllDay,
		}
	}

	return events, nil
}

func (c *eventKitClient) CreateEvent(event Event) (string, error) {
	cTitle := C.CString(event.Title)
	defer C.free(unsafe.Pointer(cTitle))

	cCalendarID := C.CString(event.Calendar)
	defer C.free(unsafe.Pointer(cCalendarID))

	cLocation := C.CString(event.Location)
	defer C.free(unsafe.Pointer(cLocation))

	cNotes := C.CString(event.Notes)
	defer C.free(unsafe.Pointer(cNotes))

	allDay := C.int(0)
	if event.AllDay {
		allDay = C.int(1)
	}

	cEventID := C.CreateEvent(
		cTitle,
		C.longlong(event.StartTime.Unix()),
		C.longlong(event.EndTime.Unix()),
		cCalendarID,
		cLocation,
		cNotes,
		allDay,
	)

	if cEventID == nil {
		return "", ErrFailed
	}
	defer C.FreeString(cEventID)

	return C.GoString(cEventID), nil
}

func (c *eventKitClient) DeleteEvent(id string) error {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	result := C.DeleteEvent(cID)

	switch result {
	case C.EK_SUCCESS:
		return nil
	case C.EK_ERROR_ACCESS_DENIED:
		return ErrAccessDenied
	case C.EK_ERROR_NOT_FOUND:
		return ErrNotFound
	default:
		return ErrFailed
	}
}
