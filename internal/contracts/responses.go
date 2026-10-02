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

type AlarmView struct {
	ID             string `json:"id"`
	GateID         string `json:"gate_id"`
	Source         string `json:"source"`
	Severity       string `json:"severity"`
	Detail         string `json:"detail,omitempty"`
	RegisteredBy   string `json:"registered_by"`
	Status         string `json:"status"`
	AcknowledgedBy string `json:"acknowledged_by,omitempty"`
	AcknowledgedAt string `json:"acknowledged_at,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	ClosedBy       string `json:"closed_by,omitempty"`
	ClosedAt       string `json:"closed_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	Version        int64  `json:"version"`
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

type AlarmResponse struct {
	Alarm AlarmView `json:"alarm"`
}

type AlarmListResponse struct {
	Alarms []AlarmView `json:"alarms"`
}
