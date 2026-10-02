package domain

import (
	"strings"
	"time"
)

type AlarmLevel string

const (
	// AlarmAdvisory 提示级告警：记录现场异常，但不阻断命令占用。
	AlarmAdvisory AlarmLevel = "advisory"
	// AlarmBlocking 阻断级告警：未关闭前禁止新命令占用闸门。
	AlarmBlocking AlarmLevel = "blocking"
)

type AlarmStatus string

const (
	AlarmOpen         AlarmStatus = "open"
	AlarmAcknowledged AlarmStatus = "acknowledged"
	AlarmClosed       AlarmStatus = "closed"
)

// AlarmAcknowledgement 记录“现场已经看到”的事实，只追加，不改写。
type AlarmAcknowledgement struct {
	AcknowledgedBy string
	AcknowledgedAt time.Time
}

// AlarmClosure 记录“现场已经处理”的当前处置事实，只追加，不改写。
type AlarmClosure struct {
	HandledBy  string
	Resolution string
	ClosedAt   time.Time
}

// GateAlarm 是独立于闸门锁存状态的告警记录。登记后只能通过确认、关闭
// 追加处置事实，登记时的来源、级别、依据等历史字段不允许被改写。
type GateAlarm struct {
	ID              string
	GateID          string
	Source          string
	Level           AlarmLevel
	Basis           string
	Note            string
	ReportedBy      string
	Status          AlarmStatus
	Acknowledgement *AlarmAcknowledgement
	Closure         *AlarmClosure
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int64
}

func NewGateAlarm(
	alarmID string,
	gateID string,
	source string,
	level string,
	basis string,
	note string,
	reportedBy string,
	now time.Time,
) (GateAlarm, error) {
	alarmID = strings.TrimSpace(alarmID)
	if alarmID == "" {
		return GateAlarm{}, Invalid("invalid_alarm_id", "alarm id is required")
	}

	gateID = strings.TrimSpace(gateID)
	if gateID == "" {
		return GateAlarm{}, Invalid("invalid_gate_reference", "gate id is required")
	}

	source = strings.TrimSpace(source)
	if source == "" {
		return GateAlarm{}, Invalid("invalid_alarm_source", "alarm source is required")
	}

	normalizedLevel := AlarmLevel(strings.ToLower(strings.TrimSpace(level)))
	if normalizedLevel != AlarmAdvisory && normalizedLevel != AlarmBlocking {
		return GateAlarm{}, Invalid("invalid_alarm_level", "alarm level must be advisory or blocking")
	}

	basis = strings.TrimSpace(basis)
	if basis == "" {
		return GateAlarm{}, Invalid("invalid_alarm_basis", "alarm basis is required")
	}

	reportedBy = strings.TrimSpace(reportedBy)
	if reportedBy == "" {
		return GateAlarm{}, Invalid("invalid_alarm_reporter", "reported_by is required")
	}

	return GateAlarm{
		ID:         alarmID,
		GateID:     gateID,
		Source:     source,
		Level:      normalizedLevel,
		Basis:      basis,
		Note:       strings.TrimSpace(note),
		ReportedBy: reportedBy,
		Status:     AlarmOpen,
		CreatedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
		Version:    1,
	}, nil
}

// Acknowledge 记录现场已看到告警。确认不代表关闭，阻断级告警确认后仍然阻断。
func (a *GateAlarm) Acknowledge(acknowledgedBy string, now time.Time) error {
	acknowledgedBy = strings.TrimSpace(acknowledgedBy)
	if acknowledgedBy == "" {
		return Invalid("invalid_acknowledged_by", "acknowledged_by is required")
	}

	switch a.Status {
	case AlarmAcknowledged:
		return Conflict("alarm_already_acknowledged", "alarm has already been acknowledged")
	case AlarmClosed:
		return Conflict("alarm_already_closed", "closed alarms cannot be acknowledged")
	}

	a.Acknowledgement = &AlarmAcknowledgement{
		AcknowledgedBy: acknowledgedBy,
		AcknowledgedAt: now.UTC(),
	}
	a.Status = AlarmAcknowledged
	a.UpdatedAt = now.UTC()
	a.Version++
	return nil
}

// Close 基于当前处置事实关闭告警。关闭前必须先确认，且不能重复关闭，
// 处置事实作为独立记录追加，登记时的历史字段保持不变。
func (a *GateAlarm) Close(handledBy, resolution string, now time.Time) error {
	handledBy = strings.TrimSpace(handledBy)
	if handledBy == "" {
		return Invalid("invalid_handled_by", "handled_by is required")
	}

	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return Invalid("invalid_alarm_resolution", "closing an alarm requires current handling facts")
	}

	switch a.Status {
	case AlarmOpen:
		return Conflict("alarm_not_acknowledged", "alarm must be acknowledged before it can be closed")
	case AlarmClosed:
		return Conflict("alarm_already_closed", "alarm has already been closed")
	}

	a.Closure = &AlarmClosure{
		HandledBy:  handledBy,
		Resolution: resolution,
		ClosedAt:   now.UTC(),
	}
	a.Status = AlarmClosed
	a.UpdatedAt = now.UTC()
	a.Version++
	return nil
}

func (a GateAlarm) IsBlocking() bool {
	return a.Level == AlarmBlocking
}

// BlocksCommands 表示告警当前是否阻止新命令占用：只有阻断级且尚未关闭时成立，
// 已确认但未关闭的阻断级告警仍然阻止占用。
func (a GateAlarm) BlocksCommands() bool {
	return a.IsBlocking() && a.Status != AlarmClosed
}
