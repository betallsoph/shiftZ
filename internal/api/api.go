package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

type shopReader interface {
	ByID(ctx context.Context, id uuid.UUID) (*store.Shop, error)
}

type scheduleRepo interface {
	ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Schedule, error)
	Approve(ctx context.Context, shopID, scheduleID uuid.UUID) (*store.Schedule, error)
}

type weekGenerator interface {
	GenerateWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error)
}

// Server exposes JSON HTTP handlers for development and testing.
type Server struct {
	shops     shopReader
	schedules scheduleRepo
	planner   weekGenerator
	log       *slog.Logger
}

// New wires the API on top of the store and planner.
func New(st *store.Store, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		shops:     st.Shops,
		schedules: st.Schedules,
		planner:   planner.New(st),
		log:       log,
	}
}

// Register mounts API routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/dev/generate-schedule", s.handleGenerateSchedule)
	mux.HandleFunc("GET /api/v1/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/v1/schedules/{id}/approve", s.handleApproveSchedule)
}
