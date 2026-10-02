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
	// 阻断级告警只拦截“占用”（clear 复核预留闸门），不拦截不占用闸门的待复核申请。
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
		if alarm, blocked := m.openBlockingAlarm(command.GateID); blocked {
			return domain.GateCommand{}, domain.Gate{}, domain.Conflict(
				"gate_alarm_blocking",
				"open blocking alarm "+alarm.ID+" must be closed before the command can occupy the gate",
			)
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

func (m *MemoryRepository) CreateAlarm(_ context.Context, alarm domain.GateAlarm) (domain.GateAlarm, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.gates[alarm.GateID]; !exists {
		return domain.GateAlarm{}, domain.NotFound("gate_not_found", "gate id was not found")
	}
	if _, exists := m.alarms[alarm.ID]; exists {
		return domain.GateAlarm{}, domain.Conflict("duplicate_alarm_id", "alarm id already exists")
	}

	m.alarms[alarm.ID] = alarm
	return cloneAlarm(alarm), nil
}

func (m *MemoryRepository) Alarm(_ context.Context, id string) (domain.GateAlarm, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alarm, exists := m.alarms[id]
	if !exists {
		return domain.GateAlarm{}, domain.NotFound("alarm_not_found", "alarm id was not found")
	}
	return cloneAlarm(alarm), nil
}

func (m *MemoryRepository) GateAlarms(_ context.Context, gateID string) ([]domain.GateAlarm, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.gates[gateID]; !exists {
		return nil, domain.NotFound("gate_not_found", "gate id was not found")
	}

	alarms := make([]domain.GateAlarm, 0)
	for _, alarm := range m.alarms {
		if alarm.GateID == gateID {
			alarms = append(alarms, cloneAlarm(alarm))
		}
	}
	sort.Slice(alarms, func(i, j int) bool {
		return alarms[i].CreatedAt.Before(alarms[j].CreatedAt)
	})
	return alarms, nil
}

func (m *MemoryRepository) AcknowledgeAlarm(
	_ context.Context,
	alarmID string,
	acknowledgedBy string,
	now time.Time,
) (domain.GateAlarm, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alarm, exists := m.alarms[alarmID]
	if !exists {
		return domain.GateAlarm{}, domain.NotFound("alarm_not_found", "alarm id was not found")
	}

	if err := alarm.Acknowledge(acknowledgedBy, now); err != nil {
		return domain.GateAlarm{}, err
	}

	m.alarms[alarmID] = alarm
	return cloneAlarm(alarm), nil
}

func (m *MemoryRepository) CloseAlarm(
	_ context.Context,
	alarmID string,
	handledBy string,
	resolution string,
	now time.Time,
) (domain.GateAlarm, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alarm, exists := m.alarms[alarmID]
	if !exists {
		return domain.GateAlarm{}, domain.NotFound("alarm_not_found", "alarm id was not found")
	}

	if err := alarm.Close(handledBy, resolution, now); err != nil {
		return domain.GateAlarm{}, err
	}

	m.alarms[alarmID] = alarm
	return cloneAlarm(alarm), nil
}

// openBlockingAlarm 必须在持有 m.mu 的情况下调用，返回闸门上最早登记且
// 尚未关闭的阻断级告警。
func (m *MemoryRepository) openBlockingAlarm(gateID string) (domain.GateAlarm, bool) {
	var blocker domain.GateAlarm
	found := false
	for _, alarm := range m.alarms {
		if alarm.GateID != gateID || !alarm.BlocksCommands() {
			continue
		}
		if !found || alarm.CreatedAt.Before(blocker.CreatedAt) {
			blocker = alarm
			found = true
		}
	}
	return blocker, found
}

func cloneCommand(command domain.GateCommand) domain.GateCommand {
	if command.Interlock == nil {
		return command
	}
	decision := *command.Interlock
	command.Interlock = &decision
	return command
}

func cloneAlarm(alarm domain.GateAlarm) domain.GateAlarm {
	if alarm.Acknowledgement != nil {
		acknowledgement := *alarm.Acknowledgement
		alarm.Acknowledgement = &acknowledgement
	}
	if alarm.Closure != nil {
		closure := *alarm.Closure
		alarm.Closure = &closure
	}
	return alarm
}
