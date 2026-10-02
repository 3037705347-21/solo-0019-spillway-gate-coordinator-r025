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

	alarmID, err := s.newID("alarm")
	if err != nil {
		return domain.GateAlarm{}, domain.Internal("alarm_id_generation_failed", "alarm id could not be generated")
	}

	alarm, err := domain.NewGateAlarm(
		alarmID,
		gateID,
		request.Source,
		request.Severity,
		request.Detail,
		request.RegisteredBy,
		s.now(),
	)
	if err != nil {
		return domain.GateAlarm{}, err
	}
	if err := s.repository.CreateAlarm(ctx, alarm); err != nil {
		return domain.GateAlarm{}, err
	}
	return alarm, nil
}

func (s *Service) Alarm(ctx context.Context, gateID string, alarmID string) (domain.GateAlarm, error) {
	return s.repository.Alarm(ctx, gateID, alarmID)
}

func (s *Service) Alarms(ctx context.Context, gateID string) ([]domain.GateAlarm, error) {
	return s.repository.Alarms(ctx, gateID)
}

func (s *Service) AcknowledgeAlarm(
	ctx context.Context,
	gateID string,
	alarmID string,
	request contracts.AcknowledgeAlarmRequest,
) (domain.GateAlarm, error) {
	return s.repository.AcknowledgeAlarm(ctx, gateID, alarmID, request.AcknowledgedBy, s.now())
}

func (s *Service) CloseAlarm(
	ctx context.Context,
	gateID string,
	alarmID string,
	request contracts.CloseAlarmRequest,
) (domain.GateAlarm, error) {
	return s.repository.CloseAlarm(
		ctx,
		gateID,
		alarmID,
		request.Resolution,
		request.ClosedBy,
		s.now(),
	)
}
