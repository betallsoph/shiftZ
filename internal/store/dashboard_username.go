package store

import (
	"fmt"
	"regexp"
	"strings"
)

var dashboardUsernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// ValidPlans lists supported SaaS plan tiers (metadata only in v0).
var ValidPlans = map[string]struct{}{
	"free":    {},
	"starter": {},
	"pro":     {},
}

// NormalizeDashboardUsername lowercases and trims a dashboard username.
func NormalizeDashboardUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// ValidateDashboardUsername checks username format after normalization.
func ValidateDashboardUsername(username string) error {
	norm := NormalizeDashboardUsername(username)
	if norm == "" {
		return fmt.Errorf("%w: dashboard username is required", ErrValidation)
	}
	if !dashboardUsernamePattern.MatchString(norm) {
		return fmt.Errorf("%w: dashboard username must match [a-z0-9][a-z0-9._-]{2,31}", ErrValidation)
	}
	return nil
}

// ValidatePlan checks that plan is one of the supported tiers.
func ValidatePlan(plan string) error {
	plan = strings.TrimSpace(strings.ToLower(plan))
	if plan == "" {
		return fmt.Errorf("%w: plan is required", ErrValidation)
	}
	if _, ok := ValidPlans[plan]; !ok {
		return fmt.Errorf("%w: invalid plan %q", ErrValidation, plan)
	}
	return nil
}
