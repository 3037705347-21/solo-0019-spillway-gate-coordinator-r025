package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"example.com/spillway-gate-coordinator/internal/domain"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	gates    map[string]domain.Gate
	commands map[string]domain.GateCommand
	alarms   map[string]domain.GateAlarm
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		gates:    make(map[string]domain.Gate),
		commands: make(map[string]domain.GateCommand),
		alarms:   make(map[string]domain.GateAlarm),
	}
}

func (m *MemoryRepository) CreateGate(_ context.Context, gate domain.Gate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.gates[gate.ID]; exists {
		return domain.Conflict("duplicate_gate_id", "gate id already exists")
	}
	m.gates[gate.ID] = gate
	return nil
}

func (m *MemoryRepository) Gate(_ context.Context, id string) (domain.Gate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gate, exists := m.gates[id]
	if !exists {
		return domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}
	return gate, nil
}

func (m *MemoryRepository) CreateCommand(_ context.Context, command domain.GateCommand) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.NotFound("gate_not_found", "gate id was not found")
	}
	if gate.Status == domain.GateFaulted {
		return domain.Conflict("gate_faulted", "faulted gates cannot receive new commands")
	}
	if command.StartingAperture != gate.Aperture {
		return domain.Conflict("gate_aperture_changed", "gate aperture changed before the command was stored")
	}
	if command.TargetAperture > gate.MaxAperture {
		return domain.Invalid("invalid_target_aperture", "target aperture is outside the gate limit")
	}
	if _, exists := m.commands[command.ID]; exists {
		return domain.Conflict("duplicate_command_id", "command id already exists")
	}

	m.commands[command.ID] = command
	return nil
}

func (m *MemoryRepository) Command(_ context.Context, id string) (domain.GateCommand, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	command, exists := m.commands[id]
	if !exists {
		return domain.GateCommand{}, domain.NotFound("command_not_found", "command id was not found")
	}
	return cloneCommand(command), nil
}

func (m *MemoryRepository) ReviewCommand(
	_ context.Context,
	commandID string,
	decision domain.InterlockDecision,
	token string,
	now time.Time,
) (domain.GateCommand, domain.Gate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	command, exists := m.commands[commandID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("command_not_found", "command id was not found")
	}
	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}

	nextCommand := cloneCommand(command)
	if err := nextCommand.Review(decision, token, now); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	nextGate := gate
	if decision.IsClear() {
		if err := m.ensureGateNotBlockedLocked(command.GateID); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
		if err := nextGate.Reserve(nextCommand.ID, now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
	}

	m.commands[nextCommand.ID] = nextCommand
	m.gates[nextGate.ID] = nextGate
	return cloneCommand(nextCommand), nextGate, nil
}

func (m *MemoryRepository) ExecuteCommand(
	_ context.Context,
	commandID string,
	token string,
	operator string,
	simulateFault bool,
	now time.Time,
) (domain.GateCommand, domain.Gate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	command, exists := m.commands[commandID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("command_not_found", "command id was not found")
	}
	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}

	nextCommand := cloneCommand(command)
	if err := nextCommand.BeginExecution(token, operator, now); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	nextGate := gate
	if err := nextGate.StartMovement(nextCommand.ID, now); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	if simulateFault {
		if err := nextGate.Settle(nextCommand.TargetAperture, true, now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
		if err := nextCommand.Abort("simulated_gate_fault", now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
	} else {
		if err := nextGate.Settle(nextCommand.TargetAperture, false, now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
		if err := nextCommand.Complete(now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
	}

	m.commands[nextCommand.ID] = nextCommand
	m.gates[nextGate.ID] = nextGate
	return cloneCommand(nextCommand), nextGate, nil
}

func cloneCommand(command domain.GateCommand) domain.GateCommand {
	if command.Interlock == nil {
		return command
	}
	decision := *command.Interlock
	command.Interlock = &decision
	return command
}

func (m *MemoryRepository) CreateAlarm(_ context.Context, alarm domain.GateAlarm) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.gates[alarm.GateID]; !exists {
		return domain.NotFound("gate_not_found", "gate id was not found")
	}
	if _, exists := m.alarms[alarm.ID]; exists {
		return domain.Conflict("duplicate_alarm_id", "alarm id already exists")
	}

	m.alarms[alarm.ID] = alarm
	return nil
}

func (m *MemoryRepository) Alarm(_ context.Context, gateID string, alarmID string) (domain.GateAlarm, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.requireGateLocked(gateID); err != nil {
		return domain.GateAlarm{}, err
	}
	alarm, exists := m.alarms[alarmID]
	if !exists || alarm.GateID != gateID {
		return domain.GateAlarm{}, domain.NotFound("alarm_not_found", "alarm id was not found for this gate")
	}
	return cloneAlarm(alarm), nil
}

func (m *MemoryRepository) Alarms(_ context.Context, gateID string) ([]domain.GateAlarm, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.requireGateLocked(gateID); err != nil {
		return nil, err
	}

	alarms := make([]domain.GateAlarm, 0)
	for _, alarm := range m.alarms {
		if alarm.GateID == gateID {
			alarms = append(alarms, cloneAlarm(alarm))
		}
	}
	sort.Slice(alarms, func(i, j int) bool {
		if !alarms[i].CreatedAt.Equal(alarms[j].CreatedAt) {
			return alarms[i].CreatedAt.Before(alarms[j].CreatedAt)
		}
		return alarms[i].ID < alarms[j].ID
	})
	return alarms, nil
}

func (m *MemoryRepository) AcknowledgeAlarm(
	_ context.Context,
	gateID string,
	alarmID string,
	acknowledgedBy string,
	now time.Time,
) (domain.GateAlarm, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alarm, err := m.gateAlarmLocked(gateID, alarmID)
	if err != nil {
		return domain.GateAlarm{}, err
	}
	if err := alarm.Acknowledge(acknowledgedBy, now); err != nil {
		return domain.GateAlarm{}, err
	}
	m.alarms[alarm.ID] = alarm
	return cloneAlarm(alarm), nil
}

func (m *MemoryRepository) CloseAlarm(
	_ context.Context,
	gateID string,
	alarmID string,
	resolution string,
	closedBy string,
	now time.Time,
) (domain.GateAlarm, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alarm, err := m.gateAlarmLocked(gateID, alarmID)
	if err != nil {
		return domain.GateAlarm{}, err
	}
	if err := alarm.Close(resolution, closedBy, now); err != nil {
		return domain.GateAlarm{}, err
	}
	m.alarms[alarm.ID] = alarm
	return cloneAlarm(alarm), nil
}

func (m *MemoryRepository) requireGateLocked(gateID string) error {
	if _, exists := m.gates[gateID]; !exists {
		return domain.NotFound("gate_not_found", "gate id was not found")
	}
	return nil
}

func (m *MemoryRepository) gateAlarmLocked(gateID, alarmID string) (domain.GateAlarm, error) {
	if _, exists := m.gates[gateID]; !exists {
		return domain.GateAlarm{}, domain.NotFound("gate_not_found", "gate id was not found")
	}
	alarm, exists := m.alarms[alarmID]
	if !exists || alarm.GateID != gateID {
		return domain.GateAlarm{}, domain.NotFound("alarm_not_found", "alarm id was not found for this gate")
	}
	return alarm, nil
}

// ensureGateNotBlockedLocked must run while holding m.mu.Lock(). It shares
// the same critical section as alarm registration/closure and gate
// reservation, so a concurrent review can never reserve a gate that has an
// unclosed blocking alarm.
func (m *MemoryRepository) ensureGateNotBlockedLocked(gateID string) error {
	for _, alarm := range m.alarms {
		if alarm.GateID == gateID && alarm.IsBlocking() {
			return domain.Conflict(
				"gate_alarm_blocking",
				"an unclosed blocking alarm prevents new command occupation",
			)
		}
	}
	return nil
}

func cloneAlarm(alarm domain.GateAlarm) domain.GateAlarm {
	if alarm.AcknowledgedAt != nil {
		at := *alarm.AcknowledgedAt
		alarm.AcknowledgedAt = &at
	}
	if alarm.ClosedAt != nil {
		at := *alarm.ClosedAt
		alarm.ClosedAt = &at
	}
	return alarm
}
