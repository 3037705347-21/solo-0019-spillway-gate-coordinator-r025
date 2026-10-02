package httpapi

import (
	"net/http"

	"example.com/spillway-gate-coordinator/internal/contracts"
)

func (h *handler) registerAlarm(w http.ResponseWriter, r *http.Request) {
	var request contracts.RegisterAlarmRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeDomainError(w, err)
		return
	}

	alarm, err := h.service.RegisterAlarm(r.Context(), r.PathValue("gateID"), request)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, contracts.AlarmResponse{Alarm: alarmView(alarm)})
}

func (h *handler) alarms(w http.ResponseWriter, r *http.Request) {
	alarms, err := h.service.Alarms(r.Context(), r.PathValue("gateID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contracts.AlarmListResponse{Alarms: alarmViews(alarms)})
}

func (h *handler) alarm(w http.ResponseWriter, r *http.Request) {
	alarm, err := h.service.Alarm(r.Context(), r.PathValue("gateID"), r.PathValue("alarmID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contracts.AlarmResponse{Alarm: alarmView(alarm)})
}

func (h *handler) acknowledgeAlarm(w http.ResponseWriter, r *http.Request) {
	var request contracts.AcknowledgeAlarmRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeDomainError(w, err)
		return
	}

	alarm, err := h.service.AcknowledgeAlarm(
		r.Context(),
		r.PathValue("gateID"),
		r.PathValue("alarmID"),
		request,
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contracts.AlarmResponse{Alarm: alarmView(alarm)})
}

func (h *handler) closeAlarm(w http.ResponseWriter, r *http.Request) {
	var request contracts.CloseAlarmRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeDomainError(w, err)
		return
	}

	alarm, err := h.service.CloseAlarm(
		r.Context(),
		r.PathValue("gateID"),
		r.PathValue("alarmID"),
		request,
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contracts.AlarmResponse{Alarm: alarmView(alarm)})
}
