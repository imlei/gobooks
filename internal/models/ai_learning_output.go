package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	AIJobReportUsageLearning     = "report_usage_learning"
	AIJobDashboardRecommendation = "dashboard_recommendation"
	AIJobActionCenterGeneration  = "action_center_generation"
	AIJobStatusSkipped           = "skipped"

	ReportUsageOpened               = "report_opened"
	ReportUsageFiltered             = "report_filtered"
	ReportUsageExported             = "report_exported"
	ReportUsagePrinted              = "report_printed"
	ReportUsageDrilldownClicked     = "report_drilldown_clicked"
	ReportUsageAddedToDashboard     = "report_added_to_dashboard"
	ReportUsageRemovedFromDashboard = "report_removed_from_dashboard"
	ReportUsageSuggestionAccepted   = "report_suggestion_accepted"
	ReportUsageSuggestionDismissed  = "report_suggestion_dismissed"

	DashboardWidgetSourceUser          = "user"
	DashboardWidgetSourceSuggestion    = "suggestion"
	DashboardWidgetSourceSystemDefault = "system_default"

	DashboardSuggestionSourceSystem = "system"
	DashboardSuggestionSourceAI     = "ai"
	DashboardSuggestionPending      = "pending"
	DashboardSuggestionAccepted     = "accepted"
	DashboardSuggestionDismissed    = "dismissed"
	DashboardSuggestionSnoozed      = "snoozed"
	DashboardSuggestionExpired      = "expired"

	ActionTaskPriorityLow    = "low"
	ActionTaskPriorityMedium = "medium"
	ActionTaskPriorityHigh   = "high"
	ActionTaskPriorityUrgent = "urgent"

	ActionTaskStatusOpen       = "open"
	ActionTaskStatusInProgress = "in_progress"
	ActionTaskStatusDone       = "done"
	ActionTaskStatusDismissed  = "dismissed"
	ActionTaskStatusSnoozed    = "snoozed"
	ActionTaskStatusExpired    = "expired"
	ActionTaskStatusBlocked    = "blocked"

	ActionTaskEventCreated       = "created"
	ActionTaskEventViewed        = "viewed"
	ActionTaskEventStarted       = "started"
	ActionTaskEventCompleted     = "completed"
	ActionTaskEventDismissed     = "dismissed"
	ActionTaskEventSnoozed       = "snoozed"
	ActionTaskEventExpired       = "expired"
	ActionTaskEventReopened      = "reopened"
	ActionTaskEventClickedAction = "clicked_action"
)

type ReportUsageEvent struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CompanyID    uint       `gorm:"not null;index:idx_report_usage_company_user_report_created,priority:1;index:idx_report_usage_company_report_event_created,priority:1;index:idx_report_usage_company_event_created,priority:1"`
	UserID       *uuid.UUID `gorm:"type:uuid;index:idx_report_usage_company_user_report_created,priority:2"`
	ReportKey    string     `gorm:"type:text;not null;index:idx_report_usage_company_user_report_created,priority:3;index:idx_report_usage_company_report_event_created,priority:2"`
	EventType    string     `gorm:"type:text;not null;index:idx_report_usage_company_report_event_created,priority:3;index:idx_report_usage_company_event_created,priority:2"`
	DateRangeKey string     `gorm:"type:text"`
	FiltersJSON  string     `gorm:"column:filters_json;type:jsonb"`
	SourceRoute  string     `gorm:"type:text"`
	MetadataJSON string     `gorm:"column:metadata_json;type:jsonb"`
	CreatedAt    time.Time  `gorm:"not null;index:idx_report_usage_company_user_report_created,priority:4;index:idx_report_usage_company_report_event_created,priority:4;index:idx_report_usage_company_event_created,priority:3"`
}

func (ReportUsageEvent) TableName() string { return "report_usage_events" }

func (m *ReportUsageEvent) BeforeCreate(_ *gorm.DB) error {
	ensureUUID(&m.ID)
	return nil
}

type ReportUsageStat struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CompanyID          uint       `gorm:"not null;uniqueIndex:uq_report_usage_scope,priority:1"`
	ScopeType          string     `gorm:"type:text;not null;uniqueIndex:uq_report_usage_scope,priority:2"`
	UserID             *uuid.UUID `gorm:"type:uuid;uniqueIndex:uq_report_usage_scope,priority:3"`
	ReportKey          string     `gorm:"type:text;not null;uniqueIndex:uq_report_usage_scope,priority:4"`
	OpenCount          int        `gorm:"not null;default:0"`
	ExportCount        int        `gorm:"not null;default:0"`
	PrintCount         int        `gorm:"not null;default:0"`
	DrilldownCount     int        `gorm:"not null;default:0"`
	FilterCount        int        `gorm:"not null;default:0"`
	LastOpenedAt       *time.Time
	LastUsedAt         *time.Time
	CommonDateRangeKey string    `gorm:"type:text"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (ReportUsageStat) TableName() string { return "report_usage_stats" }

func (m *ReportUsageStat) BeforeCreate(_ *gorm.DB) error {
	ensureUUID(&m.ID)
	return nil
}

type DashboardUserWidget struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CompanyID  uint       `gorm:"not null;uniqueIndex:uq_dashboard_user_widget,priority:1"`
	UserID     *uuid.UUID `gorm:"type:uuid;uniqueIndex:uq_dashboard_user_widget,priority:2"`
	WidgetKey  string     `gorm:"type:text;not null;uniqueIndex:uq_dashboard_user_widget,priority:3"`
	Title      string     `gorm:"type:text"`
	ConfigJSON string     `gorm:"column:config_json;type:jsonb"`
	Position   *int
	Source     string    `gorm:"type:text;not null"`
	Active     bool      `gorm:"not null;default:true;index"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (DashboardUserWidget) TableName() string { return "dashboard_user_widgets" }

func (m *DashboardUserWidget) BeforeCreate(_ *gorm.DB) error {
	ensureUUID(&m.ID)
	return nil
}

type DashboardWidgetSuggestion struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID    uint            `gorm:"not null;index:idx_dashboard_suggestion_status,priority:1;index:idx_dashboard_suggestion_widget_status,priority:1"`
	UserID       *uuid.UUID      `gorm:"type:uuid;index:idx_dashboard_suggestion_status,priority:2"`
	WidgetKey    string          `gorm:"type:text;not null;index:idx_dashboard_suggestion_widget_status,priority:2"`
	Title        string          `gorm:"type:text;not null"`
	Reason       string          `gorm:"type:text;not null"`
	EvidenceJSON string          `gorm:"column:evidence_json;type:jsonb"`
	Confidence   decimal.Decimal `gorm:"type:numeric(8,4);not null;default:0"`
	Source       string          `gorm:"type:text;not null"`
	Status       string          `gorm:"type:text;not null;index:idx_dashboard_suggestion_status,priority:3;index:idx_dashboard_suggestion_widget_status,priority:3"`
	JobRunID     *uuid.UUID      `gorm:"type:uuid;index"`
	AcceptedAt   *time.Time
	DismissedAt  *time.Time
	SnoozedUntil *time.Time
	CreatedAt    time.Time `gorm:"not null;index:idx_dashboard_suggestion_status,priority:4"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (DashboardWidgetSuggestion) TableName() string { return "dashboard_widget_suggestions" }

func (m *DashboardWidgetSuggestion) BeforeCreate(_ *gorm.DB) error {
	ensureUUID(&m.ID)
	return nil
}

type ActionCenterTask struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CompanyID      uint       `gorm:"not null;uniqueIndex:uq_action_task_fingerprint,priority:1;index:idx_action_task_status_due,priority:1;index:idx_action_task_user_status,priority:1;index:idx_action_task_type_status,priority:1"`
	AssignedUserID *uuid.UUID `gorm:"type:uuid;index:idx_action_task_user_status,priority:2"`
	TaskType       string     `gorm:"type:text;not null;index:idx_action_task_type_status,priority:2"`
	SourceEngine   string     `gorm:"type:text;not null"`
	SourceType     string     `gorm:"type:text;not null"`
	SourceObjectID *uint
	Title          string          `gorm:"type:text;not null"`
	Description    string          `gorm:"type:text"`
	Reason         string          `gorm:"type:text;not null"`
	EvidenceJSON   string          `gorm:"column:evidence_json;type:jsonb"`
	Priority       string          `gorm:"type:text;not null"`
	DueDate        *time.Time      `gorm:"type:date;index:idx_action_task_status_due,priority:3"`
	ActionURL      string          `gorm:"type:text"`
	Status         string          `gorm:"type:text;not null;index:idx_action_task_status_due,priority:2;index:idx_action_task_user_status,priority:3;index:idx_action_task_type_status,priority:3"`
	Fingerprint    string          `gorm:"type:text;not null;uniqueIndex:uq_action_task_fingerprint,priority:2"`
	AIGenerated    bool            `gorm:"not null;default:false"`
	Confidence     decimal.Decimal `gorm:"type:numeric(8,4);not null;default:0"`
	CreatedAt      time.Time       `gorm:"not null"`
	UpdatedAt      time.Time       `gorm:"not null"`
	CompletedAt    *time.Time
	DismissedAt    *time.Time
	SnoozedUntil   *time.Time
}

func (ActionCenterTask) TableName() string { return "action_center_tasks" }

func (m *ActionCenterTask) BeforeCreate(_ *gorm.DB) error {
	ensureUUID(&m.ID)
	return nil
}

type ActionCenterTaskEvent struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CompanyID    uint       `gorm:"not null;index:idx_action_task_event_task_created,priority:1;index:idx_action_task_event_user_created,priority:1"`
	TaskID       uuid.UUID  `gorm:"type:uuid;not null;index:idx_action_task_event_task_created,priority:2"`
	UserID       *uuid.UUID `gorm:"type:uuid;index:idx_action_task_event_user_created,priority:2"`
	EventType    string     `gorm:"type:text;not null"`
	MetadataJSON string     `gorm:"column:metadata_json;type:jsonb"`
	CreatedAt    time.Time  `gorm:"not null;index:idx_action_task_event_task_created,priority:3;index:idx_action_task_event_user_created,priority:3"`
}

func (ActionCenterTaskEvent) TableName() string { return "action_center_task_events" }

func (m *ActionCenterTaskEvent) BeforeCreate(_ *gorm.DB) error {
	ensureUUID(&m.ID)
	return nil
}
