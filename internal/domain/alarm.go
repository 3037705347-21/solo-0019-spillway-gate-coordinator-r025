package domain

import (
	"strings"
	"time"
)

type AlarmSeverity string

const (
	SeverityAdvisory AlarmSeverity = "advisory"
	SeverityBlocking AlarmSeverity = "blocking"
)

type AlarmStatus string

const (
	AlarmOpen         AlarmStatus = "open"
	AlarmAcknowledged AlarmStatus = "acknowledged"
	AlarmClosed       AlarmStatus = "closed"
)

// GateAlarm is an independent abnormality record for a gate. It exists on its
// own rather than being latched onto a failed command: monitoring or duty
// staff register the anomaly source, severity, and later record whether the
// site has seen (acknowledged) and handled (closed) it.
type GateAlarm struct {
	ID             string
	GateID         string
	Source         string
	Severity       AlarmSeverity
	Detail         string
	RegisteredBy   string
	Status         AlarmStatus
	AcknowledgedBy string
	AcknowledgedAt *time.Time
	Resolution     string
	ClosedBy       string
	ClosedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

func NewGateAlarm(
	alarmID string,
	gateID string,
	source string,
	severity string,
	detail string,
	registeredBy string,
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

	normalizedSeverity := AlarmSeverity(strings.ToLower(strings.TrimSpace(severity)))
	if normalizedSeverity != SeverityAdvisory && normalizedSeverity != SeverityBlocking {
		return GateAlarm{}, Invalid("invalid_alarm_severity", "severity must be advisory or blocking")
	}

	registeredBy = strings.TrimSpace(registeredBy)
	if registeredBy == "" {
		return GateAlarm{}, Invalid("invalid_registered_by", "registered_by is required")
	}

	return GateAlarm{
		ID:           alarmID,
		GateID:       gateID,
		Source:       source,
		Severity:     normalizedSeverity,
		Detail:       strings.TrimSpace(detail),
		RegisteredBy: registeredBy,
		Status:       AlarmOpen,
		CreatedAt:    now.UTC(),
		UpdatedAt:    now.UTC(),
		Version:      1,
	}, nil
}

// Acknowledge records that on-site staff have seen the alarm. It is an
// independent step that must never be treated as closure: an acknowledged
// blocking alarm still blocks new command occupation.
func (a *GateAlarm) Acknowledge(acknowledgedBy string, now time.Time) error {
	if a.Status == AlarmClosed {
		return Conflict("alarm_closed", "closed alarms cannot be acknowledged")
	}
	if a.Status == AlarmAcknowledged {
		return Conflict("alarm_already_acknowledged", "alarm has already been acknowledged")
	}

	acknowledgedBy = strings.TrimSpace(acknowledgedBy)
	if acknowledgedBy == "" {
		return Invalid("invalid_acknowledged_by", "acknowledged_by is required")
	}

	at := now.UTC()
	a.Status = AlarmAcknowledged
	a.AcknowledgedBy = acknowledgedBy
	a.AcknowledgedAt = &at
	a.UpdatedAt = at
	a.Version++
	return nil
}

// Close records the current disposition facts that justify taking the alarm
// out of effect. Closing is terminal: a closed alarm can never be reopened
// and its resolution basis can never be rewritten.
func (a *GateAlarm) Close(resolution string, closedBy string, now time.Time) error {
	if a.Status == AlarmClosed {
		return Conflict("alarm_already_closed", "alarm has already been closed")
	}

	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return Invalid("invalid_alarm_resolution", "closing an alarm requires the current disposition basis")
	}

	closedBy = strings.TrimSpace(closedBy)
	if closedBy == "" {
		return Invalid("invalid_closed_by", "closed_by is required")
	}

	at := now.UTC()
	a.Status = AlarmClosed
	a.Resolution = resolution
	a.ClosedBy = closedBy
	a.ClosedAt = &at
	a.UpdatedAt = at
	a.Version++
	return nil
}

// IsBlocking reports whether the alarm currently prevents new commands from
// occupying the gate. Only a closed alarm stops blocking; acknowledgement
// alone does not.
func (a GateAlarm) IsBlocking() bool {
	return a.Severity == SeverityBlocking && a.Status != AlarmClosed
}
