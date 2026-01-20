# CGO层优化实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标:** 移除 cgoAdapter 和 cgoBroker 中的 CGO 封装层，直接使用新协议格式，消除二次序列化/解析

**架构:** 移除 `[len][topic][data]` 封装，直接使用 `[PackType][HeadLen][Header JSON][Content]`，从 Header 生成 topic

**技术栈:** Go 1.20+, easy-con library

---

## Task 1: cgoAdapter - 新增 generateTopic 方法

**Files:**
- Modify: `cgoAdapter.go`

**Step 1: Write the failing test**

先了解现有测试，确认需要测试的场景。运行测试确保当前状态：
```bash
go test ./unitTest/cgotest -v -run TestExpress
```

**Step 2: Add generateTopic method**

在 `cgoAdapter.go` 中添加方法（在 `readLoop` 方法之前）：

```go
func (adapter *cgoAdapter) generateTopic(pack IPack) string {
    prefix := adapter.setting.PreFix
    switch p := pack.(type) {
    case *PackReq:
        return BuildReqTopic(prefix, p.Header.To)
    case *PackResp:
        return BuildRespTopic(prefix, p.Header.From)
    case *PackNotice:
        return BuildNoticeTopic(prefix, p.Header.Route)
    case *PackLog:
        return BuildLogTopic(prefix)
    }
    return ""
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功，无错误

**Step 4: Commit**

```bash
git add cgoAdapter.go
git commit -m "feat(cgoAdapter): add generateTopic method for header-based topic generation"
```

---

## Task 2: cgoAdapter - 重构 readLoop

**Files:**
- Modify: `cgoAdapter.go`

**Step 1: Understand current readLoop**

阅读 `readLoop` 方法（当前第60-90行），理解现有逻辑。

**Step 2: Rewrite readLoop**

替换 `readLoop` 方法为：

```go
func (adapter *cgoAdapter) readLoop() {
    for {
        select {
        case <-adapter.stopChan:
            return
        case rawPack := <-adapter.readChan:
            // 直接解析新协议格式
            pack, err := UnmarshalPack(rawPack)
            if err != nil {
                adapter.Err("Deserialize error", err)
                continue
            }

            // 从Header生成topic
            topic := adapter.generateTopic(pack)

            // 匹配处理函数
            adapter.mu.Lock()
            t, b := adapter.topics[topic]
            adapter.mu.Unlock()

            if b {
                t.Func(pack)
            } else {
                // 尝试monitor topic
                tt := getMonitorTopic(topic)
                adapter.mu.Lock()
                t, b = adapter.topics[tt]
                adapter.mu.Unlock()
                if b {
                    t.Func(pack)
                }
            }
        }
    }
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功，无错误

**Step 4: Commit**

```bash
git add cgoAdapter.go
git commit -m "refactor(cgoAdapter): rewrite readLoop to use new protocol directly"
```

---

## Task 3: cgoAdapter - 修改 onPublish 方法

**Files:**
- Modify: `cgoAdapter.go`

**Step 1: Locate onPublish method**

找到 `onPublish` 方法（当前第132-136行）

**Step 2: Simplify onPublish**

替换为：

```go
func (adapter *cgoAdapter) onPublish(topic string, _ bool, pack IPack) error {
    js, _ := pack.Raw()
    return adapter.onWrite(js)
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add cgoAdapter.go
git commit -m "refactor(cgoAdapter): simplify onPublish to send new protocol directly"
```

---

## Task 4: cgoAdapter - 修改 PublishRaw 方法

**Files:**
- Modify: `cgoAdapter.go`

**Step 1: Locate PublishRaw method**

找到 `PublishRaw` 方法（当前第139-142行）

**Step 2: Simplify PublishRaw**

替换为：

```go
func (adapter *cgoAdapter) PublishRaw(topic string, isRetain bool, data []byte) error {
    return adapter.onWrite(data)
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add cgoAdapter.go
git commit -m "refactor(cgoAdapter): simplify PublishRaw to send data directly"
```

---

## Task 5: cgoAdapter - 修改 onSubscribe 方法

**Files:**
- Modify: `cgoAdapter.go`

**Step 1: Locate onSubscribe method**

找到 `onSubscribe` 方法（当前第120-131行）

**Step 2: Rewrite onSubscribe**

替换为：

```go
func (adapter *cgoAdapter) onSubscribe(topic string, pType EPType, f func(IPack)) {
    adapter.mu.Lock()
    adapter.topics[topic] = topicBack{EType: pType, Func: f}
    adapter.mu.Unlock()

    pack := newReqPack(adapter.setting.Module, "Broker", "Subscribe", topic)
    js, _ := pack.Raw()
    // To字段为"Broker"，接收方会生成"Request/Broker"topic
    err := adapter.onWrite(js)
    if err != nil {
        fmt.Printf("[%s]:CGO Subscribe Failed because %s\r\n", time.Now().Format("15:04:05.000"), err.Error())
    }
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add cgoAdapter.go
git commit -m "refactor(cgoAdapter): rewrite onSubscribe to send new protocol directly"
```

---

## Task 6: cgoAdapter - 移除废弃函数

**Files:**
- Modify: `cgoAdapter.go`

**Step 1: Remove unused functions**

删除以下函数：
- `marshalCgoPack` (第100-108行)
- `unMarshalCgoPack` (第109-118行)
- `callFunc` (第92-99行，如果还存在)

**Step 2: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 3: Run CGO tests**

```bash
go test ./unitTest/cgotest -v
```

Expected: 测试失败（因为 cgoBroker 还未修改）

**Step 4: Commit**

```bash
git add cgoAdapter.go
git commit -m "refactor(cgoAdapter): remove unused marshal/unmarshalCgoPack and callFunc functions"
```

---

## Task 7: cgoBroker - 新增 generateTopic 方法

**Files:**
- Modify: `cgoBroker.go`

**Step 1: Add generateTopic method**

在 `CgoBroker` 中添加方法（在 `Publish` 方法之前）：

```go
func (broker *CgoBroker) generateTopic(pack IPack) string {
    switch p := pack.(type) {
    case *PackReq:
        return ReqTopic + p.Header.To      // "Request/" + To
    case *PackResp:
        return RespTopic + p.Header.From   // "Response/" + From
    case *PackNotice:
        return NoticeTopic + "/" + p.Header.Route  // "Notice/" + Route
    case *PackLog:
        return LogTopic                     // "Log"
    }
    return ""
}
```

**Step 2: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add cgoBroker.go
git commit -m "feat(cgoBroker): add generateTopic method for header-based topic generation"
```

---

## Task 8: cgoBroker - 重写 Publish 方法

**Files:**
- Modify: `cgoBroker.go`

**Step 1: Locate current Publish method**

找到 `Publish` 方法（当前第32-52行）

**Step 2: Rewrite Publish**

替换为：

```go
func (broker *CgoBroker) Publish(raw []byte) error {
    // 直接解析新协议格式
    pack, err := UnmarshalPack(raw)
    if err != nil {
        broker.err(err)
        return err
    }

    // 从Header生成topic
    topic := broker.generateTopic(pack)

    // 特殊处理：Broker请求
    if topic == "Request/Broker" {
        if reqPack, ok := pack.(*PackReq); ok {
            code, resp := broker.onReq(*reqPack)
            respPack := newRespPack(*reqPack, code, resp)
            js, _ := respPack.Raw()
            broker.onSend(BuildRespTopic("", reqPack.Target()), js)
            return nil
        }
    }

    broker.onSend(topic, raw)
    return nil
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add cgoBroker.go
git commit -m "refactor(cgoBroker): rewrite Publish to parse new protocol and generate topic from header"
```

---

## Task 9: cgoBroker - 更新 onSend 方法

**Files:**
- Modify: `cgoBroker.go`

**Step 1: Locate onSend method**

找到 `onSend` 方法（当前第60-94行）

**Step 2: Update onSend**

将参数 `cgoRaw []byte` 改为 `raw []byte`，方法签名和注释更新：

```go
func (broker *CgoBroker) onSend(topic string, raw []byte) {
    var modulesToNotify []func([]byte)

    broker.lock.RLock()
    modules, ok := broker.topics[topic]
    if ok {
        for module := range modules {
            f, b := broker.clients[module]
            if b {
                modulesToNotify = append(modulesToNotify, f)
            }
        }
    }
    broker.lock.RUnlock()

    monitorTopic := getMonitorTopic(topic)
    broker.lock.RLock()

    modules, ok = broker.topics[monitorTopic]
    if ok {
        for module := range modules {
            f, b := broker.clients[module]
            if b {
                modulesToNotify = append(modulesToNotify, f)
            }
        }
    }
    broker.lock.RUnlock()

    // 直接传递新协议格式数据
    for _, f := range modulesToNotify {
        f(raw)
    }
}
```

**Step 3: Verify code compiles**

```bash
go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add cgoBroker.go
git commit -m "refactor(cgoBroker): update onSend to accept new protocol format"
```

---

## Task 10: 运行完整测试套件

**Files:**
- Test: `unitTest/cgotest/`

**Step 1: Build all**

```bash
go build ./...
```

Expected: 编译成功

**Step 2: Run CGO tests**

```bash
go test ./unitTest/cgotest -v
```

Expected: 所有测试通过

**Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: 所有测试通过

**Step 4: Fix any failing tests**

如果有测试失败，分析原因并修复。

**Step 5: Commit**

```bash
git add .
git commit -m "test: ensure all tests pass after CGO optimization"
```

---

## Task 11: 最终验证

**Files:**
- All modified files

**Step 1: Verify functionality**

检查以下功能：
- Subscribe 流程正常
- Request/Response 流程正常
- Notice/Log 流程正常
- Monitor topic 路由正常

**Step 2: Code review**

检查代码：
- 无遗留的 CGO 封装调用
- 错误处理完整
- 代码风格一致

**Step 3: Documentation check**

确认设计文档和实施计划一致。

**Step 4: Final commit**

```bash
git add docs/plans/2025-01-19-cgo-optimization.md docs/plans/2025-01-19-cgo-optimization-design.md
git commit -m "docs: add CGO optimization design and implementation plan"
```

---

**Remember:**
- 每个任务完成后立即提交
- 使用 @superpowers:test-driven-development 进行测试驱动开发
- 遵循 DRY, YAGNI 原则
- 遇到问题及时沟通
