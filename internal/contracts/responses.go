package contracts

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InterlockDecisionView struct {
	Verdict    string `json:"verdict"`
	Verifier   string `json:"verifier"`
	Note       string `json:"note"`
	ReviewedAt string `json:"reviewed_at"`
}

type GateView struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Aperture        float64 `json:"aperture"`
	MaxAperture     float64 `json:"max_aperture"`
	ActiveCommandID string  `json:"active_command_id,omitempty"`
	UpdatedAt       string  `json:"updated_at"`
	Version         int64   `json:"version"`
}

type CommandView struct {
	ID               string                 `json:"id"`
	GateID           string                 `json:"gate_id"`
	TargetAperture   float64                `json:"target_aperture"`
	StartingAperture float64                `json:"starting_aperture"`
	Reason           string                 `json:"reason"`
	RequestedBy      string                 `json:"requested_by"`
	Status           string                 `json:"status"`
	Interlock        *InterlockDecisionView `json:"interlock,omitempty"`
	ExecutionToken   string                 `json:"execution_token,omitempty"`
	ExecutedBy       string                 `json:"executed_by,omitempty"`
	TerminalReason   string                 `json:"terminal_reason,omitempty"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	Version          int64                  `json:"version"`
}

type RegisterGateResponse struct {
	Gate GateView `json:"gate"`
}

type GateResponse struct {
	Gate GateView `json:"gate"`
}

type CommandResponse struct {
	Command CommandView `json:"command"`
}

type ReviewCommandResponse struct {
	Command CommandView `json:"command"`
	Gate    GateView    `json:"gate"`
}

type ExecuteCommandResponse struct {
	Command CommandView `json:"command"`
	Gate    GateView    `json:"gate"`
}

type AlarmAcknowledgementView struct {
	AcknowledgedBy string `json:"acknowledged_by"`
	AcknowledgedAt string `json:"acknowledged_at"`
}

type AlarmClosureView struct {
	HandledBy  string `json:"handled_by"`
	Resolution string `json:"resolution"`
	ClosedAt   string `json:"closed_at"`
}

type AlarmView struct {
	ID              string                    `json:"id"`
	GateID          string                    `json:"gate_id"`
	Source          string                    `json:"source"`
	Level           string                    `json:"level"`
	Basis           string                    `json:"basis"`
	Note            string                    `json:"note,omitempty"`
	ReportedBy      string                    `json:"reported_by"`
	Status          string                    `json:"status"`
	Acknowledgement *AlarmAcknowledgementView `json:"acknowledgement,omitempty"`
	Closure         *AlarmClosureView         `json:"closure,omitempty"`
	CreatedAt       string                    `json:"created_at"`
	UpdatedAt       string                    `json:"updated_at"`
	Version         int64                     `json:"version"`
}

type RegisterAlarmResponse struct {
	Alarm AlarmView `json:"alarm"`
}

type AlarmResponse struct {
	Alarm AlarmView `json:"alarm"`
}

type GateAlarmsResponse struct {
	Alarms []AlarmView `json:"alarms"`
}
