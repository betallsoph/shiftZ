package dashboard

import (
	"context"
	"html/template"
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

type shopAuthenticator interface {
	ByDashboardUsername(ctx context.Context, username string) (*store.Shop, error)
}

type scheduleRepo interface {
	ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Schedule, error)
	Approve(ctx context.Context, shopID, scheduleID uuid.UUID) (*store.Schedule, error)
}

type employeeLister interface {
	ListActiveByShop(ctx context.Context, shopID uuid.UUID) ([]*store.Employee, error)
}

type availabilityLister interface {
	ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Availability, error)
}

type weekGenerator interface {
	GenerateWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error)
}

// Server renders the owner dashboard with HTMX partials.
type Server struct {
	shops         shopReader
	shopAuth      shopAuthenticator
	shopTelegram  shopTelegramSetup
	shifts        shiftRepo
	schedules     scheduleRepo
	employees     employeeLister
	employeeMgmt  employeeAdmin
	availability  availabilityLister
	planner       weekGenerator
	onboarding    shopOnboarder
	signupEnabled bool
	sessions      *SessionManager
	log           *slog.Logger
	tmpl          *templateSet
}

// New wires the dashboard on top of the store and planner.
func New(st *store.Store, sessions *SessionManager, onboard shopOnboarder, signupEnabled bool, log *slog.Logger) (*Server, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		shops:         st.Shops,
		shopAuth:      st.Shops,
		shopTelegram:  st.Shops,
		shifts:        st.Shifts,
		schedules:     st.Schedules,
		employees:     st.Employees,
		employeeMgmt:  st.Employees,
		availability:  st.Availability,
		planner:       planner.New(st),
		onboarding:    onboard,
		signupEnabled: signupEnabled,
		sessions:      sessions,
		log:           log,
		tmpl:          &templateSet{tmpl},
	}, nil
}

// Register mounts dashboard routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", s.handleLoginGET)
	mux.HandleFunc("POST /login", s.handleLoginPOST)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /dashboard/week", s.handleWeek)
	mux.HandleFunc("POST /dashboard/generate", s.handleGenerate)
	mux.HandleFunc("POST /dashboard/schedules/{id}/approve", s.handleApprove)
	mux.HandleFunc("POST /dashboard/telegram/setup-code", s.handleRotateTelegramSetupCode)
	mux.HandleFunc("POST /dashboard/shifts", s.handleCreateShift)
	mux.HandleFunc("POST /dashboard/shifts/{id}/activate", s.handleActivateShift)
	mux.HandleFunc("POST /dashboard/shifts/{id}/deactivate", s.handleDeactivateShift)
	mux.HandleFunc("POST /dashboard/employees/{id}", s.handleUpdateEmployee)
	mux.HandleFunc("POST /dashboard/employees/{id}/activate", s.handleActivateEmployee)
	mux.HandleFunc("POST /dashboard/employees/{id}/deactivate", s.handleDeactivateEmployee)
}

type templateSet struct {
	root *template.Template
}

func (t *templateSet) render(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	return t.root.ExecuteTemplate(w, name, data)
}
