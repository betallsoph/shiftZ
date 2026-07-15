package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

const dateLayout = "2006-01-02"

type generateResponse struct {
	ShopID     string          `json:"shop_id"`
	WeekStart  string          `json:"week_start"`
	Candidates []candidateJSON `json:"candidates"`
	Warnings   []string        `json:"warnings"`
}

type candidateJSON struct {
	ID              string   `json:"id"`
	VariantLabel    string   `json:"variant_label"`
	SolverLabel     string   `json:"solver_label"`
	Score           float64  `json:"score"`
	AssignmentCount int      `json:"assignment_count"`
	Violations      []string `json:"violations"`
}

type listSchedulesResponse struct {
	ShopID    string         `json:"shop_id"`
	WeekStart string         `json:"week_start"`
	Schedules []scheduleJSON `json:"schedules"`
}

type scheduleJSON struct {
	ID           string           `json:"id"`
	VariantLabel string           `json:"variant_label"`
	Status       string           `json:"status"`
	Score        float64          `json:"score"`
	Assignments  []assignmentJSON `json:"assignments"`
}

type assignmentJSON struct {
	ID             string `json:"id"`
	ShiftID        string `json:"shift_id"`
	EmployeeID     string `json:"employee_id"`
	Date           string `json:"date"`
	ShiftName      string `json:"shift_name"`
	ShiftWeekday   int    `json:"shift_weekday"`
	ShiftStartTime string `json:"shift_start_time"`
	ShiftEndTime   string `json:"shift_end_time"`
	EmployeeName   string `json:"employee_name"`
}

type approveResponse struct {
	ID           string  `json:"id"`
	ShopID       string  `json:"shop_id"`
	WeekStart    string  `json:"week_start"`
	VariantLabel string  `json:"variant_label"`
	Status       string  `json:"status"`
	Score        float64 `json:"score"`
}

func (s *Server) handleGenerateSchedule(w http.ResponseWriter, r *http.Request) {
	shopID, weekStart, ok := s.parseShopWeek(r, w)
	if !ok {
		return
	}

	result, err := s.planner.GenerateWeek(r.Context(), shopID, weekStart)
	if err != nil {
		s.handlePlannerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toGenerateResponse(result))
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	shopID, weekStart, ok := s.parseShopWeek(r, w)
	if !ok {
		return
	}

	schedules, err := s.schedules.ListByShopWeek(r.Context(), shopID, weekStart)
	if err != nil {
		s.log.Error("list schedules failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}
	writeJSON(w, http.StatusOK, listSchedulesResponse{
		ShopID:    shopID.String(),
		WeekStart: formatDate(weekStart),
		Schedules: toScheduleJSONs(schedules),
	})
}

func (s *Server) handleApproveSchedule(w http.ResponseWriter, r *http.Request) {
	shopID, err := parseShopID(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scheduleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	approved, err := s.schedules.Approve(r.Context(), shopID, scheduleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		s.log.Error("approve schedule failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to approve schedule")
		return
	}
	writeJSON(w, http.StatusOK, approveResponse{
		ID:           approved.ID.String(),
		ShopID:       approved.ShopID.String(),
		WeekStart:    formatDate(approved.WeekStart),
		VariantLabel: approved.VariantLabel,
		Status:       approved.Status,
		Score:        approved.Score,
	})
}

func (s *Server) parseShopWeek(r *http.Request, w http.ResponseWriter) (uuid.UUID, time.Time, bool) {
	shopID, err := parseShopID(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return uuid.Nil, time.Time{}, false
	}
	shop, err := s.shops.ByID(r.Context(), shopID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "shop not found")
			return uuid.Nil, time.Time{}, false
		}
		s.log.Error("load shop failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to load shop")
		return uuid.Nil, time.Time{}, false
	}
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		s.log.Error("load timezone failed", "timezone", shop.Timezone, "err", err)
		writeError(w, http.StatusInternalServerError, "invalid shop timezone")
		return uuid.Nil, time.Time{}, false
	}
	weekStart, err := parseWeekStart(r.URL.Query(), loc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return uuid.Nil, time.Time{}, false
	}
	return shopID, weekStart, true
}

func (s *Server) handlePlannerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, planner.ErrSchedulesExist):
		writeError(w, http.StatusConflict, "schedules already exist for this shop and week")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "shop not found")
	default:
		s.log.Error("generate schedule failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to generate schedules")
	}
}

func parseShopID(q url.Values) (uuid.UUID, error) {
	raw := q.Get("shop_id")
	if raw == "" {
		return uuid.Nil, fmt.Errorf("missing shop_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid shop_id")
	}
	return id, nil
}

func parseWeekStart(q url.Values, loc *time.Location) (time.Time, error) {
	raw := q.Get("week_start")
	if raw == "" {
		return time.Time{}, fmt.Errorf("missing week_start")
	}
	parsed, err := time.ParseInLocation(dateLayout, raw, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid week_start, expected YYYY-MM-DD")
	}
	return store.WeekStart(parsed, loc), nil
}

func formatDate(t time.Time) string {
	return t.Format(dateLayout)
}

func toGenerateResponse(result *planner.GenerateResult) generateResponse {
	candidates := make([]candidateJSON, len(result.Candidates))
	for i, c := range result.Candidates {
		violations := c.Violations
		if violations == nil {
			violations = []string{}
		}
		candidates[i] = candidateJSON{
			ID:              c.ID.String(),
			VariantLabel:    c.VariantLabel,
			SolverLabel:     c.SolverLabel,
			Score:           c.Score,
			AssignmentCount: c.AssignmentCount,
			Violations:      violations,
		}
	}
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return generateResponse{
		ShopID:     result.ShopID.String(),
		WeekStart:  formatDate(result.WeekStart),
		Candidates: candidates,
		Warnings:   warnings,
	}
}

func toScheduleJSONs(schedules []*store.Schedule) []scheduleJSON {
	out := make([]scheduleJSON, len(schedules))
	for i, sched := range schedules {
		assignments := make([]assignmentJSON, len(sched.Assignments))
		for j, a := range sched.Assignments {
			assignments[j] = assignmentJSON{
				ID:             a.ID.String(),
				ShiftID:        a.ShiftID.String(),
				EmployeeID:     a.EmployeeID.String(),
				Date:           formatDate(a.Date),
				ShiftName:      a.ShiftName,
				ShiftWeekday:   a.ShiftWeekday,
				ShiftStartTime: a.ShiftStartTime,
				ShiftEndTime:   a.ShiftEndTime,
				EmployeeName:   a.EmployeeName,
			}
		}
		out[i] = scheduleJSON{
			ID:           sched.ID.String(),
			VariantLabel: sched.VariantLabel,
			Status:       sched.Status,
			Score:        sched.Score,
			Assignments:  assignments,
		}
	}
	return out
}
