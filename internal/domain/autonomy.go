package domain

import "time"

type User struct {
	ID             UUID
	TelegramUserID int64
	DefaultChatID  int64
	Username       string
	FirstName      string
	LastName       string
	Timezone       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastSeenAt     time.Time
}

type RegisterTelegramUserInput struct {
	TelegramUserID         int64
	ChatID                 int64
	Username               string
	FirstName              string
	LastName               string
	Timezone               string
	DefaultAutonomyEnabled bool
}

type AutonomySettings struct {
	UserID            UUID
	Enabled           bool
	QuietStart        string
	QuietEnd          string
	MaxMessagesPerDay int
	MorningTime       string
	MiddayTime        string
	ShutdownTime      string
	ReviewTime        string
	WeeklyTime        string
	AllowedTypes      []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type NotificationRule struct {
	ID           UUID
	UserID       UUID
	Kind         NotificationKind
	Enabled      bool
	ScheduleTime string
	ScheduleDOW  []int
	Payload      map[string]any
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type NotificationKind string

const (
	NotificationKindDailyDirection NotificationKind = "daily_direction"
	NotificationKindMiddayCheckin  NotificationKind = "midday_checkin"
	NotificationKindShutdown       NotificationKind = "shutdown"
	NotificationKindDailyReview    NotificationKind = "daily_review"
	NotificationKindWeeklyReview   NotificationKind = "weekly_review"
	NotificationKindPatternNudge   NotificationKind = "pattern_nudge"
)

type NotificationStatus string

const (
	NotificationStatusPending NotificationStatus = "pending"
	NotificationStatusSending NotificationStatus = "sending"
	NotificationStatusSent    NotificationStatus = "sent"
	NotificationStatusDone    NotificationStatus = "done"
	NotificationStatusSkipped NotificationStatus = "skipped"
	NotificationStatusSnoozed NotificationStatus = "snoozed"
	NotificationStatusFailed  NotificationStatus = "failed"
)

type ScheduledNotification struct {
	ID              UUID
	UserID          UUID
	ChatID          int64
	Kind            NotificationKind
	NotificationKey string
	DueAt           time.Time
	Status          NotificationStatus
	Payload         map[string]any
	Attempts        int
	LockedAt        *time.Time
	SentAt          *time.Time
	ActionedAt      *time.Time
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type NotificationLog struct {
	ID                      UUID
	UserID                  UUID
	ScheduledNotificationID *UUID
	Kind                    NotificationKind
	Action                  string
	MessageText             string
	Metadata                map[string]any
	CreatedAt               time.Time
}

type AutonomyUser struct {
	User     User
	Settings AutonomySettings
	Rules    []NotificationRule
}
