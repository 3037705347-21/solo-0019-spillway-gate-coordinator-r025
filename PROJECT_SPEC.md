# 泄洪闸门操作协调服务

## 项目目标

该服务用于水工控制室协调泄洪闸门的命令申请、安全联锁复核和单次开度调整。系统在同一闸门上只允许一条已通过复核且尚未结束的执行命令，任何冲突都必须被显式拒绝，不能覆盖已经占用的闸门。

## 使用角色

- 运行协调员：登记闸门并提交开度调整命令。
- 安全复核员：提交联锁结论，决定命令是否通过。
- 闸门操作员：凭执行令牌启动命令并登记执行结果。
- 监控或值班人员：登记闸门异常告警，确认现场已看到，并在处置完成后基于当前处置依据关闭告警。

## 核心实体

- `gate`：闸门及其当前开度、运行状态、最大开度和活动命令。
- `gate_command`：一次开度调整申请，包含目标开度、申请原因、状态和执行令牌。
- `interlock_decision`：安全复核结论，状态为 clear 或 blocked。
- `gate_alarm`：独立于命令的闸门异常告警，包含异常来源、级别（advisory/blocking）、登记人、确认记录和关闭时的处置依据。

## 业务流程

### register-gate

运行协调员通过 `POST /gates` 登记闸门。服务校验标识、名称、最大开度和初始开度，写入空闲状态；重复标识返回稳定冲突错误，不覆盖原记录。

### request-gate-command

运行协调员通过 `POST /commands` 提交开度调整命令。服务读取闸门当前开度和限制，校验目标开度与申请信息，创建待复核命令并冻结起始开度。

### review-safety-interlock

安全复核员通过 `POST /commands/{id}/interlock` 提交复核结论。clear 结论会原子占用闸门并签发一次执行令牌；blocked 结论会把命令置为 rejected。其他命令先占用同一闸门时，当前 clear 结论返回冲突且不改变闸门。

### execute-gate-command

闸门操作员通过 `POST /commands/{id}/execute` 提交执行令牌。服务校验命令、令牌、闸门占用关系和目标开度，在临界区内完成 approved、executing、completed 或 aborted 的转换，并同步更新闸门状态。令牌在命令结束后失效，不能复用。

### register-gate-alarm

监控或值班人员通过 `POST /gates/{id}/alarms` 登记异常来源、级别（advisory 或 blocking）、补充说明和登记人。告警独立于命令存在：即使没有任何命令执行失败，也可以登记告警。重复标识返回稳定冲突错误。

### acknowledge-gate-alarm

值班人员通过 `POST /gates/{id}/alarms/{alarmId}/acknowledge` 确认已看到告警。确认只表示现场已知悉，不改变阻断效果：已确认但未关闭的阻断级告警仍然阻止新的命令占用。

### close-gate-alarm

处置完成后，值班人员通过 `POST /gates/{id}/alarms/{alarmId}/close` 提交当前处置依据（resolution）和关闭人。关闭必须显式提供当前事实，服务端写入关闭时间。关闭是单向终态：不能重复关闭、不能重新确认、不能改写处置依据，历史告警永久保留在告警列表中。阻断级告警关闭后，新的 clear 复核才能占用闸门。

## 状态与规则

- `gate` 状态：idle、reserved、moving、faulted。
- `gate_command` 状态：pending、approved、rejected、executing、completed、aborted。
- `gate_alarm` 状态：open、acknowledged、closed；流转 open->acknowledged、open->closed、acknowledged->closed。
- 只有 pending 命令可以复核。
- 只有 approved 命令可以执行，且执行令牌必须匹配。
- clear 复核与闸门占用必须同时成功；任一步失败时两者都不写入。
- 未关闭（open 或 acknowledged）的 blocking 告警与闸门占用检查在同一临界区内完成：存在此类告警时 clear 复核返回 `gate_alarm_blocking` 冲突，命令保持 pending、闸门保持 idle。
- advisory 告警不阻止命令占用。
- 确认告警不能代替关闭；关闭必须携带当前处置依据，且关闭后历史告警不可改写。
- blocked 复核使命令 rejected，不会占用闸门。
- 模拟故障会令命令 aborted、闸门 faulted，并保留最终开度。
- 已完成或已中止的命令不能再次执行。

## 模块边界

- `cmd/server`：进程入口、监听地址和退出控制。
- `internal/httpapi`：HTTP 路由、JSON 解码、状态码映射。
- `internal/coordination`：工作流编排、命令标识和执行令牌签发。
- `internal/domain`：实体、不变量和状态转换规则。
- `internal/storage`：带互斥保护的内存仓库和原子写入。
- `internal/contracts`：API 请求、响应和稳定错误结构。
- `internal/bootstrap`：运行时依赖装配。

## 接口

- `GET /health`
- `POST /gates`
- `GET /gates/{id}`
- `POST /commands`
- `GET /commands/{id}`
- `POST /commands/{id}/interlock`
- `POST /commands/{id}/execute`
- `POST /gates/{id}/alarms`
- `GET /gates/{id}/alarms`
- `GET /gates/{id}/alarms/{alarmId}`
- `POST /gates/{id}/alarms/{alarmId}/acknowledge`
- `POST /gates/{id}/alarms/{alarmId}/close`

错误响应统一为：

```json
{"code":"stable_error_code","message":"可读说明"}
```

## 验证计划

本初始化基线刻意不生成单元测试、夹具或浏览器测试文件，也不声明 `test_command`。后续任务阶段负责补充测试，并在该阶段建立红绿验证。当前每条工作流通过 `cmd/workflowcheck` 启动真实 HTTP 服务并调用公开接口，校验成功状态、持久化结果和关键失败分支。告警能力由 `gate-alarm` 工作流覆盖登记、阻断、确认不代替关闭、基于处置依据关闭和历史不可改写，由 `alarm-review-concurrency` 工作流在高并发下验证告警处理与命令复核只能产生一个与现场一致的结果。
