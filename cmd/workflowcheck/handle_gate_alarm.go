package main

import (
	"fmt"
	"net/http"
	"sync"
)

func runHandleGateAlarm(client *apiClient) error {
	// 1. 登记阻断级告警：独立记录，闸门保持自身状态。
	if _, err := fixtureGate(client, "gate-alarm-a", "告警验证闸甲", 4, 0.5); err != nil {
		return err
	}
	alarm, err := fixtureAlarm(client, "gate-alarm-a", "blocking", "vibration-sensor")
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
	if level, err := stringField(alarm, "level"); err != nil || level != "blocking" {
		return fmt.Errorf("alarm level: expected blocking, got %q", level)
	}

	// 2. 未确认时不能直接关闭：确认不能省略，关闭必须基于处置事实。
	closeEarly, err := client.post("/alarms/"+alarmID+"/close", map[string]any{
		"handled_by": "repair-han",
		"resolution": "现场更换传感器",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(closeEarly, http.StatusConflict, "reject closing an unacknowledged alarm"); err != nil {
		return err
	}

	// 3. 未关闭的阻断级告警必须阻止新命令占用。待复核命令允许登记，但 clear
	//    复核必须被拦截，且不能占用闸门。
	command, err := fixtureCommand(client, "gate-alarm-a", 1.5, "告警期间调整", "dispatcher-lin")
	if err != nil {
		return err
	}
	commandID, err := stringField(command, "id")
	if err != nil {
		return err
	}
	blockedReview, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "告警阻断验证",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(blockedReview, http.StatusConflict, "block clear review while alarm open"); err != nil {
		return err
	}
	if code, err := stringField(blockedReview.Body, "code"); err != nil || code != "gate_alarm_blocking" {
		return fmt.Errorf("blocked review error code: expected gate_alarm_blocking, got %q", code)
	}
	if err := assertGateState(client, "gate-alarm-a", "idle", ""); err != nil {
		return err
	}
	if cmd, err := client.get("/commands/" + commandID); err != nil {
		return err
	} else {
		body, err := objectField(cmd.Body, "command")
		if err != nil {
			return err
		}
		if status, err := stringField(body, "status"); err != nil || status != "pending" {
			return fmt.Errorf("blocked review must not change command: expected pending, got %q", status)
		}
	}

	// 4. 确认告警只表示现场已看到：状态 acknowledged，阻断仍然有效。
	ack, err := client.post("/alarms/"+alarmID+"/acknowledge", map[string]any{
		"acknowledged_by": "duty-gao",
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
	ackRecord, err := objectField(ackBody, "acknowledgement")
	if err != nil {
		return err
	}
	if by, err := stringField(ackRecord, "acknowledged_by"); err != nil || by != "duty-gao" {
		return fmt.Errorf("acknowledgement actor mismatch: %q", by)
	}
	blockedAfterAck, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "确认不等于关闭",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(blockedAfterAck, http.StatusConflict, "acknowledgement must not lift the block"); err != nil {
		return err
	}

	// 5. 关闭必须携带当前处置事实；缺少处置结果时拒绝。
	closeWithoutFacts, err := client.post("/alarms/"+alarmID+"/close", map[string]any{
		"handled_by": "repair-han",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(closeWithoutFacts, http.StatusBadRequest, "reject closure without handling facts"); err != nil {
		return err
	}

	// 6. 基于当前处置事实关闭，随后 clear 复核与闸门占用才能成功。
	closed, err := client.post("/alarms/"+alarmID+"/close", map[string]any{
		"handled_by": "repair-han",
		"resolution": "现场更换振动传感器并复测正常",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(closed, http.StatusOK, "close alarm with handling facts"); err != nil {
		return err
	}
	closedBody, err := objectField(closed.Body, "alarm")
	if err != nil {
		return err
	}
	if status, err := stringField(closedBody, "status"); err != nil || status != "closed" {
		return fmt.Errorf("closed alarm status: expected closed, got %q", status)
	}

	// 7. 关闭不改写历史告警：登记字段、确认记录都保留，处置事实只追加。
	fetched, err := client.get("/alarms/" + alarmID)
	if err != nil {
		return err
	}
	if err := requireStatus(fetched, http.StatusOK, "read closed alarm"); err != nil {
		return err
	}
	history, err := objectField(fetched.Body, "alarm")
	if err != nil {
		return err
	}
	if source, err := stringField(history, "source"); err != nil || source != "vibration-sensor" {
		return fmt.Errorf("alarm history source must be preserved, got %q", source)
	}
	if basis, err := stringField(history, "basis"); err != nil || basis != "巡检发现闸门异常信号" {
		return fmt.Errorf("alarm history basis must be preserved, got %q", basis)
	}
	if reporter, err := stringField(history, "reported_by"); err != nil || reporter != "monitor-zhou" {
		return fmt.Errorf("alarm history reporter must be preserved, got %q", reporter)
	}
	if _, err := objectField(history, "acknowledgement"); err != nil {
		return fmt.Errorf("acknowledgement record must be preserved after closure: %w", err)
	}
	closureRecord, err := objectField(history, "closure")
	if err != nil {
		return err
	}
	if resolution, err := stringField(closureRecord, "resolution"); err != nil ||
		resolution != "现场更换振动传感器并复测正常" {
		return fmt.Errorf("closure resolution mismatch: %q", resolution)
	}

	// 8. 终态单向：重复确认、重复关闭都必须冲突。
	repeatAck, err := client.post("/alarms/"+alarmID+"/acknowledge", map[string]any{
		"acknowledged_by": "duty-gao",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(repeatAck, http.StatusConflict, "reject repeated acknowledgement"); err != nil {
		return err
	}
	repeatClose, err := client.post("/alarms/"+alarmID+"/close", map[string]any{
		"handled_by": "repair-han",
		"resolution": "重复关闭尝试",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(repeatClose, http.StatusConflict, "reject repeated closure"); err != nil {
		return err
	}

	// 9. 关闭后命令可以完成完整流程。
	approved, err := fixtureClearReview(client, commandID, "safety-wu")
	if err != nil {
		return err
	}
	token, err := stringField(approved, "execution_token")
	if err != nil {
		return err
	}
	executed, err := client.post("/commands/"+commandID+"/execute", map[string]any{
		"operator":       "operator-chen",
		"token":          token,
		"simulate_fault": false,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(executed, http.StatusOK, "execute after alarm closure"); err != nil {
		return err
	}
	if err := assertGateState(client, "gate-alarm-a", "idle", ""); err != nil {
		return err
	}

	// 10. 提示级告警不阻断占用；历史列表完整保留两条告警。
	advisory, err := fixtureAlarm(client, "gate-alarm-a", "advisory", "noise-monitor")
	if err != nil {
		return err
	}
	advisoryID, err := stringField(advisory, "id")
	if err != nil {
		return err
	}
	advisoryCommand, err := fixtureCommand(client, "gate-alarm-a", 3.0, "提示级不阻断", "dispatcher-lin")
	if err != nil {
		return err
	}
	advisoryCommandID, err := stringField(advisoryCommand, "id")
	if err != nil {
		return err
	}
	if _, err := fixtureClearReview(client, advisoryCommandID, "safety-wu"); err != nil {
		return fmt.Errorf("advisory alarm must not block command occupation: %w", err)
	}
	if err := assertGateState(client, "gate-alarm-a", "reserved", advisoryCommandID); err != nil {
		return err
	}
	listResult, err := client.get("/gates/gate-alarm-a/alarms")
	if err != nil {
		return err
	}
	if err := requireStatus(listResult, http.StatusOK, "list gate alarms"); err != nil {
		return err
	}
	list, err := alarmList(listResult.Body)
	if err != nil {
		return err
	}
	if len(list) != 2 {
		return fmt.Errorf("gate alarm history: expected 2 records, got %d", len(list))
	}
	if id, err := stringField(list[0], "id"); err != nil || id != alarmID {
		return fmt.Errorf("alarm list order/contents mismatch, first id=%q", id)
	}
	if id, err := stringField(list[1], "id"); err != nil || id != advisoryID {
		return fmt.Errorf("alarm list order/contents mismatch, second id=%q", id)
	}

	// 11. 多条阻断告警时只关闭一条仍然阻断；全部关闭才放行。
	if _, err := fixtureGate(client, "gate-alarm-b", "告警验证闸乙", 4, 0.5); err != nil {
		return err
	}
	first, err := fixtureAlarm(client, "gate-alarm-b", "blocking", "position-sensor")
	if err != nil {
		return err
	}
	second, err := fixtureAlarm(client, "gate-alarm-b", "blocking", "power-feed")
	if err != nil {
		return err
	}
	if err := acknowledgeAndClose(client, first); err != nil {
		return err
	}
	pending, err := fixtureCommand(client, "gate-alarm-b", 1.0, "仍有阻断告警", "dispatcher-lin")
	if err != nil {
		return err
	}
	pendingID, err := stringField(pending, "id")
	if err != nil {
		return err
	}
	stillBlocked, err := client.post("/commands/"+pendingID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "第二条阻断告警未关闭",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(stillBlocked, http.StatusConflict, "still blocked while another blocking alarm open"); err != nil {
		return err
	}
	if err := acknowledgeAndClose(client, second); err != nil {
		return err
	}
	if _, err := fixtureClearReview(client, pendingID, "safety-wu"); err != nil {
		return fmt.Errorf("command must occupy once all blocking alarms are closed: %w", err)
	}
	if err := assertGateState(client, "gate-alarm-b", "reserved", pendingID); err != nil {
		return err
	}

	// 12. 告警关闭与 clear 复核并发：同一把仓库锁下只有一个与现场一致的结果。
	if err := verifyAlarmReviewRace(client, "gate-alarm-c"); err != nil {
		return err
	}

	// 13. open 告警的关闭与复核并发：关闭必败，复核也必败，现场维持阻断。
	if err := verifyOpenAlarmRace(client, "gate-alarm-d"); err != nil {
		return err
	}

	fmt.Println("handle-gate-alarm: alarm lifecycle, blocking and concurrency verified")
	return nil
}

func acknowledgeAndClose(client *apiClient, alarm map[string]any) error {
	id, err := stringField(alarm, "id")
	if err != nil {
		return err
	}
	ack, err := client.post("/alarms/"+id+"/acknowledge", map[string]any{
		"acknowledged_by": "duty-gao",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(ack, http.StatusOK, "acknowledge alarm before close"); err != nil {
		return err
	}
	closed, err := client.post("/alarms/"+id+"/close", map[string]any{
		"handled_by": "repair-han",
		"resolution": "现场处置完成并复测正常",
	})
	if err != nil {
		return err
	}
	return requireStatus(closed, http.StatusOK, "close acknowledged alarm")
}

func assertGateState(client *apiClient, gateID, expectedStatus, expectedCommandID string) error {
	result, err := client.get("/gates/" + gateID)
	if err != nil {
		return err
	}
	if err := requireStatus(result, http.StatusOK, "read gate state"); err != nil {
		return err
	}
	gate, err := objectField(result.Body, "gate")
	if err != nil {
		return err
	}
	status, err := stringField(gate, "status")
	if err != nil {
		return err
	}
	if status != expectedStatus {
		return fmt.Errorf("gate %s status: expected %q, got %q", gateID, expectedStatus, status)
	}
	activeID, _ := gate["active_command_id"].(string)
	if activeID != expectedCommandID {
		return fmt.Errorf(
			"gate %s active command: expected %q, got %q",
			gateID,
			expectedCommandID,
			activeID,
		)
	}
	return nil
}

func alarmList(body map[string]any) ([]map[string]any, error) {
	value, exists := body["alarms"]
	if !exists {
		return nil, fmt.Errorf("response is missing list field %q", "alarms")
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("response field %q is not a list", "alarms")
	}
	alarms := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("alarm list entry is not an object")
		}
		alarms = append(alarms, entry)
	}
	return alarms, nil
}

// verifyAlarmReviewRace 让“关闭已确认告警”和“clear 复核占用闸门”并发执行，
// 反复验证最终状态与唯一赢家保持一致：批准的命令必须与闸门占用一一对应。
func verifyAlarmReviewRace(client *apiClient, gateIDPrefix string) error {
	for round := 0; round < 10; round++ {
		gateID := fmt.Sprintf("%s-%d", gateIDPrefix, round)
		if _, err := fixtureGate(client, gateID, "告警并发验证闸", 4, 0.5); err != nil {
			return err
		}
		alarm, err := fixtureAlarm(client, gateID, "blocking", "race-sensor")
		if err != nil {
			return err
		}
		alarmID, err := stringField(alarm, "id")
		if err != nil {
			return err
		}
		command, err := fixtureCommand(client, gateID, 2.0, "并发占用窗口", "dispatcher-lin")
		if err != nil {
			return err
		}
		commandID, err := stringField(command, "id")
		if err != nil {
			return err
		}
		ack, err := client.post("/alarms/"+alarmID+"/acknowledge", map[string]any{
			"acknowledged_by": "duty-gao",
		})
		if err != nil {
			return err
		}
		if err := requireStatus(ack, http.StatusOK, "acknowledge before race"); err != nil {
			return err
		}

		var wg sync.WaitGroup
		var closeStatus, reviewStatus int
		wg.Add(2)
		go func() {
			defer wg.Done()
			result, err := client.post("/alarms/"+alarmID+"/close", map[string]any{
				"handled_by": "repair-han",
				"resolution": "并发窗口内现场处置完成",
			})
			if err == nil {
				closeStatus = result.Status
			}
		}()
		go func() {
			defer wg.Done()
			result, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
				"verdict":  "clear",
				"verifier": "safety-wu",
				"note":     "并发占用尝试",
			})
			if err == nil {
				reviewStatus = result.Status
			}
		}()
		wg.Wait()

		if closeStatus != http.StatusOK {
			return fmt.Errorf("race round %d: alarm closure expected 200, got %d", round, closeStatus)
		}
		if reviewStatus != http.StatusOK && reviewStatus != http.StatusConflict {
			return fmt.Errorf("race round %d: review expected 200 or 409, got %d", round, reviewStatus)
		}

		finalGate, err := client.get("/gates/" + gateID)
		if err != nil {
			return err
		}
		gateBody, err := objectField(finalGate.Body, "gate")
		if err != nil {
			return err
		}
		finalCommand, err := client.get("/commands/" + commandID)
		if err != nil {
			return err
		}
		commandBody, err := objectField(finalCommand.Body, "command")
		if err != nil {
			return err
		}
		commandStatus, err := stringField(commandBody, "status")
		if err != nil {
			return err
		}
		gateStatus, err := stringField(gateBody, "status")
		if err != nil {
			return err
		}
		activeID, _ := gateBody["active_command_id"].(string)

		// approved 必须恰好伴随闸门被该命令占用；pending 必须恰好伴随闸门空闲。
		switch reviewStatus {
		case http.StatusOK:
			if commandStatus != "approved" || gateStatus != "reserved" || activeID != commandID {
				return fmt.Errorf(
					"race round %d inconsistent win: review=200 command=%s gate=%s active=%s",
					round, commandStatus, gateStatus, activeID,
				)
			}
		case http.StatusConflict:
			if commandStatus != "pending" || gateStatus != "idle" || activeID != "" {
				return fmt.Errorf(
					"race round %d inconsistent loss: review=409 command=%s gate=%s active=%s",
					round, commandStatus, gateStatus, activeID,
				)
			}
		}
	}
	return nil
}

// verifyOpenAlarmRace 让“直接关闭 open 告警”（缺少确认，必败）与 clear 复核并发，
// 任何顺序下复核都必须被阻断，现场状态保持一致：命令 pending、闸门 idle。
func verifyOpenAlarmRace(client *apiClient, gateID string) error {
	if _, err := fixtureGate(client, gateID, "open告警并发闸", 4, 0.5); err != nil {
		return err
	}
	alarm, err := fixtureAlarm(client, gateID, "blocking", "open-race-sensor")
	if err != nil {
		return err
	}
	alarmID, err := stringField(alarm, "id")
	if err != nil {
		return err
	}
	command, err := fixtureCommand(client, gateID, 2.0, "open告警并发命令", "dispatcher-lin")
	if err != nil {
		return err
	}
	commandID, err := stringField(command, "id")
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var closeStatus, reviewStatus int
	wg.Add(2)
	go func() {
		defer wg.Done()
		result, err := client.post("/alarms/"+alarmID+"/close", map[string]any{
			"handled_by": "repair-han",
			"resolution": "未经确认的关闭尝试",
		})
		if err == nil {
			closeStatus = result.Status
		}
	}()
	go func() {
		defer wg.Done()
		result, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
			"verdict":  "clear",
			"verifier": "safety-wu",
			"note":     "open告警并发复核",
		})
		if err == nil {
			reviewStatus = result.Status
		}
	}()
	wg.Wait()

	if closeStatus != http.StatusConflict {
		return fmt.Errorf("closing an open alarm must conflict, got %d", closeStatus)
	}
	if reviewStatus != http.StatusConflict {
		return fmt.Errorf("review against an open alarm must conflict, got %d", reviewStatus)
	}
	return assertGateState(client, gateID, "idle", "")
}
