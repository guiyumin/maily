package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DatePickerField represents which field is currently focused
type DatePickerField int

const (
	FieldYear DatePickerField = iota
	FieldMonth
	FieldDay
)

// DatePicker is a scrollable date picker component
type DatePicker struct {
	year       int
	month      int // 1-12
	day        int // 1-31
	focused    bool
	focusField DatePickerField
}

// NewDatePicker creates a new date picker with today's date
func NewDatePicker() DatePicker {
	now := time.Now()
	return DatePicker{
		year:       now.Year(),
		month:      int(now.Month()),
		day:        now.Day(),
		focused:    false,
		focusField: FieldMonth, // Start on month as it's most commonly changed
	}
}

// Focus sets the picker as focused
func (d *DatePicker) Focus() {
	d.focused = true
}

// Blur removes focus from the picker
func (d *DatePicker) Blur() {
	d.focused = false
}

// Focused returns whether the picker is focused
func (d DatePicker) Focused() bool {
	return d.focused
}

// SetDate sets the date from a time.Time
func (d *DatePicker) SetDate(t time.Time) {
	d.year = t.Year()
	d.month = int(t.Month())
	d.day = t.Day()
}

// SetDateString sets the date from a string in "2006-01-02" format
func (d *DatePicker) SetDateString(dateStr string) error {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return err
	}
	d.SetDate(t)
	return nil
}

// Value returns the date as a time.Time
func (d DatePicker) Value() time.Time {
	return time.Date(d.year, time.Month(d.month), d.day, 0, 0, 0, 0, time.Local)
}

// ValueString returns the date in "2006-01-02" format
func (d DatePicker) ValueString() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}

// daysInMonth returns the number of days in the current month
func (d DatePicker) daysInMonth() int {
	// Create a date for the first day of the next month, then subtract one day
	t := time.Date(d.year, time.Month(d.month)+1, 0, 0, 0, 0, 0, time.Local)
	return t.Day()
}

// clampDay ensures the day is valid for the current month
func (d *DatePicker) clampDay() {
	maxDay := d.daysInMonth()
	if d.day > maxDay {
		d.day = maxDay
	}
	if d.day < 1 {
		d.day = 1
	}
}

// Update handles key messages for the date picker
func (d DatePicker) Update(msg tea.Msg) (DatePicker, tea.Cmd) {
	if !d.focused {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			d.increment()
		case "down", "j":
			d.decrement()
		case "left", "h":
			d.prevField()
		case "right", "l":
			d.nextField()
		}
	}

	return d, nil
}

func (d *DatePicker) increment() {
	switch d.focusField {
	case FieldYear:
		d.year++
		d.clampDay()
	case FieldMonth:
		d.month++
		if d.month > 12 {
			d.month = 1
			d.year++
		}
		d.clampDay()
	case FieldDay:
		d.day++
		if d.day > d.daysInMonth() {
			d.day = 1
		}
	}
}

func (d *DatePicker) decrement() {
	switch d.focusField {
	case FieldYear:
		d.year--
		if d.year < 1900 {
			d.year = 1900
		}
		d.clampDay()
	case FieldMonth:
		d.month--
		if d.month < 1 {
			d.month = 12
			d.year--
		}
		d.clampDay()
	case FieldDay:
		d.day--
		if d.day < 1 {
			d.day = d.daysInMonth()
		}
	}
}

func (d *DatePicker) nextField() {
	d.focusField = (d.focusField + 1) % 3
}

func (d *DatePicker) prevField() {
	if d.focusField == 0 {
		d.focusField = FieldDay
	} else {
		d.focusField--
	}
}

// View renders the date picker
func (d DatePicker) View() string {
	// Styles
	normalStyle := lipgloss.NewStyle().Foreground(Text)
	focusedStyle := lipgloss.NewStyle().Foreground(Text).Background(Primary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(Muted)

	// Format parts
	monthNames := []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	monthStr := monthNames[d.month]
	dayStr := fmt.Sprintf("%2d", d.day)
	yearStr := fmt.Sprintf("%d", d.year)

	var monthView, dayView, yearView string

	if d.focused {
		if d.focusField == FieldMonth {
			monthView = focusedStyle.Render(monthStr)
		} else {
			monthView = normalStyle.Render(monthStr)
		}

		if d.focusField == FieldDay {
			dayView = focusedStyle.Render(dayStr)
		} else {
			dayView = normalStyle.Render(dayStr)
		}

		if d.focusField == FieldYear {
			yearView = focusedStyle.Render(yearStr)
		} else {
			yearView = normalStyle.Render(yearStr)
		}
	} else {
		monthView = dimStyle.Render(monthStr)
		dayView = dimStyle.Render(dayStr)
		yearView = dimStyle.Render(yearStr)
	}

	// Format as "Jan 15, 2025"
	return monthView + " " + dayView + normalStyle.Render(", ") + yearView
}

// ViewCompact renders a compact view
func (d DatePicker) ViewCompact() string {
	monthNames := []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	return fmt.Sprintf("%s %d, %d", monthNames[d.month], d.day, d.year)
}
