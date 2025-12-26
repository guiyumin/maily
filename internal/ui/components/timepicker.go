package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TimePickerField represents which field is currently focused
type TimePickerField int

const (
	FieldHour TimePickerField = iota
	FieldMinute
	FieldPeriod
)

// TimePicker is a scrollable time picker component
type TimePicker struct {
	hour       int             // 1-12
	minute     int             // 0-59
	isPM       bool            // AM = false, PM = true
	focused    bool            // whether the picker is focused
	focusField TimePickerField // which field is focused (hour, minute, period)
}

// NewTimePicker creates a new time picker with default time (9:00 AM)
func NewTimePicker() TimePicker {
	return TimePicker{
		hour:       9,
		minute:     0,
		isPM:       false,
		focused:    false,
		focusField: FieldHour,
	}
}

// Focus sets the picker as focused
func (t *TimePicker) Focus() {
	t.focused = true
}

// Blur removes focus from the picker
func (t *TimePicker) Blur() {
	t.focused = false
}

// Focused returns whether the picker is focused
func (t TimePicker) Focused() bool {
	return t.focused
}

// SetTime24 sets the time using 24-hour format (e.g., "15:04" or "09:00")
func (t *TimePicker) SetTime24(timeStr string) error {
	parsed, err := time.Parse("15:04", timeStr)
	if err != nil {
		return err
	}

	hour24 := parsed.Hour()
	t.minute = parsed.Minute()

	if hour24 == 0 {
		t.hour = 12
		t.isPM = false
	} else if hour24 < 12 {
		t.hour = hour24
		t.isPM = false
	} else if hour24 == 12 {
		t.hour = 12
		t.isPM = true
	} else {
		t.hour = hour24 - 12
		t.isPM = true
	}

	return nil
}

// Value24 returns the time in 24-hour format (e.g., "15:04")
func (t TimePicker) Value24() string {
	hour24 := t.hour
	if t.isPM && t.hour != 12 {
		hour24 = t.hour + 12
	} else if !t.isPM && t.hour == 12 {
		hour24 = 0
	}
	return fmt.Sprintf("%02d:%02d", hour24, t.minute)
}

// Value12 returns the time in 12-hour format (e.g., "3:04 PM")
func (t TimePicker) Value12() string {
	period := "AM"
	if t.isPM {
		period = "PM"
	}
	return fmt.Sprintf("%d:%02d %s", t.hour, t.minute, period)
}

// Update handles key messages for the time picker
func (t TimePicker) Update(msg tea.Msg) (TimePicker, tea.Cmd) {
	if !t.focused {
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			t.increment()
		case "down", "j":
			t.decrement()
		case "left", "h":
			t.prevField()
		case "right", "l":
			t.nextField()
		}
	}

	return t, nil
}

func (t *TimePicker) increment() {
	switch t.focusField {
	case FieldHour:
		t.hour++
		if t.hour > 12 {
			t.hour = 1
		}
	case FieldMinute:
		t.minute += 5 // 5-minute increments for easier scrolling
		if t.minute >= 60 {
			t.minute = 0
		}
	case FieldPeriod:
		t.isPM = !t.isPM
	}
}

func (t *TimePicker) decrement() {
	switch t.focusField {
	case FieldHour:
		t.hour--
		if t.hour < 1 {
			t.hour = 12
		}
	case FieldMinute:
		t.minute -= 5
		if t.minute < 0 {
			t.minute = 55
		}
	case FieldPeriod:
		t.isPM = !t.isPM
	}
}

func (t *TimePicker) nextField() {
	t.focusField = (t.focusField + 1) % 3
}

func (t *TimePicker) prevField() {
	if t.focusField == 0 {
		t.focusField = FieldPeriod
	} else {
		t.focusField--
	}
}

// View renders the time picker
func (t TimePicker) View() string {
	// Styles
	normalStyle := lipgloss.NewStyle().Foreground(Text)
	focusedStyle := lipgloss.NewStyle().Foreground(Text).Background(Primary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(Muted)

	// Format parts
	hourStr := fmt.Sprintf("%2d", t.hour)
	minuteStr := fmt.Sprintf("%02d", t.minute)
	period := "AM"
	if t.isPM {
		period = "PM"
	}

	var hourView, minuteView, periodView string

	if t.focused {
		// Show focused field with highlight
		if t.focusField == FieldHour {
			hourView = focusedStyle.Render(hourStr)
		} else {
			hourView = normalStyle.Render(hourStr)
		}

		if t.focusField == FieldMinute {
			minuteView = focusedStyle.Render(minuteStr)
		} else {
			minuteView = normalStyle.Render(minuteStr)
		}

		if t.focusField == FieldPeriod {
			periodView = focusedStyle.Render(period)
		} else {
			periodView = normalStyle.Render(period)
		}
	} else {
		// Not focused - show all dim
		hourView = dimStyle.Render(hourStr)
		minuteView = dimStyle.Render(minuteStr)
		periodView = dimStyle.Render(period)
	}

	return hourView + normalStyle.Render(":") + minuteView + " " + periodView
}

// ViewCompact renders a compact view (just the time, no interactive hints)
func (t TimePicker) ViewCompact() string {
	period := "AM"
	if t.isPM {
		period = "PM"
	}
	return fmt.Sprintf("%d:%02d %s", t.hour, t.minute, period)
}
