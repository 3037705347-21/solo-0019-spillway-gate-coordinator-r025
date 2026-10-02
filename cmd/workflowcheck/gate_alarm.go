package main

import (
	"fmt"
	"net/http"
	"sync"
)

// runGateAlarm exercises the independent alarm workflow:
// registration, acknowledgement (seen but not handled), closure based on
// current disposition facts, blocking of new command occupation, immutable
// history, and concurrent alarm handling vs. interlock review.
func runGateAlarm(client *apiClient) error {
	if _, err := fixtureGate(client, "gate-alarm", "告警泄洪闸", 5, 1.0); err != nil {
		return err
	}

	// Register a blocking alarm from device monitoring.
	alarm, err := fixtureAlarm(
		client,
		"gate-alarm",
		"vibration-sensor-v2",
		"blocking",
		"启闭机振动超阈值",
		"monitor-zhou",
	)
	if err != nil {
		return err
	}
	alarmID, err := stringField(alarm, "id")
	if err != nil {
		return err
	}
	if status, err := stringField(alarm, "status"); err != nil || status != "open" {
		return fmt.Errorf("new alarm status: expected open, got %q", status)
	}
	if severity, err := stringField(alarm, "severity"); err != nil || severity != "blocking" {
		return fmt.Errorf("new alarm severity: expected blocking, got %q", severity)
	}
	if source, err := stringField(alarm, "source"); err != nil || source != "vibration-sensor-v2" {
		return fmt.Errorf("new alarm source mismatch: %q", source)
	}

	// Invalid registrations are rejected with stable errors.
	badSeverity, err := client.post("/gates/gate-alarm/alarms", map[string]any{
		"source":        "visual-patrol",
		"severity":      "fatal",
		"registered_by": "monitor-zhou",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(badSeverity, http.StatusBadRequest, "reject unknown severity"); err != nil {
		return err
	}
	missingSource, err := client.post("/gates/gate-alarm/alarms", map[string]any{
		"severity":      "advisory",
		"registered_by": "monitor-zhou",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(missingSource, http.StatusBadRequest, "reject missing source"); err != nil {
		return err
	}
	unknownGate, err := client.post("/gates/gate-missing/alarms", map[string]any{
		"source":        "visual-patrol",
		"severity":      "advisory",
		"registered_by": "monitor-zhou",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(unknownGate, http.StatusNotFound, "reject alarm on unknown gate"); err != nil {
		return err
	}

	// A pending command can still be created, but a clear review must fail to
	// occupy the gate while the blocking alarm is open.
	command, err := fixtureCommand(client, "gate-alarm", 2.5, "水位调节", "dispatcher-lin")
	if err != nil {
		return err
	}
	commandID, err := stringField(command, "id")
	if err != nil {
		return err
	}
	blocked, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "尝试占用",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(blocked, http.StatusConflict, "blocked alarm prevents occupation"); err != nil {
		return err
	}
	if code, _ := blocked.Body["code"].(string); code != "gate_alarm_blocking" {
		return fmt.Errorf("blocking review error code: expected gate_alarm_blocking, got %q", code)
	}

	// The failed review must not mutate anything: command stays pending, gate
	// stays idle with no active command.
	pendingResult, err := client.get("/commands/" + commandID)
	if err != nil {
		return err
	}
	pendingCommand, err := objectField(pendingResult.Body, "command")
	if err != nil {
		return err
	}
	if status, err := stringField(pendingCommand, "status"); err != nil || status != "pending" {
		return fmt.Errorf("command after blocked review: expected pending, got %q", status)
	}
	gateResult, err := client.get("/gates/gate-alarm")
	if err != nil {
		return err
	}
	gateBody, err := objectField(gateResult.Body, "gate")
	if err != nil {
		return err
	}
	if status, err := stringField(gateBody, "status"); err != nil || status != "idle" {
		return fmt.Errorf("gate after blocked review: expected idle, got %q", status)
	}
	if _, present := gateBody["active_command_id"]; present {
		return fmt.Errorf("gate after blocked review still references an active command")
	}

	// Acknowledgement records that the site saw the alarm, but must NOT be
	// allowed to replace closure: occupation must still be refused.
	ack, err := client.post("/gates/gate-alarm/alarms/"+alarmID+"/acknowledge", map[string]any{
		"acknowledged_by": "operator-chen",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(ack, http.StatusOK, "acknowledge alarm"); err != nil {
		return err
	}
	ackBody, err := objectField(ack.Body, "alarm")
	if err != nil {
		return err
	}
	if status, err := stringField(ackBody, "status"); err != nil || status != "acknowledged" {
		return fmt.Errorf("acknowledged alarm status: expected acknowledged, got %q", status)
	}
	if by, err := stringField(ackBody, "acknowledged_by"); err != nil || by != "operator-chen" {
		return fmt.Errorf("acknowledged_by mismatch: %q", by)
	}
	stillBlocked, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "确认后再次尝试",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(stillBlocked, http.StatusConflict, "acknowledged alarm still blocks occupation"); err != nil {
		return err
	}

	// Double acknowledgement and closing without a disposition basis fail.
	doubleAck, err := client.post("/gates/gate-alarm/alarms/"+alarmID+"/acknowledge", map[string]any{
		"acknowledged_by": "operator-chen",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(doubleAck, http.StatusConflict, "reject duplicate acknowledgement"); err != nil {
		return err
	}
	closeWithoutBasis, err := client.post("/gates/gate-alarm/alarms/"+alarmID+"/close", map[string]any{
		"closed_by": "engineer-he",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(closeWithoutBasis, http.StatusBadRequest, "reject closure without disposition basis"); err != nil {
		return err
	}

	// Closing with the current disposition facts lifts the block.
	closed, err := client.post("/gates/gate-alarm/alarms/"+alarmID+"/close", map[string]any{
		"resolution": "现场更换固定螺栓并复测振动恢复正常",
		"closed_by":  "engineer-he",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(closed, http.StatusOK, "close alarm"); err != nil {
		return err
	}
	closedBody, err := objectField(closed.Body, "alarm")
	if err != nil {
		return err
	}
	if status, err := stringField(closedBody, "status"); err != nil || status != "closed" {
		return fmt.Errorf("closed alarm status: expected closed, got %q", status)
	}
	if by, err := stringField(closedBody, "closed_by"); err != nil || by != "engineer-he" {
		return fmt.Errorf("closed_by mismatch: %q", by)
	}
	if resolution, err := stringField(closedBody, "resolution"); err != nil ||
		resolution != "现场更换固定螺栓并复测振动恢复正常" {
		return fmt.Errorf("resolution mismatch: %q", resolution)
	}

	// History is immutable: the closed alarm cannot be acknowledged, closed
	// again, or have its disposition basis rewritten.
	reAck, err := client.post("/gates/gate-alarm/alarms/"+alarmID+"/acknowledge", map[string]any{
		"acknowledged_by": "operator-chen",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(reAck, http.StatusConflict, "closed alarm cannot be acknowledged"); err != nil {
		return err
	}
	reClose, err := client.post("/gates/gate-alarm/alarms/"+alarmID+"/close", map[string]any{
		"resolution": "试图改写处置依据",
		"closed_by":  "someone-else",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(reClose, http.StatusConflict, "closed alarm cannot be closed again"); err != nil {
		return err
	}
	persisted, err := client.get("/gates/gate-alarm/alarms/" + alarmID)
	if err != nil {
		return err
	}
	if err := requireStatus(persisted, http.StatusOK, "read persisted alarm"); err != nil {
		return err
	}
	persistedBody, err := objectField(persisted.Body, "alarm")
	if err != nil {
		return err
	}
	if resolution, _ := persistedBody["resolution"].(string); resolution != "现场更换固定螺栓并复测振动恢复正常" {
		return fmt.Errorf("historical alarm resolution was rewritten: %q", resolution)
	}
	if closedBy, _ := persistedBody["closed_by"].(string); closedBy != "engineer-he" {
		return fmt.Errorf("historical closed_by was rewritten: %q", closedBy)
	}

	// Once closed, the clear review can occupy the gate exactly once.
	approved, err := fixtureClearReview(client, commandID, "safety-wu")
	if err != nil {
		return err
	}
	if status, err := stringField(approved, "status"); err != nil || status != "approved" {
		return fmt.Errorf("review after closure: expected approved, got %q", status)
	}
	token, err := stringField(approved, "execution_token")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("review after closure did not issue a token")
	}

	// An advisory alarm never blocks occupation. Verify on a second gate.
	if _, err := fixtureGate(client, "gate-advisory", "提示级告警闸", 4, 0.5); err != nil {
		return err
	}
	if _, err := fixtureAlarm(client, "gate-advisory", "acoustic-sensor", "advisory", "轻微异响", "monitor-zhou"); err != nil {
		return err
	}
	advisoryCommand, err := fixtureCommand(client, "gate-advisory", 1.5, "常规调节", "dispatcher-yang")
	if err != nil {
		return err
	}
	advisoryCommandID, err := stringField(advisoryCommand, "id")
	if err != nil {
		return err
	}
	advisoryApproved, err := fixtureClearReview(client, advisoryCommandID, "safety-he")
	if err != nil {
		return err
	}
	if status, err := stringField(advisoryApproved, "status"); err != nil || status != "approved" {
		return fmt.Errorf("advisory alarm must not block occupation: %q", status)
	}

	// The list endpoint returns the gate's alarm history including the closed
	// record, in registration order.
	list, err := client.get("/gates/gate-alarm/alarms")
	if err != nil {
		return err
	}
	if err := requireStatus(list, http.StatusOK, "list alarms"); err != nil {
		return err
	}
	items, ok := list.Body["alarms"].([]any)
	if !ok {
		return fmt.Errorf("alarm list response missing alarms array")
	}
	if len(items) != 1 {
		return fmt.Errorf("alarm list length: expected 1, got %d", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		return fmt.Errorf("alarm list item is not an object")
	}
	if id, _ := first["id"].(string); id != alarmID {
		return fmt.Errorf("alarm list order/content mismatch: %q", id)
	}
	if status, _ := first["status"].(string); status != "closed" {
		return fmt.Errorf("closed alarm missing from historical list: %q", status)
	}

	fmt.Println("gate-alarm: alarm lifecycle, occupation blocking, and immutable history verified")
	return nil
}

// runAlarmReviewConcurrency hammers alarm handling against interlock review
// on fresh gates. Whatever order the repository lock picks, each gate must end
// in exactly one state consistent with the site: either the clear review won
// (command approved + gate reserved, alarm registered afterwards stays open)
// or the blocking alarm won (command pending + gate idle, review refused).
func runAlarmReviewConcurrency(client *apiClient) error {
	const iterations = 24

	var wg sync.WaitGroup
	errCh := make(chan error, iterations)
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			gateID := fmt.Sprintf("gate-race-%02d", i)
			if _, err := fixtureGate(client, gateID, fmt.Sprintf("并发闸%02d", i), 5, 1.0); err != nil {
				errCh <- err
				return
			}
			command, err := fixtureCommand(client, gateID, 2.0, "并发占用", "dispatcher-lin")
			if err != nil {
				errCh <- err
				return
			}
			commandID, _ := stringField(command, "id")

			start := make(chan struct{})
			pair := sync.WaitGroup{}
			var reviewStatus int
			var alarmID string
			pair.Add(2)
			go func() {
				defer pair.Done()
				<-start
				result, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
					"verdict":  "clear",
					"verifier": "safety-wu",
					"note":     "并发复核",
				})
				if err != nil {
					errCh <- err
					return
				}
				reviewStatus = result.Status
			}()
			go func() {
				defer pair.Done()
				<-start
				alarm, err := fixtureAlarm(client, gateID, "vibration-sensor", "blocking", "并发登记", "monitor-zhou")
				if err != nil {
					errCh <- err
					return
				}
				alarmID, _ = stringField(alarm, "id")
			}()
			close(start)
			pair.Wait()

			gateRes, err := client.get("/gates/" + gateID)
			if err != nil {
				errCh <- err
				return
			}
			gateBody, err := objectField(gateRes.Body, "gate")
			if err != nil {
				errCh <- err
				return
			}
			cmdRes, err := client.get("/commands/" + commandID)
			if err != nil {
				errCh <- err
				return
			}
			cmdBody, err := objectField(cmdRes.Body, "command")
			if err != nil {
				errCh <- err
				return
			}
			gateStatus, _ := stringField(gateBody, "status")
			cmdStatus, _ := stringField(cmdBody, "status")
			activeID, _ := stringField(gateBody, "active_command_id")

			switch reviewStatus {
			case http.StatusOK:
				// Review won the race: command approved and gate reserved by it.
				if cmdStatus != "approved" || gateStatus != "reserved" || activeID != commandID {
					errCh <- fmt.Errorf(
						"iteration %d review-won mismatch: review=200 command=%q gate=%q active=%q",
						i, cmdStatus, gateStatus, activeID,
					)
					return
				}
			case http.StatusConflict:
				// Alarm won the race: nothing was occupied or mutated.
				if cmdStatus != "pending" || gateStatus != "idle" || activeID != "" {
					errCh <- fmt.Errorf(
						"iteration %d alarm-won mismatch: review=409 command=%q gate=%q active=%q",
						i, cmdStatus, gateStatus, activeID,
					)
					return
				}
			default:
				errCh <- fmt.Errorf("iteration %d unexpected review status %d", i, reviewStatus)
				return
			}

			// The alarm exists in exactly one historical form and matches the
			// winner: open if the review occupied first, still blocking.
			alarmRes, err := client.get("/gates/" + gateID + "/alarms/" + alarmID)
			if err != nil {
				errCh <- err
				return
			}
			if err := requireStatus(alarmRes, http.StatusOK, "read race alarm"); err != nil {
				errCh <- err
				return
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	fmt.Println("alarm-review-concurrency: every race resolved to exactly one site-consistent outcome")
	return nil
}
