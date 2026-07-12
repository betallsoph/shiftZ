package planner

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/solver"
	"github.com/betallsoph/shiftz/internal/store"
)

// ErrSchedulesExist is returned when schedules already exist for a shop week.
var ErrSchedulesExist = errors.New("planner: schedules already exist for shop week")

var uiLabels = []string{"A", "B", "C"}

// Service loads store data, runs the solver, and persists schedule candidates.
type Service struct {
	store *store.Store
}

// New wires a planner on top of the store layer.
func New(st *store.Store) *Service {
	return &Service{store: st}
}

// GenerateResult is the outcome of one weekly generation run.
type GenerateResult struct {
	ShopID     uuid.UUID
	WeekStart  time.Time
	Candidates []CandidateSummary
	Warnings   []string
}

// CandidateSummary describes one persisted schedule variant.
type CandidateSummary struct {
	ID              uuid.UUID
	VariantLabel    string
	SolverLabel     string
	Score           float64
	AssignmentCount int
	Violations      []string
}

// GenerateWeek loads data, generates solver candidates, and persists them.
func (s *Service) GenerateWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*GenerateResult, error) {
	shop, err := s.store.Shops.ByID(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("planner: load shop: %w", err)
	}
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		return nil, fmt.Errorf("planner: load timezone %q: %w", shop.Timezone, err)
	}
	weekStart = store.WeekStart(weekStart, loc)

	employees, err := s.store.Employees.ListActiveByShop(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("planner: load employees: %w", err)
	}
	if len(employees) == 0 {
		return nil, fmt.Errorf("planner: no active employees for shop")
	}

	shifts, err := s.store.Shifts.ListByShop(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("planner: load shifts: %w", err)
	}
	if len(shifts) == 0 {
		return nil, fmt.Errorf("planner: no shift templates for shop")
	}

	availability, err := s.store.Availability.ListByShopWeek(ctx, shopID, weekStart)
	if err != nil {
		return nil, fmt.Errorf("planner: load availability: %w", err)
	}

	rules, err := s.store.Rules.ListByShop(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("planner: load rules: %w", err)
	}

	problem, occurrences, err := BuildProblem(weekStart, loc, employees, shifts, availability)
	if err != nil {
		return nil, err
	}

	penaltyRules, ruleWarnings := mapRules(employees, rules)
	seed := seedFor(shopID, weekStart)

	candidates, err := solver.GenerateCandidates(problem, penaltyRules, 3, seed)
	if err != nil {
		return nil, fmt.Errorf("planner: generate candidates: %w", err)
	}

	result := &GenerateResult{
		ShopID:    shopID,
		WeekStart: weekStart,
		Warnings:  append([]string(nil), ruleWarnings...),
	}

	newCandidates := make([]store.NewScheduleCandidate, 0, len(candidates))
	summaries := make([]CandidateSummary, 0, len(candidates))
	for i, cand := range candidates {
		label := uiLabels[i%len(uiLabels)]
		violations := violationStrings(solver.Validate(problem, cand.Schedule))
		assignments, count := assignmentsFromSchedule(cand.Schedule, occurrences)

		newCandidates = append(newCandidates, store.NewScheduleCandidate{
			VariantLabel: label,
			Score:        cand.Score,
			Assignments:  assignments,
		})
		summaries = append(summaries, CandidateSummary{
			VariantLabel:    label,
			SolverLabel:     cand.Label,
			Score:           cand.Score,
			AssignmentCount: count,
			Violations:      violations,
		})
	}

	saved, err := s.store.Schedules.CreateCandidates(ctx, shopID, weekStart, newCandidates)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, ErrSchedulesExist
		}
		return nil, fmt.Errorf("planner: save candidates: %w", err)
	}
	for i, row := range saved {
		summaries[i].ID = row.ID
		result.Candidates = append(result.Candidates, summaries[i])
	}
	return result, nil
}

func seedFor(shopID uuid.UUID, weekStart time.Time) int64 {
	h := fnv.New64a()
	_, _ = h.Write(shopID[:])
	_, _ = h.Write([]byte(weekStart.Format("2006-01-02")))
	return int64(h.Sum64())
}

func violationStrings(vs []solver.Violation) []string {
	if len(vs) == 0 {
		return nil
	}
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.String()
	}
	return out
}

func assignmentsFromSchedule(
	sched *solver.Schedule,
	occurrences map[solver.ShiftID]ShiftOccurrence,
) ([]store.NewScheduleAssignment, int) {
	var out []store.NewScheduleAssignment
	count := 0
	for shiftID, emps := range sched.Assignments {
		occ, ok := occurrences[shiftID]
		if !ok {
			continue
		}
		for _, empID := range emps {
			employeeID, err := uuid.Parse(string(empID))
			if err != nil {
				continue
			}
			out = append(out, store.NewScheduleAssignment{
				ShiftID:    occ.ShiftID,
				EmployeeID: employeeID,
				Date:       occ.Date,
			})
			count++
		}
	}
	return out, count
}
