package coordination

import (
	"context"

	"example.com/spillway-gate-coordinator/internal/contracts"
	"example.com/spillway-gate-coordinator/internal/domain"
)

func (s *Service) RegisterAlarm(
	ctx context.Context,
	gateID string,
	request contracts.RegisterAlarmRequest,
) (domain.GateAlarm, error) {
	if _, err := s.repository.Gate(ctx, gateID); err != nil {
		return domain.GateAlarm{}, err
	}

	alarmID, err := s.newID("alm")
	if err != nil {
		return domain.GateAlarm{}, domain.Internal("alarm_id_generation_failed", "alarm id could not be generated")
	}

	alarm, err := domain.NewGateAlarm(
		alarmID,
		gateID,
		request.Source,
		request.Level,
		request.Basis,
		request.Note,
		request.ReportedBy,
		s.now(),
	)
	if err != nil {
		return domain.GateAlarm{}, err
	}
	return s.repository.CreateAlarm(ctx, alarm)
}

func (s *Service) Alarm(ctx context.Context, id string) (domain.GateAlarm, error) {
	return s.repository.Alarm(ctx, id)
}

func (s *Service) GateAlarms(ctx context.Context, gateID string) ([]domain.GateAlarm, error) {
	return s.repository.GateAlarms(ctx, gateID)
}

func (s *Service) AcknowledgeAlarm(
	ctx context.Context,
	alarmID string,
	request contracts.AcknowledgeAlarmRequest,
) (domain.GateAlarm, error) {
	return s.repository.AcknowledgeAlarm(ctx, alarmID, request.AcknowledgedBy, s.now())
}

func (s *Service) CloseAlarm(
	ctx context.Context,
	alarmID string,
	request contracts.CloseAlarmRequest,
) (domain.GateAlarm, error) {
	return s.repository.CloseAlarm(ctx, alarmID, request.HandledBy, request.Resolution, s.now())
}
