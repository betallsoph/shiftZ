package store

import (
	"strings"
	"unicode/utf8"
)

// UpdateEmployeeInput is owner-editable employee profile data.
type UpdateEmployeeInput struct {
	DisplayName     string
	Role            string
	MaxHoursPerWeek float64
}

// ValidateUpdateEmployeeInput checks owner employee profile updates.
func ValidateUpdateEmployeeInput(input UpdateEmployeeInput) error {
	name := strings.TrimSpace(input.DisplayName)
	if name == "" {
		return validationError("tên hiển thị không được để trống")
	}
	if utf8.RuneCountInString(name) > 100 {
		return validationError("tên hiển thị tối đa 100 ký tự")
	}
	if utf8.RuneCountInString(strings.TrimSpace(input.Role)) > 50 {
		return validationError("vai trò tối đa 50 ký tự")
	}
	if input.MaxHoursPerWeek <= 0 || input.MaxHoursPerWeek > 168 {
		return validationError("giới hạn giờ/tuần phải lớn hơn 0 và tối đa 168")
	}
	return nil
}
