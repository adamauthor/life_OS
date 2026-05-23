package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"life_os/internal/domain"
	"life_os/internal/telegram"
)

const (
	defaultTickInterval = time.Minute
	defaultDueLimit     = 20
	staleAfter          = 24 * time.Hour
)

type Repository interface {
	RegisterTelegramUser(ctx context.Context, input domain.RegisterTelegramUserInput) (domain.User, error)
	GetUser(ctx context.Context, userID domain.UUID) (domain.User, error)
	UpdateUserTimezone(ctx context.Context, userID domain.UUID, timezone string) error
	GetAutonomySettings(ctx context.Context, userID domain.UUID) (domain.AutonomySettings, error)
	SetAutonomyEnabled(ctx context.Context, userID domain.UUID, enabled bool) error
	UpdateAutonomyQuietHours(ctx context.Context, userID domain.UUID, quietStart string, quietEnd string) error
	UpdateAutonomyDailyLimit(ctx context.Context, userID domain.UUID, limit int) error
	ListNotificationRules(ctx context.Context, userID domain.UUID) ([]domain.NotificationRule, error)
	UpdateNotificationRuleTime(ctx context.Context, userID domain.UUID, kind domain.NotificationKind, scheduleTime string) error
	ListAutonomyUsers(ctx context.Context) ([]domain.AutonomyUser, error)
	UpsertScheduledNotification(ctx context.Context, notification domain.ScheduledNotification) (domain.ScheduledNotification, bool, error)
	ListDueNotifications(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledNotification, error)
	ClaimScheduledNotification(ctx context.Context, notificationID domain.UUID) (bool, error)
	MarkNotificationSent(ctx context.Context, notificationID domain.UUID) error
	MarkNotificationFailed(ctx context.Context, notificationID domain.UUID, message string) error
	MarkNotificationActioned(ctx context.Context, userID domain.UUID, notificationID domain.UUID, status domain.NotificationStatus) error
	SnoozeNotification(ctx context.Context, userID domain.UUID, notificationID domain.UUID, dueAt time.Time) error
	CountSentNotifications(ctx context.Context, userID domain.UUID, from time.Time, to time.Time) (int, error)
	LogNotification(ctx context.Context, log domain.NotificationLog) error
	GetScheduledNotificationForUser(ctx context.Context, userID domain.UUID, notificationID domain.UUID) (domain.ScheduledNotification, error)
}

type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithButtons(ctx context.Context, chatID int64, text string, buttons []telegram.InlineButton) error
}

type PlanningService interface {
	BuildDailyDirection(ctx context.Context, userID domain.UUID, date time.Time) (*domain.DailyDirection, error)
	BuildReplanProposal(ctx context.Context, userID domain.UUID, input string, date time.Time) (*domain.ReplanProposal, error)
}

type ReviewService interface {
	BuildWeeklyReview(ctx context.Context, userID domain.UUID, weekStart time.Time) (string, error)
}

type PatternService interface {
	ListActive(ctx context.Context, userID domain.UUID) ([]domain.BehavioralPattern, error)
}

type Service struct {
	repository Repository
	sender     Sender
	planning   PlanningService
	review     ReviewService
	patterns   PatternService
	timezone   *time.Location
	logger     *slog.Logger
	tick       time.Duration
	enabled    bool
	defaultOn  bool
}

type Config struct {
	SchedulerEnabled bool
	DefaultEnabled   bool
	Tick             time.Duration
}

func NewService(repository Repository, sender Sender, planning PlanningService, review ReviewService, patterns PatternService, timezone *time.Location, logger *slog.Logger, cfg Config) *Service {
	if timezone == nil {
		timezone = time.UTC
	}
	if logger == nil {
		logger = slog.Default()
	}
	tick := cfg.Tick
	if tick <= 0 {
		tick = defaultTickInterval
	}
	return &Service{
		repository: repository,
		sender:     sender,
		planning:   planning,
		review:     review,
		patterns:   patterns,
		timezone:   timezone,
		logger:     logger,
		tick:       tick,
		enabled:    cfg.SchedulerEnabled,
		defaultOn:  cfg.DefaultEnabled,
	}
}

func (s *Service) Run(ctx context.Context) {
	if !s.enabled {
		s.logger.Info("autonomy scheduler disabled")
		return
	}
	s.logger.Info("autonomy scheduler started", "tick", s.tick.String())
	s.runOnce(ctx)
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("autonomy scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Service) RegisterTelegramUser(ctx context.Context, input domain.RegisterTelegramUserInput) (domain.User, error) {
	if input.Timezone == "" {
		input.Timezone = s.timezone.String()
	}
	input.DefaultAutonomyEnabled = s.defaultOn
	return s.repository.RegisterTelegramUser(ctx, input)
}

func (s *Service) SetUserTimezone(ctx context.Context, userID domain.UUID, timezoneName string) error {
	loc, err := time.LoadLocation(strings.TrimSpace(timezoneName))
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	return s.repository.UpdateUserTimezone(ctx, userID, loc.String())
}

func (s *Service) UserLocation(ctx context.Context, userID domain.UUID) *time.Location {
	if s.repository == nil {
		return s.timezone
	}
	user, err := s.repository.GetUser(ctx, userID)
	if err != nil || strings.TrimSpace(user.Timezone) == "" {
		return s.timezone
	}
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		return s.timezone
	}
	return loc
}

func (s *Service) SetEnabled(ctx context.Context, userID domain.UUID, enabled bool) error {
	return s.repository.SetAutonomyEnabled(ctx, userID, enabled)
}

func (s *Service) SetQuietHours(ctx context.Context, userID domain.UUID, quietStart string, quietEnd string) error {
	start, err := normalizeClock(quietStart)
	if err != nil {
		return fmt.Errorf("invalid quiet start: %w", err)
	}
	end, err := normalizeClock(quietEnd)
	if err != nil {
		return fmt.Errorf("invalid quiet end: %w", err)
	}
	return s.repository.UpdateAutonomyQuietHours(ctx, userID, start, end)
}

func (s *Service) SetDailyLimit(ctx context.Context, userID domain.UUID, limit int) error {
	if limit < 1 || limit > 12 {
		return fmt.Errorf("daily limit must be between 1 and 12")
	}
	return s.repository.UpdateAutonomyDailyLimit(ctx, userID, limit)
}

func (s *Service) SetRuleTime(ctx context.Context, userID domain.UUID, kind domain.NotificationKind, scheduleTime string) error {
	if kind == "" {
		return fmt.Errorf("notification kind is required")
	}
	clock, err := normalizeClock(scheduleTime)
	if err != nil {
		return err
	}
	return s.repository.UpdateNotificationRuleTime(ctx, userID, kind, clock)
}

func (s *Service) StatusText(ctx context.Context, userID domain.UUID) (string, error) {
	settings, err := s.repository.GetAutonomySettings(ctx, userID)
	if err != nil {
		return "", err
	}
	rules, err := s.repository.ListNotificationRules(ctx, userID)
	if err != nil {
		return "", err
	}
	status := "off"
	if settings.Enabled {
		status = "on"
	}
	lines := []string{
		"Autonomy: " + status,
		fmt.Sprintf("Quiet hours: %s-%s", trimSeconds(settings.QuietStart), trimSeconds(settings.QuietEnd)),
		fmt.Sprintf("Daily limit: %d proactive messages", settings.MaxMessagesPerDay),
		"",
		"Автономные сообщения:",
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", rule.Kind, trimSeconds(rule.ScheduleTime)))
	}
	lines = append(lines,
		"",
		"Команды:",
		"/autonomy on",
		"/autonomy off",
		"/autonomy quiet 23:30 08:00",
		"/autonomy limit 5",
		"/autonomy time daily_review 22:30",
	)
	return strings.Join(lines, "\n"), nil
}

func (s *Service) MarkDone(ctx context.Context, userID domain.UUID, notificationID domain.UUID) error {
	if err := s.repository.MarkNotificationActioned(ctx, userID, notificationID, domain.NotificationStatusDone); err != nil {
		return err
	}
	return s.logAction(ctx, userID, notificationID, "", "done", "")
}

func (s *Service) MarkSkipped(ctx context.Context, userID domain.UUID, notificationID domain.UUID) error {
	if err := s.repository.MarkNotificationActioned(ctx, userID, notificationID, domain.NotificationStatusSkipped); err != nil {
		return err
	}
	return s.logAction(ctx, userID, notificationID, "", "skipped", "")
}

func (s *Service) Snooze(ctx context.Context, userID domain.UUID, notificationID domain.UUID, duration time.Duration) error {
	if duration <= 0 {
		duration = 30 * time.Minute
	}
	if err := s.repository.SnoozeNotification(ctx, userID, notificationID, time.Now().Add(duration)); err != nil {
		return err
	}
	return s.logAction(ctx, userID, notificationID, "", "snoozed", duration.String())
}

func (s *Service) BuildReplanFromNotification(ctx context.Context, userID domain.UUID, notificationID domain.UUID) (*domain.ReplanProposal, error) {
	if s.planning == nil {
		return nil, fmt.Errorf("planning service is not configured")
	}
	notification, err := s.repository.GetScheduledNotificationForUser(ctx, userID, notificationID)
	if err != nil {
		return nil, err
	}
	if err := s.repository.MarkNotificationActioned(ctx, userID, notificationID, domain.NotificationStatusDone); err != nil {
		return nil, err
	}
	if err := s.logAction(ctx, userID, notificationID, notification.Kind, "replan_requested", ""); err != nil {
		return nil, err
	}
	return s.planning.BuildReplanProposal(ctx, userID, "Автономный check-in показал, что день надо реалистично перестроить. Сохрани fixed events и предложи новый план.", time.Now().In(s.timezone))
}

func (s *Service) runOnce(ctx context.Context) {
	if err := s.ensureSchedules(ctx); err != nil {
		s.logger.Error("failed to ensure notification schedules", "error", err)
	}
	if err := s.processDue(ctx); err != nil {
		s.logger.Error("failed to process due notifications", "error", err)
	}
}

func (s *Service) ensureSchedules(ctx context.Context) error {
	users, err := s.repository.ListAutonomyUsers(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, user := range users {
		if err := s.ensureUserSchedule(ctx, user, now); err != nil {
			s.logger.Error("failed to ensure user notification schedule", "error", err, "user_id", user.User.ID.String())
		}
	}
	return nil
}

func (s *Service) ensureUserSchedule(ctx context.Context, user domain.AutonomyUser, now time.Time) error {
	loc := s.userLocation(user)
	nowLocal := now.In(loc)
	for _, rule := range user.Rules {
		if !rule.Enabled || !s.kindAllowed(user.Settings, rule.Kind) {
			continue
		}
		if !ruleRunsToday(rule, nowLocal) {
			continue
		}
		dueLocal, ok := dueLocalTime(nowLocal, rule.ScheduleTime)
		if !ok {
			continue
		}
		if dueLocal.Before(nowLocal.Add(-10 * time.Minute)) {
			continue
		}
		key := fmt.Sprintf("%s:%s", rule.Kind, dueLocal.Format("2006-01-02"))
		notification := domain.ScheduledNotification{
			ID:              uuid.New(),
			UserID:          user.User.ID,
			ChatID:          user.User.DefaultChatID,
			Kind:            rule.Kind,
			NotificationKey: key,
			DueAt:           dueLocal.UTC(),
			Status:          domain.NotificationStatusPending,
			Payload:         map[string]any{"timezone": loc.String()},
		}
		if _, _, err := s.repository.UpsertScheduledNotification(ctx, notification); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processDue(ctx context.Context) error {
	notifications, err := s.repository.ListDueNotifications(ctx, time.Now().UTC(), defaultDueLimit)
	if err != nil {
		return err
	}
	for _, notification := range notifications {
		if err := s.processNotification(ctx, notification); err != nil {
			s.logger.Error("failed to process notification", "error", err, "notification_id", notification.ID.String())
		}
	}
	return nil
}

func (s *Service) processNotification(ctx context.Context, notification domain.ScheduledNotification) error {
	claimed, err := s.repository.ClaimScheduledNotification(ctx, notification.ID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if time.Since(notification.DueAt) > staleAfter {
		_ = s.repository.MarkNotificationActioned(ctx, notification.UserID, notification.ID, domain.NotificationStatusSkipped)
		return s.logAction(ctx, notification.UserID, notification.ID, notification.Kind, "skipped_stale", "")
	}

	settings, err := s.repository.GetAutonomySettings(ctx, notification.UserID)
	if err != nil {
		_ = s.repository.MarkNotificationFailed(ctx, notification.ID, err.Error())
		return err
	}
	loc := s.locationFromName(payloadString(notification.Payload, "timezone", s.timezone.String()))
	nowLocal := time.Now().In(loc)
	if inQuietHours(nowLocal, settings.QuietStart, settings.QuietEnd) {
		next := quietEndTime(nowLocal, settings.QuietEnd).UTC()
		if err := s.repository.SnoozeNotification(ctx, notification.UserID, notification.ID, next); err != nil {
			return err
		}
		return s.logAction(ctx, notification.UserID, notification.ID, notification.Kind, "snoozed_quiet_hours", next.Format(time.RFC3339))
	}

	dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).UTC()
	dayEnd := dayStart.Add(24 * time.Hour)
	count, err := s.repository.CountSentNotifications(ctx, notification.UserID, dayStart, dayEnd)
	if err != nil {
		_ = s.repository.MarkNotificationFailed(ctx, notification.ID, err.Error())
		return err
	}
	if count >= settings.MaxMessagesPerDay {
		_ = s.repository.MarkNotificationActioned(ctx, notification.UserID, notification.ID, domain.NotificationStatusSkipped)
		return s.logAction(ctx, notification.UserID, notification.ID, notification.Kind, "skipped_daily_limit", "")
	}

	text, buttons, err := s.buildNotification(ctx, notification, loc)
	if err != nil {
		_ = s.repository.MarkNotificationFailed(ctx, notification.ID, err.Error())
		return err
	}
	if err := s.sender.SendMessageWithButtons(ctx, notification.ChatID, text, buttons); err != nil {
		_ = s.repository.MarkNotificationFailed(ctx, notification.ID, err.Error())
		return err
	}
	if err := s.repository.MarkNotificationSent(ctx, notification.ID); err != nil {
		return err
	}
	return s.repository.LogNotification(ctx, domain.NotificationLog{
		UserID:                  notification.UserID,
		ScheduledNotificationID: &notification.ID,
		Kind:                    notification.Kind,
		Action:                  "sent",
		MessageText:             text,
		Metadata:                map[string]any{"chat_id": notification.ChatID},
	})
}

func (s *Service) buildNotification(ctx context.Context, notification domain.ScheduledNotification, loc *time.Location) (string, []telegram.InlineButton, error) {
	switch notification.Kind {
	case domain.NotificationKindDailyDirection:
		text := "Сегодня:\n1. Выйти из комнаты\n2. Движение 30+ минут\n3. Один deep work блок\n4. Shutdown без телефона"
		if s.planning != nil {
			direction, err := s.planning.BuildDailyDirection(ctx, notification.UserID, time.Now().In(loc))
			if err == nil {
				text = formatDailyDirection(*direction)
			}
		}
		return text, defaultButtons(notification.ID), nil
	case domain.NotificationKindMiddayCheckin:
		return "Проверка дня.\nЕсли утро уже съехало, не пытайся догнать всё. Выбери один следующий блок.\n\nСледующий шаг: 60 минут deep work или перестрой день.", replanButtons(notification.ID), nil
	case domain.NotificationKindPatternNudge:
		text := "Паттерн риска: если сейчас уходишь в телефон/кровать, день снова съедет.\n\nСледующий шаг: встать, вода, выйти на 20 минут."
		if s.patterns != nil {
			patterns, err := s.patterns.ListActive(ctx, notification.UserID)
			if err == nil && len(patterns) > 0 {
				text = formatPatternNudge(patterns[0])
			}
		}
		return text, defaultButtons(notification.ID), nil
	case domain.NotificationKindDailyReview:
		return dailyReviewText(), reviewButtons(notification.ID), nil
	case domain.NotificationKindShutdown:
		return "Shutdown.\nНе открывай новый цикл. Телефон на зарядку не у кровати, душ, свет вниз.\n\nСледующий шаг: закрыть день и лечь до целевого времени.", defaultButtons(notification.ID), nil
	case domain.NotificationKindWeeklyReview:
		text := "Weekly review.\nЧто работало, что ломало режим, главный паттерн, проблема и фокус следующей недели."
		if s.review != nil {
			now := time.Now().In(loc)
			weekly, err := s.review.BuildWeeklyReview(ctx, notification.UserID, startOfWeek(now))
			if err == nil {
				text = weekly
			}
		}
		return text, defaultButtons(notification.ID), nil
	default:
		return "", nil, fmt.Errorf("unsupported notification kind %q", notification.Kind)
	}
}

func (s *Service) userLocation(user domain.AutonomyUser) *time.Location {
	return s.locationFromName(user.User.Timezone)
}

func (s *Service) locationFromName(name string) *time.Location {
	if name == "" {
		return s.timezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return s.timezone
	}
	return loc
}

func (s *Service) kindAllowed(settings domain.AutonomySettings, kind domain.NotificationKind) bool {
	if len(settings.AllowedTypes) == 0 {
		return true
	}
	for _, allowed := range settings.AllowedTypes {
		if allowed == string(kind) {
			return true
		}
	}
	return false
}

func (s *Service) logAction(ctx context.Context, userID domain.UUID, notificationID domain.UUID, kind domain.NotificationKind, action string, note string) error {
	if kind == "" {
		notification, err := s.repository.GetScheduledNotificationForUser(ctx, userID, notificationID)
		if err == nil {
			kind = notification.Kind
		}
	}
	return s.repository.LogNotification(ctx, domain.NotificationLog{
		UserID:                  userID,
		ScheduledNotificationID: &notificationID,
		Kind:                    kind,
		Action:                  action,
		Metadata:                map[string]any{"note": note},
	})
}

func defaultButtons(id domain.UUID) []telegram.InlineButton {
	return []telegram.InlineButton{
		{Text: "Сделал", Data: "notify_done:" + id.String()},
		{Text: "Отложить 30м", Data: "notify_snooze30:" + id.String()},
		{Text: "Отложить 2ч", Data: "notify_snooze120:" + id.String()},
		{Text: "Пропустить", Data: "notify_skip:" + id.String()},
	}
}

func replanButtons(id domain.UUID) []telegram.InlineButton {
	return []telegram.InlineButton{
		{Text: "Перестроить", Data: "notify_replan:" + id.String()},
		{Text: "Сделал", Data: "notify_done:" + id.String()},
		{Text: "Отложить 30м", Data: "notify_snooze30:" + id.String()},
		{Text: "Пропустить", Data: "notify_skip:" + id.String()},
	}
}

func reviewButtons(id domain.UUID) []telegram.InlineButton {
	return []telegram.InlineButton{
		{Text: "Ответить", Data: "notify_review:" + id.String()},
		{Text: "Отложить 30м", Data: "notify_snooze30:" + id.String()},
		{Text: "Пропустить", Data: "notify_skip:" + id.String()},
	}
}

func formatDailyDirection(direction domain.DailyDirection) string {
	lines := []string{"Сегодня:"}
	for i, anchor := range direction.Anchors {
		if i >= 5 {
			break
		}
		line := fmt.Sprintf("%d. %s", i+1, anchor.Title)
		if anchor.Window != "" {
			line += " — " + anchor.Window
		}
		lines = append(lines, line)
	}
	if len(direction.Priorities) > 0 {
		lines = append(lines, "", "Приоритеты:")
		for i, priority := range direction.Priorities {
			if i >= 3 {
				break
			}
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, priority.Title))
		}
	}
	if direction.Text != "" {
		lines = append(lines, "", direction.Text)
	}
	return strings.Join(lines, "\n")
}

func formatPatternNudge(pattern domain.BehavioralPattern) string {
	action := "встать и сделать один внешний шаг"
	if len(pattern.CounterActions) > 0 {
		action = pattern.CounterActions[0]
	}
	return fmt.Sprintf("Паттерн риска: %s.\n%s\n\nСледующий шаг: %s.", pattern.Code, pattern.Description, action)
}

func dailyReviewText() string {
	return strings.Join([]string{
		"Daily review. Ответь одним сообщением:",
		"1. Что сделал?",
		"2. Что слил?",
		"3. Что помогло?",
		"4. Что ухудшило день?",
		"5. Что завтра обязательно?",
	}, "\n")
}

func ruleRunsToday(rule domain.NotificationRule, now time.Time) bool {
	if len(rule.ScheduleDOW) == 0 {
		return true
	}
	day := int(now.Weekday())
	if day == 0 {
		day = 7
	}
	for _, allowed := range rule.ScheduleDOW {
		if allowed == day {
			return true
		}
	}
	return false
}

func dueLocalTime(now time.Time, clock string) (time.Time, bool) {
	if clock == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("15:04:05", clock)
	if err != nil {
		parsed, err = time.Parse("15:04", clock)
		if err != nil {
			return time.Time{}, false
		}
	}
	return time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location()), true
}

func inQuietHours(now time.Time, quietStart string, quietEnd string) bool {
	start, okStart := dueLocalTime(now, quietStart)
	end, okEnd := dueLocalTime(now, quietEnd)
	if !okStart || !okEnd {
		return false
	}
	if start.Before(end) {
		return !now.Before(start) && now.Before(end)
	}
	return !now.Before(start) || now.Before(end)
}

func quietEndTime(now time.Time, quietEnd string) time.Time {
	end, ok := dueLocalTime(now, quietEnd)
	if !ok {
		return now.Add(time.Hour)
	}
	if end.Before(now) {
		end = end.Add(24 * time.Hour)
	}
	return end
}

func payloadString(payload map[string]any, key string, fallback string) string {
	value, ok := payload[key].(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func startOfWeek(value time.Time) time.Time {
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := value.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
}

func trimSeconds(value string) string {
	if len(value) >= 5 {
		return value[:5]
	}
	return value
}

func normalizeClock(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("time is required")
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		parsed, err = time.Parse("15:04:05", value)
		if err != nil {
			return "", fmt.Errorf("use HH:MM")
		}
	}
	return parsed.Format("15:04:05"), nil
}
