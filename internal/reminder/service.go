package reminder

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

const (
	defaultTickInterval = time.Minute
	defaultSendBatch    = 100

	reminderHour   = 10
	reminderWindow = 24 * time.Hour
)

// Messenger sends Telegram text messages.
type Messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type shopLister interface {
	ListAll(ctx context.Context) ([]*store.Shop, error)
}

type shopReader interface {
	ByID(ctx context.Context, id uuid.UUID) (*store.Shop, error)
}

type employeeLister interface {
	ListActiveByShop(ctx context.Context, shopID uuid.UUID) ([]*store.Employee, error)
	ByID(ctx context.Context, id uuid.UUID) (*store.Employee, error)
}

type availabilityLister interface {
	ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Availability, error)
}

type deliveryRepo interface {
	CreatePending(ctx context.Context, shopID, employeeID uuid.UUID, weekStart time.Time, kind string) (bool, error)
	ListPending(ctx context.Context, limit int) ([]*store.ReminderDelivery, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastErr string) error
}

// Config controls the reminder worker.
type Config struct {
	TickInterval time.Duration
}

// Service runs weekly availability reminders and non-responder nags.
type Service struct {
	shops        shopLister
	shopByID     shopReader
	employees    employeeLister
	availability availabilityLister
	deliveries   deliveryRepo
	messenger    Messenger
	log          *slog.Logger
	cfg          Config
	now          func() time.Time
}

// New wires the reminder service.
func New(
	shops shopLister,
	shopByID shopReader,
	employees employeeLister,
	availability availabilityLister,
	deliveries deliveryRepo,
	messenger Messenger,
	log *slog.Logger,
	cfg Config,
) *Service {
	if log == nil {
		log = slog.Default()
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = defaultTickInterval
	}
	return &Service{
		shops:        shops,
		shopByID:     shopByID,
		employees:    employees,
		availability: availability,
		deliveries:   deliveries,
		messenger:    messenger,
		log:          log,
		cfg:          cfg,
		now:          time.Now,
	}
}

// Run ticks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.Tick(ctx, s.now()); err != nil {
				s.log.Error("reminder tick failed", "err", err)
			}
		}
	}
}

// Tick enqueues due reminders/nags and sends pending deliveries.
func (s *Service) Tick(ctx context.Context, now time.Time) error {
	shops, err := s.shops.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("reminder: list shops: %w", err)
	}
	for _, shop := range shops {
		if err := s.tickShop(ctx, shop, now); err != nil {
			s.log.Error("reminder shop tick failed", "shop", shop.ID, "err", err)
		}
	}
	return s.sendPending(ctx)
}

func (s *Service) tickShop(ctx context.Context, shop *store.Shop, now time.Time) error {
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone %q: %w", shop.Timezone, err)
	}
	nowLocal := now.In(loc)
	targetWeek := targetWeekStart(nowLocal, loc)

	if isDue(nowLocal, reminderDueAt(targetWeek, loc)) {
		if err := s.enqueueReminder(ctx, shop.ID, targetWeek); err != nil {
			return fmt.Errorf("enqueue reminder: %w", err)
		}
	}
	if isDue(nowLocal, nagDueAt(targetWeek, loc)) {
		if err := s.enqueueNag(ctx, shop.ID, targetWeek); err != nil {
			return fmt.Errorf("enqueue nag: %w", err)
		}
	}
	return nil
}

func (s *Service) enqueueReminder(ctx context.Context, shopID uuid.UUID, weekStart time.Time) error {
	employees, err := s.employees.ListActiveByShop(ctx, shopID)
	if err != nil {
		return err
	}
	for _, emp := range employees {
		if emp.TelegramUserID == 0 {
			continue
		}
		if _, err := s.deliveries.CreatePending(ctx, shopID, emp.ID, weekStart, store.ReminderKindAvailabilityReminder); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enqueueNag(ctx context.Context, shopID uuid.UUID, weekStart time.Time) error {
	employees, err := s.employees.ListActiveByShop(ctx, shopID)
	if err != nil {
		return err
	}
	submitted := map[uuid.UUID]struct{}{}
	rows, err := s.availability.ListByShopWeek(ctx, shopID, weekStart)
	if err != nil {
		return err
	}
	for _, row := range rows {
		submitted[row.EmployeeID] = struct{}{}
	}
	for _, emp := range employees {
		if emp.TelegramUserID == 0 {
			continue
		}
		if _, ok := submitted[emp.ID]; ok {
			continue
		}
		if _, err := s.deliveries.CreatePending(ctx, shopID, emp.ID, weekStart, store.ReminderKindAvailabilityNag); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) sendPending(ctx context.Context) error {
	pending, err := s.deliveries.ListPending(ctx, defaultSendBatch)
	if err != nil {
		return err
	}
	for _, d := range pending {
		if err := s.sendOne(ctx, d); err != nil {
			s.log.Error("send reminder failed", "delivery", d.ID, "err", err)
		}
	}
	return nil
}

func (s *Service) sendOne(ctx context.Context, d *store.ReminderDelivery) error {
	emp, err := s.employees.ByID(ctx, d.EmployeeID)
	if err != nil {
		return s.failDelivery(ctx, d, err)
	}
	if emp.TelegramUserID == 0 {
		return s.failDelivery(ctx, d, fmt.Errorf("employee has no telegram id"))
	}

	shop, err := s.shopByID.ByID(ctx, d.ShopID)
	if err != nil {
		return s.failDelivery(ctx, d, err)
	}
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		loc = time.UTC
	}

	text := messageForKind(d.Kind, d.WeekStart.In(loc))
	if err := s.messenger.SendMessage(ctx, emp.TelegramUserID, text); err != nil {
		return s.failDelivery(ctx, d, err)
	}
	return s.deliveries.MarkSent(ctx, d.ID)
}

func (s *Service) failDelivery(ctx context.Context, d *store.ReminderDelivery, err error) error {
	attempts := d.Attempts + 1
	if markErr := s.deliveries.MarkFailed(ctx, d.ID, attempts, err.Error()); markErr != nil {
		return markErr
	}
	return err
}

// targetWeekStart returns the upcoming Monday scheduling week in loc.
func targetWeekStart(now time.Time, loc *time.Location) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	days := (int(time.Monday) - int(t.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return t.AddDate(0, 0, days)
}

func reminderDueAt(weekStart time.Time, loc *time.Location) time.Time {
	thursday := weekStart.AddDate(0, 0, -4)
	return time.Date(thursday.Year(), thursday.Month(), thursday.Day(), reminderHour, 0, 0, 0, loc)
}

func nagDueAt(weekStart time.Time, loc *time.Location) time.Time {
	saturday := weekStart.AddDate(0, 0, -2)
	return time.Date(saturday.Year(), saturday.Month(), saturday.Day(), reminderHour, 0, 0, 0, loc)
}

func isDue(now, dueAt time.Time) bool {
	return !now.Before(dueAt) && now.Before(dueAt.Add(reminderWindow))
}

func messageForKind(kind string, weekStart time.Time) string {
	weekEnd := weekStart.AddDate(0, 0, 6)
	rangeLabel := fmt.Sprintf("%s–%s", weekStart.Format("02/01"), weekEnd.Format("02/01"))
	switch kind {
	case store.ReminderKindAvailabilityNag:
		return fmt.Sprintf("Mình chưa nhận lịch rảnh của bạn cho tuần tới (%s). Gửi giúp mình trước khi quán chốt lịch nha.", rangeLabel)
	default:
		return fmt.Sprintf(`Tuần sau (%s) bạn rảnh/bận lúc nào? Nhắn mình lịch rảnh để quán xếp ca nha.

Ví dụ:
Thứ 2 rảnh sáng, thứ 4 bận, thứ 6 ưu tiên ca tối.`, rangeLabel)
	}
}
