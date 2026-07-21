package dashboard

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
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
	HasDashboardPassword(ctx context.Context, shopID uuid.UUID) (bool, error)
	SetDashboardCredentials(ctx context.Context, shopID uuid.UUID, password, email, hint string) error
	VerifyDashboardPassword(ctx context.Context, shopID uuid.UUID, password string) error
	DashboardEmail(ctx context.Context, shopID uuid.UUID) (string, error)
	IssueDashboardPasswordReset(ctx context.Context, shopID uuid.UUID) (string, error)
	ResetDashboardPasswordWithToken(ctx context.Context, token, password string) (*store.Shop, error)
}

type mailSender interface {
	Send(ctx context.Context, to, subject, body string) error
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
	shops             shopReader
	shopAuth          shopAuthenticator
	ownerLinks        ownerLinkIssuer
	shifts            shiftRepo
	schedules         scheduleRepo
	employees         employeeLister
	employeeMgmt      employeeAdmin
	availability      availabilityLister
	planner           weekGenerator
	onboarding        shopOnboarder
	signupEnabled     bool
	botUsername       string
	incidentMessenger incidentMessenger
	incidentChatID    int64
	mail              mailSender
	dashboardBaseURL  string
	sessions          *SessionManager
	log               *slog.Logger
	tmpl              *templateSet
}

// SetTelegramBotUsername configures the public bot username used for employee invites.
func (s *Server) SetTelegramBotUsername(username string) {
	s.botUsername = normalizeTelegramUsername(username)
}

// SetPasswordResetMail configures outbound email for owner password recovery.
func (s *Server) SetPasswordResetMail(sender mailSender, baseURL string) {
	s.mail = sender
	s.dashboardBaseURL = strings.TrimSpace(baseURL)
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
		ownerLinks:    st.Shops,
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
	mux.HandleFunc("GET /login/reset", s.handlePasswordResetGET)
	mux.HandleFunc("POST /login/reset", s.handlePasswordResetPOST)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /dashboard/week", s.handleWeek)
	mux.HandleFunc("POST /dashboard/generate", s.handleGenerate)
	mux.HandleFunc("POST /dashboard/schedules/{id}/approve", s.handleApprove)
	mux.HandleFunc("POST /dashboard/shifts", s.handleCreateShift)
	mux.HandleFunc("POST /dashboard/shifts/{id}/activate", s.handleActivateShift)
	mux.HandleFunc("POST /dashboard/shifts/{id}/deactivate", s.handleDeactivateShift)
	mux.HandleFunc("POST /dashboard/employees/{id}", s.handleUpdateEmployee)
	mux.HandleFunc("POST /dashboard/employees/{id}/activate", s.handleActivateEmployee)
	mux.HandleFunc("POST /dashboard/employees/{id}/deactivate", s.handleDeactivateEmployee)
	mux.HandleFunc("POST /dashboard/telegram/owner-link", s.handleOwnerTelegramLink)
	mux.HandleFunc("POST /dashboard/telegram/refresh", s.handleTelegramStatusRefresh)
	mux.HandleFunc("POST /dashboard/telegram/employees/refresh", s.handleTelegramEmployeesRefresh)
	mux.HandleFunc("POST /dashboard/incident-report", s.handleIncidentReport)
}

type templateSet struct {
	root *template.Template
}

func (t *templateSet) render(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	return t.root.ExecuteTemplate(w, name, data)
}
