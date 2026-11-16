package entity

import "time"

type AcademicClass struct {
	ID           string    `json:"id"`
	Code         string    `json:"code"`
	CourseCode   string    `json:"course_code"`
	CourseName   string    `json:"course_name"`
	PeriodName   string    `json:"period_name"`
	Shift        int       `json:"shift"`
	StudentIDs   []int64   `json:"student_ids,omitempty"`
	AssistantIDs []int64   `json:"assistant_ids,omitempty"`
	Active       bool      `json:"active"`
	Scope        Scope     `json:"scope"`
	Version      int64     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AcademicSchedule struct {
	ID         string `json:"id"`
	ClassID    string `json:"class_id"`
	ClassCode  string `json:"class_code"`
	CourseName string `json:"course_name"`
	DayOfWeek  string `json:"day_of_week"`
	Shift      int    `json:"shift"`
	Room       string `json:"room"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	Scope      Scope  `json:"scope"`
	Version    int64  `json:"version"`
}

type AcademicAttendanceSession struct {
	ID         string     `json:"id"`
	ScheduleID string     `json:"schedule_id"`
	ClassID    string     `json:"class_id"`
	Title      string     `json:"title"`
	IsOpen     bool       `json:"is_open"`
	OpenedAt   time.Time  `json:"opened_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ClosedAt   *time.Time `json:"closed_at"`
	QRToken    string     `json:"qr_token,omitempty"`
	Scope      Scope      `json:"scope"`
	Version    int64      `json:"version"`
}

// AcademicAttendanceSessionRecord is persisted storage and never returned raw over HTTP.
type AcademicAttendanceSessionRecord struct {
	Session   AcademicAttendanceSession `json:"session"`
	TokenHash string                    `json:"token_hash"`
}

type AcademicScheduleView struct {
	Schedule    AcademicSchedule           `json:"schedule"`
	OpenSession *AcademicAttendanceSession `json:"open_session"`
}

type AcademicAttendanceRecord struct {
	ID           string     `json:"id"`
	SessionID    string     `json:"session_id"`
	ScheduleID   string     `json:"schedule_id"`
	ClassID      string     `json:"class_id"`
	UserID       int64      `json:"user_id"`
	UserName     string     `json:"user_name"`
	Status       string     `json:"status"`
	VerifiedByQR bool       `json:"verified_by_qr"`
	Validated    bool       `json:"validated"`
	CheckedInAt  time.Time  `json:"checked_in_at"`
	ValidatedBy  *int64     `json:"validated_by"`
	ValidatedAt  *time.Time `json:"validated_at"`
	Notes        string     `json:"notes"`
	Scope        Scope      `json:"scope"`
	Version      int64      `json:"version"`
}

type InventoryItem struct {
	ID                string    `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	TotalQuantity     int       `json:"total_quantity"`
	AvailableQuantity int       `json:"available_quantity"`
	RentalFee         int       `json:"rental_fee"`
	Status            string    `json:"status"`
	Scope             Scope     `json:"scope"`
	Version           int64     `json:"version"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type InventoryRental struct {
	ID              string     `json:"id"`
	ItemID          string     `json:"item_id"`
	ItemName        string     `json:"item_name"`
	BorrowerID      int64      `json:"borrower_id"`
	BorrowerName    string     `json:"borrower_name"`
	Quantity        int        `json:"quantity"`
	Purpose         string     `json:"purpose"`
	Status          string     `json:"status"`
	TotalFee        int        `json:"total_fee"`
	PaymentVerified bool       `json:"payment_verified"`
	RequestedAt     time.Time  `json:"requested_at"`
	DueAt           time.Time  `json:"due_at"`
	DecidedAt       *time.Time `json:"decided_at"`
	ReturnedAt      *time.Time `json:"returned_at"`
	DecidedBy       *int64     `json:"decided_by"`
	Scope           Scope      `json:"scope"`
	Version         int64      `json:"version"`
}

type AcademicNotification struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type AcademicLeaveRequest struct {
	ID             string     `json:"id"`
	UserID         int64      `json:"user_id"`
	UserName       string     `json:"user_name"`
	ScheduleID     string     `json:"schedule_id"`
	ClassID        string     `json:"class_id"`
	Kind           string     `json:"kind"`
	Reason         string     `json:"reason"`
	OriginalAt     time.Time  `json:"original_at"`
	Status         string     `json:"status"`
	RescheduleAt   *time.Time `json:"reschedule_at,omitempty"`
	RescheduledBy  *int64     `json:"rescheduled_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecidedBy      *int64     `json:"decided_by,omitempty"`
	Scope          Scope      `json:"scope"`
	Version        int64      `json:"version"`
}

type AcademicFinanceMutation struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	Type      string    `json:"type"`
	Note      string    `json:"note"`
	Amount    int64     `json:"amount"`
	Category  string    `json:"category"`
	RentalID  string    `json:"rental_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Scope     Scope     `json:"scope"`
	Version   int64     `json:"version"`
}

type AcademicFinanceBalance struct {
	UserID    int64              `json:"user_id"`
	UserName  string             `json:"user_name"`
	Saldo     int64              `json:"saldo"`
	Income    int64              `json:"income"`
	Expense   int64              `json:"expense"`
	Adjust    int64              `json:"adjustment"`
	Mutations []AcademicFinanceMutation `json:"mutations"`
	Timezone  string             `json:"timezone"`
}

type AcademicReportSummary struct {
	ClassesCount           int            `json:"classes_count"`
	SchedulesCount         int            `json:"schedules_count"`
	OpenAttendanceSessions int            `json:"open_attendance_sessions"`
	AttendanceByStatus     map[string]int `json:"attendance_by_status"`
	TotalInventoryItems    int            `json:"total_inventory_items"`
	AvailableInventoryUnit int            `json:"available_inventory_units"`
	ActiveRentalsCount     int            `json:"active_rentals_count"`
	TotalVerifiedRevenue   int            `json:"total_verified_revenue"`
	CSVExport              string         `json:"csv_export"`
	GeneratedAt            time.Time      `json:"generated_at"`
	Timezone               string         `json:"timezone"`
	TimezoneOffset         string         `json:"timezone_offset"`
}
