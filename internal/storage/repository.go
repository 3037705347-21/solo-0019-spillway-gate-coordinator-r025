package storage

import (
	"context"
	"time"

	"example.com/spillway-gate-coordinator/internal/domain"
)

type Repository interface {
	CreateGate(ctx context.Context, gate domain.Gate) error
	Gate(ctx context.Context, id string) (domain.Gate, error)
	CreateCommand(ctx context.Context, command domain.GateCommand) error
	Command(ctx context.Context, id string) (domain.GateCommand, error)
	ReviewCommand(
		ctx context.Context,
		commandID string,
		decision domain.InterlockDecision,
		token string,
		now time.Time,
	) (domain.GateCommand, domain.Gate, error)
	ExecuteCommand(
		ctx context.Context,
		commandID string,
		token string,
		operator string,
		simulateFault bool,
		now time.Time,
	) (domain.GateCommand, domain.Gate, error)
	CreateAlarm(ctx context.Context, alarm domain.GateAlarm) (domain.GateAlarm, error)
	Alarm(ctx context.Context, id string) (domain.GateAlarm, error)
	GateAlarms(ctx context.Context, gateID string) ([]domain.GateAlarm, error)
	AcknowledgeAlarm(
		ctx context.Context,
		alarmID string,
		acknowledgedBy string,
		now time.Time,
	) (domain.GateAlarm, error)
	CloseAlarm(
		ctx context.Context,
		alarmID string,
		handledBy string,
		resolution string,
		now time.Time,
	) (domain.GateAlarm, error)
}
