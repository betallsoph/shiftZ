package reminder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestTickDoesNotEnqueueBeforeDue(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	shopID := uuid.New()
	// Wednesday before reminder window
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, loc)

	svc := newTestService(t, testDeps{
		shops: []*store.Shop{{ID: shopID, Timezone: "Asia/Ho_Chi_Minh"}},
		employees: []*store.Employee{{
			ID: shopID, ShopID: shopID, TelegramUserID: 100, DisplayName: "Anna", IsActive: true,
		}},
	})
	svc.now = func() time.Time { return now }

	if err := svc.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if svc.deliveries.(*fakeDeliveries).createCalls != 0 {
		t.Fatalf("createCalls = %d, want 0", svc.deliveries.(*fakeDeliveries).createCalls)
	}
}

func TestReminderEnqueuesActiveLinkedEmployees(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	shopID := uuid.New()
	annaID := uuid.New()
	bobID := uuid.New()
	now := time.Date(2026, 7, 16, 10, 30, 0, 0, loc)

	svc := newTestService(t, testDeps{
		shops: []*store.Shop{{ID: shopID, Timezone: "Asia/Ho_Chi_Minh"}},
		employees: []*store.Employee{
			{ID: annaID, ShopID: shopID, TelegramUserID: 100, DisplayName: "Anna", IsActive: true},
			{ID: bobID, ShopID: shopID, TelegramUserID: 0, DisplayName: "Bob", IsActive: true},
			{ID: uuid.New(), ShopID: shopID, TelegramUserID: 200, DisplayName: "Chi", IsActive: false},
		},
	})
	svc.now = func() time.Time { return now }

	if err := svc.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if svc.deliveries.(*fakeDeliveries).createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1 (Anna only)", svc.deliveries.(*fakeDeliveries).createCalls)
	}
}

func TestReminderIdempotentAcrossTicks(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	shopID := uuid.New()
	empID := uuid.New()
	now := time.Date(2026, 7, 16, 10, 30, 0, 0, loc)

	deliveries := &fakeDeliveries{}
	messenger := &fakeMessenger{}
	svc := newTestService(t, testDeps{
		shops:     []*store.Shop{{ID: shopID, Timezone: "Asia/Ho_Chi_Minh"}},
		employees: []*store.Employee{{ID: empID, ShopID: shopID, TelegramUserID: 100, IsActive: true}},
		deliveries: deliveries,
		messenger: messenger,
	})
	svc.now = func() time.Time { return now }

	ctx := context.Background()
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(messenger.messages) != 1 {
		t.Fatalf("first tick messages = %d, want 1", len(messenger.messages))
	}
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(messenger.messages) != 1 {
		t.Fatalf("second tick duplicated messages = %d", len(messenger.messages))
	}
	if deliveries.createCalls != 2 {
		t.Fatalf("createCalls = %d, want 2 attempts", deliveries.createCalls)
	}
}

func TestNagOnlyMissingAvailability(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	shopID := uuid.New()
	annaID := uuid.New()
	bobID := uuid.New()
	now := time.Date(2026, 7, 18, 11, 0, 0, 0, loc)
	weekStart := targetWeekStart(now, loc)

	deliveries := &fakeDeliveries{}
	svc := newTestService(t, testDeps{
		shops: []*store.Shop{{ID: shopID, Timezone: "Asia/Ho_Chi_Minh"}},
		employees: []*store.Employee{
			{ID: annaID, ShopID: shopID, TelegramUserID: 100, IsActive: true},
			{ID: bobID, ShopID: shopID, TelegramUserID: 101, IsActive: true},
		},
		availability: []*store.Availability{{EmployeeID: annaID, ShopID: shopID, WeekStart: weekStart}},
		deliveries:   deliveries,
	})
	svc.now = func() time.Time { return now }

	if err := svc.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if deliveries.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1 nag for Bob", deliveries.createCalls)
	}
	if deliveries.lastKind != store.ReminderKindAvailabilityNag {
		t.Fatalf("kind = %q", deliveries.lastKind)
	}
}

func TestSendPendingMarksFailedOnMessengerError(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	deliveryID := uuid.New()
	weekStart := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	deliveries := &fakeDeliveries{
		pending: []*store.ReminderDelivery{{
			ID: deliveryID, ShopID: shopID, EmployeeID: empID, WeekStart: weekStart,
			Kind: store.ReminderKindAvailabilityReminder, Status: store.ReminderStatusPending,
		}},
	}
	messenger := &fakeMessenger{err: errors.New("telegram down")}
	svc := newTestService(t, testDeps{
		shops:     []*store.Shop{{ID: shopID, Timezone: "UTC"}},
		employees: []*store.Employee{{ID: empID, ShopID: shopID, TelegramUserID: 100, IsActive: true}},
		deliveries: deliveries,
		messenger: messenger,
	})

	if err := svc.sendPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deliveries.sentID != uuid.Nil {
		t.Fatal("should not mark sent")
	}
	if deliveries.failedID != deliveryID {
		t.Fatalf("failedID = %v", deliveries.failedID)
	}
}

func TestSendPendingMarksSent(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	deliveryID := uuid.New()

	deliveries := &fakeDeliveries{
		pending: []*store.ReminderDelivery{{
			ID: deliveryID, ShopID: shopID, EmployeeID: empID,
			WeekStart: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
			Kind:      store.ReminderKindAvailabilityReminder,
		}},
	}
	messenger := &fakeMessenger{}
	svc := newTestService(t, testDeps{
		shops:     []*store.Shop{{ID: shopID, Timezone: "UTC"}},
		employees: []*store.Employee{{ID: empID, ShopID: shopID, TelegramUserID: 100, IsActive: true}},
		deliveries: deliveries,
		messenger: messenger,
	})

	if err := svc.sendPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deliveries.sentID != deliveryID {
		t.Fatalf("sentID = %v", deliveries.sentID)
	}
	if len(messenger.messages) != 1 {
		t.Fatalf("messages = %d", len(messenger.messages))
	}
}

type testDeps struct {
	shops        []*store.Shop
	employees    []*store.Employee
	availability []*store.Availability
	deliveries   *fakeDeliveries
	messenger    *fakeMessenger
}

func newTestService(t *testing.T, deps testDeps) *Service {
	t.Helper()
	if deps.deliveries == nil {
		deps.deliveries = &fakeDeliveries{}
	}
	if deps.messenger == nil {
		deps.messenger = &fakeMessenger{}
	}
	return New(
		&fakeShops{shops: deps.shops},
		&fakeShops{shops: deps.shops},
		&fakeEmployees{employees: deps.employees},
		&fakeAvailability{rows: deps.availability},
		deps.deliveries,
		deps.messenger,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config{TickInterval: time.Minute},
	)
}

type fakeShops struct {
	shops []*store.Shop
}

func (f *fakeShops) ListAll(ctx context.Context) ([]*store.Shop, error) {
	return f.shops, nil
}

func (f *fakeShops) ByID(ctx context.Context, id uuid.UUID) (*store.Shop, error) {
	for _, s := range f.shops {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, store.ErrNotFound
}

type fakeEmployees struct {
	employees []*store.Employee
}

func (f *fakeEmployees) ListActiveByShop(ctx context.Context, shopID uuid.UUID) ([]*store.Employee, error) {
	var out []*store.Employee
	for _, e := range f.employees {
		if e.ShopID == shopID && e.IsActive {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeEmployees) ByID(ctx context.Context, id uuid.UUID) (*store.Employee, error) {
	for _, e := range f.employees {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}

type fakeAvailability struct {
	rows []*store.Availability
}

func (f *fakeAvailability) ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Availability, error) {
	var out []*store.Availability
	for _, r := range f.rows {
		if r.ShopID == shopID && r.WeekStart.Equal(weekStart) {
			out = append(out, r)
		}
	}
	return out, nil
}

type deliveryKey struct {
	shopID, employeeID uuid.UUID
	weekDate           string
	kind               string
}

func deliveryKeyFor(shopID, employeeID uuid.UUID, weekStart time.Time, kind string) deliveryKey {
	utc := weekStart.In(time.UTC)
	return deliveryKey{
		shopID:     shopID,
		employeeID: employeeID,
		weekDate:   utc.Format("2006-01-02"),
		kind:       kind,
	}
}

type fakeDeliveries struct {
	mu          sync.Mutex
	existing    map[deliveryKey]struct{}
	pending     []*store.ReminderDelivery
	createCalls int
	lastKind    string
	sentID      uuid.UUID
	failedID    uuid.UUID
}

func (f *fakeDeliveries) CreatePending(ctx context.Context, shopID, employeeID uuid.UUID, weekStart time.Time, kind string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	f.lastKind = kind
	key := deliveryKeyFor(shopID, employeeID, weekStart, kind)
	if f.existing == nil {
		f.existing = map[deliveryKey]struct{}{}
	}
	if _, ok := f.existing[key]; ok {
		return false, nil
	}
	f.existing[key] = struct{}{}
	id := uuid.New()
	f.pending = append(f.pending, &store.ReminderDelivery{
		ID: id, ShopID: shopID, EmployeeID: employeeID, WeekStart: weekStart, Kind: kind,
		Status: store.ReminderStatusPending,
	})
	return true, nil
}

func (f *fakeDeliveries) ListPending(ctx context.Context, limit int) ([]*store.ReminderDelivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*store.ReminderDelivery(nil), f.pending...), nil
}

func (f *fakeDeliveries) MarkSent(ctx context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sentID = id
	for i, p := range f.pending {
		if p.ID == id {
			f.pending = append(f.pending[:i], f.pending[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeDeliveries) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failedID = id
	for i, p := range f.pending {
		if p.ID == id {
			f.pending = append(f.pending[:i], f.pending[i+1:]...)
			break
		}
	}
	return nil
}

type fakeMessenger struct {
	messages []sentMsg
	err      error
}

type sentMsg struct {
	chatID int64
	text   string
}

func (f *fakeMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, sentMsg{chatID: chatID, text: text})
	return nil
}
