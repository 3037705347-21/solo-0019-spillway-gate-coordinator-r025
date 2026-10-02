package contracts

type RegisterGateRequest struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	MaxAperture     float64 `json:"max_aperture"`
	InitialAperture float64 `json:"initial_aperture"`
}

type RequestCommandRequest struct {
	GateID         string  `json:"gate_id"`
	TargetAperture float64 `json:"target_aperture"`
	Reason         string  `json:"reason"`
	RequestedBy    string  `json:"requested_by"`
}

type ReviewInterlockRequest struct {
	Verdict  string `json:"verdict"`
	Verifier string `json:"verifier"`
	Note     string `json:"note"`
}

type ExecuteCommandRequest struct {
	Operator      string `json:"operator"`
	Token         string `json:"token"`
	SimulateFault bool   `json:"simulate_fault"`
}

type RegisterAlarmRequest struct {
	Source       string `json:"source"`
	Severity     string `json:"severity"`
	Detail       string `json:"detail"`
	RegisteredBy string `json:"registered_by"`
}

type AcknowledgeAlarmRequest struct {
	AcknowledgedBy string `json:"acknowledged_by"`
}

type CloseAlarmRequest struct {
	Resolution string `json:"resolution"`
	ClosedBy   string `json:"closed_by"`
}
