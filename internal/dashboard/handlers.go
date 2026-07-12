package dashboard

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.tmpl.render(w, "page.html", PageData{
		Today: time.Now().Format(dateLayout),
	}); err != nil {
		s.log.Error("render page", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleWeek(w http.ResponseWriter, r *http.Request) {
	s.renderWeek(w, r, "", nil)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	shop, shopID, weekStart, errMsg := s.parseShopWeekForm(r)
	if errMsg != "" {
		s.renderWeekView(w, WeekView{Error: errMsg})
		return
	}

	result, err := s.planner.GenerateWeek(r.Context(), shopID, weekStart)
	if err != nil {
		if errors.Is(err, planner.ErrSchedulesExist) {
			schedules, listErr := s.schedules.ListByShopWeek(r.Context(), shopID, weekStart)
			if listErr != nil {
				s.log.Error("list schedules after exist", "err", listErr)
				s.renderWeekView(w, WeekView{Error: "failed to load existing schedules"})
				return
			}
			s.renderWeekView(w, buildWeekView(shop, weekStart, schedules, nil,
				"Schedules already exist for this week.", nil))
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			s.renderWeekView(w, WeekView{Error: "shop not found"})
			return
		}
		s.log.Error("generate week", "err", err)
		s.renderWeekView(w, WeekView{Error: "generation failed"})
		return
	}

	schedules, err := s.schedules.ListByShopWeek(r.Context(), shopID, weekStart)
	if err != nil {
		s.log.Error("list schedules after generate", "err", err)
		s.renderWeekView(w, WeekView{Error: "failed to load generated schedules"})
		return
	}
	s.renderWeekView(w, buildWeekView(shop, weekStart, schedules, result.Warnings, "", result))
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	shop, shopID, weekStart, errMsg := s.parseShopWeekForm(r)
	if errMsg != "" {
		s.renderWeekView(w, WeekView{Error: errMsg})
		return
	}

	scheduleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.renderWeekView(w, WeekView{Error: "invalid schedule id"})
		return
	}

	if _, err := s.schedules.Approve(r.Context(), shopID, scheduleID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderWeekView(w, WeekView{
				ShopID:    shop.ID.String(),
				ShopName:  shop.Name,
				WeekStart: weekStart.Format(dateLayout),
				Error:     "schedule not found",
			})
			return
		}
		s.log.Error("approve schedule", "err", err)
		s.renderWeekView(w, WeekView{Error: "failed to approve schedule"})
		return
	}

	s.renderWeek(w, r, "", nil)
}

func (s *Server) renderWeek(w http.ResponseWriter, r *http.Request, notice string, generated *planner.GenerateResult) {
	shop, shopID, weekStart, errMsg := s.parseShopWeekForm(r)
	if errMsg != "" {
		s.renderWeekView(w, WeekView{Error: errMsg})
		return
	}

	schedules, err := s.schedules.ListByShopWeek(r.Context(), shopID, weekStart)
	if err != nil {
		s.log.Error("list schedules", "err", err)
		s.renderWeekView(w, WeekView{Error: "failed to load schedules"})
		return
	}

	view := buildWeekView(shop, weekStart, schedules, nil, notice, generated)
	s.renderWeekView(w, view)
}

func (s *Server) renderWeekView(w http.ResponseWriter, view WeekView) {
	if err := s.tmpl.render(w, "week_panel.html", view); err != nil {
		s.log.Error("render week panel", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) parseShopWeekForm(r *http.Request) (*store.Shop, uuid.UUID, time.Time, string) {
	if err := r.ParseForm(); err != nil {
		return nil, uuid.Nil, time.Time{}, "invalid form"
	}
	rawShop := r.FormValue("shop_id")
	if rawShop == "" {
		return nil, uuid.Nil, time.Time{}, "missing shop id"
	}
	shopID, err := uuid.Parse(rawShop)
	if err != nil {
		return nil, uuid.Nil, time.Time{}, "invalid shop id"
	}
	shop, err := s.shops.ByID(r.Context(), shopID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, uuid.Nil, time.Time{}, "shop not found"
		}
		s.log.Error("load shop", "err", err)
		return nil, uuid.Nil, time.Time{}, "failed to load shop"
	}
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		s.log.Error("load timezone", "timezone", shop.Timezone, "err", err)
		return nil, uuid.Nil, time.Time{}, "invalid shop timezone"
	}
	rawWeek := r.FormValue("week_start")
	if rawWeek == "" {
		return nil, uuid.Nil, time.Time{}, "missing week start"
	}
	parsed, err := time.ParseInLocation(dateLayout, rawWeek, loc)
	if err != nil {
		return nil, uuid.Nil, time.Time{}, "invalid date, expected YYYY-MM-DD"
	}
	return shop, shopID, store.WeekStart(parsed, loc), ""
}
