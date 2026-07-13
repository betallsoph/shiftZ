package store

import (
	"fmt"
	"regexp"
	"strings"
)

var shiftTimeRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// CreateShiftInput is the owner-provided data for a new shift template.
type CreateShiftInput struct {
	Name      string
	Weekday   int
	StartTime string
	EndTime   string
	MinStaff  int
	MaxStaff  int
}

// ValidateCreateShiftInput checks owner shift input before persistence.
func ValidateCreateShiftInput(input CreateShiftInput) error {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return validationError("tên ca không được để trống")
	}
	if input.Weekday < 0 || input.Weekday > 6 {
		return validationError("thứ trong tuần không hợp lệ (0–6)")
	}
	if !shiftTimeRE.MatchString(input.StartTime) || !shiftTimeRE.MatchString(input.EndTime) {
		return validationError("giờ phải đúng định dạng HH:MM")
	}
	if input.StartTime == input.EndTime {
		return validationError("giờ bắt đầu và kết thúc không được trùng nhau")
	}
	if input.MinStaff < 0 {
		return validationError("min staff không hợp lệ")
	}
	if input.MaxStaff < 1 {
		return validationError("max staff phải >= 1")
	}
	if input.MinStaff > input.MaxStaff {
		return validationError("min staff không được lớn hơn max staff")
	}
	return nil
}

func validationError(msg string) error {
	return fmt.Errorf("%w: %s", ErrValidation, msg)
}

// ValidationMessage returns the user-facing part of a validation error.
func ValidationMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	prefix := ErrValidation.Error() + ": "
	if strings.HasPrefix(msg, prefix) {
		return strings.TrimPrefix(msg, prefix)
	}
	return msg
}
