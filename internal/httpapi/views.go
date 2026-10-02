package httpapi

import (
	"time"

	"example.com/spillway-gate-coordinator/internal/contracts"
	"example.com/spillway-gate-coordinator/internal/domain"
)

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func gateView(gate domain.Gate) contracts.GateView {
	return contracts.GateView{
		ID:              gate.ID,
		Name:            gate.Name,
		Status:          string(gate.Status),
		Aperture:        gate.Aperture,
		MaxAperture:     gate.MaxAperture,
		ActiveCommandID: gate.ActiveCommandID,
		UpdatedAt:       formatTime(gate.UpdatedAt),
		Version:         gate.Version,
	}
}

func commandView(command domain.GateCommand, includeToken bool) contracts.CommandView {
	view := contracts.CommandView{
		ID:               command.ID,
		GateID:           command.GateID,
		TargetAperture:   command.TargetAperture,
		StartingAperture: command.StartingAperture,
		Reason:           command.Reason,
		RequestedBy:      command.RequestedBy,
		Status:           string(command.Status),
		ExecutedBy:       command.ExecutedBy,
		TerminalReason:   command.TerminalReason,
		CreatedAt:        formatTime(command.CreatedAt),
		UpdatedAt:        formatTime(command.UpdatedAt),
		Version:          command.Version,
	}

	if includeToken && command.Status == domain.CommandApproved {
		view.ExecutionToken = command.ExecutionToken
	}
	if command.Interlock != nil {
		view.Interlock = &contracts.InterlockDecisionView{
			Verdict:    string(command.Interlock.Verdict),
			Verifier:   command.Interlock.Verifier,
			Note:       command.Interlock.Note,
			ReviewedAt: formatTime(command.Interlock.ReviewedAt),
		}
	}
	return view
}

func alarmView(alarm domain.GateAlarm) contracts.AlarmView {
	view := contracts.AlarmView{
		ID:         alarm.ID,
		GateID:     alarm.GateID,
		Source:     alarm.Source,
		Level:      string(alarm.Level),
		Basis:      alarm.Basis,
		Note:       alarm.Note,
		ReportedBy: alarm.ReportedBy,
		Status:     string(alarm.Status),
		CreatedAt:  formatTime(alarm.CreatedAt),
		UpdatedAt:  formatTime(alarm.UpdatedAt),
		Version:    alarm.Version,
	}
	if alarm.Acknowledgement != nil {
		view.Acknowledgement = &contracts.AlarmAcknowledgementView{
			AcknowledgedBy: alarm.Acknowledgement.AcknowledgedBy,
			AcknowledgedAt: formatTime(alarm.Acknowledgement.AcknowledgedAt),
		}
	}
	if alarm.Closure != nil {
		view.Closure = &contracts.AlarmClosureView{
			HandledBy:  alarm.Closure.HandledBy,
			Resolution: alarm.Closure.Resolution,
			ClosedAt:   formatTime(alarm.Closure.ClosedAt),
		}
	}
	return view
}

func alarmViews(alarms []domain.GateAlarm) []contracts.AlarmView {
	views := make([]contracts.AlarmView, 0, len(alarms))
	for _, alarm := range alarms {
		views = append(views, alarmView(alarm))
	}
	return views
}
