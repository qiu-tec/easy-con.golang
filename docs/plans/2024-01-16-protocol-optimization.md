# 协议优化实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标:** 将 easy-con 的消息序列化从纯 JSON 改为 Header+Content 分离的混合协议，提升 Proxy 转发性能并减少内存开销。

**架构:**
- 新协议格式: `[PackType(1 byte)][HeadLen(2 bytes)][Header JSON][Content Bytes]`
- Content 统一为 `[]byte`，序列化由用户负责
- 支持新旧协议自动识别和共存

**技术栈:**
- Go 1.20+
- encoding/json
- 现有测试框架

---

## 任务 1: 添加新协议的 Header 结构体

**文件:**
- 修改: `protocol.go` (在文件开头添加，约第 15 行之后)

**步骤 1: 添加 Header 结构体定义**

在 `protocol.go` 文件中，在 `type EProtocol string` 之后添加以下代码:

```go
// PackType 常量定义
const (
	PackTypeReq    byte = 0x01
	PackTypeResp   byte = 0x02
	PackTypeNotice byte = 0x03
	PackTypeLog    byte = 0x04
	PackTypeOld    byte = 0x7B // '{' 用于识别旧协议
)

// 新协议的 Header 结构体
type PackBaseHeader struct {
	PType EPType
	Id    uint64
}

type PackReqHeader struct {
	PackBaseHeader
	From    string
	ReqTime string
	To      string
	Route   string
}

type PackRespHeader struct {
	PackReqHeader
	RespTime string
	RespCode int
	Error    string
}

type PackNoticeHeader struct {
	PackBaseHeader
	From  string
	Route string
	Retain bool
}

type PackLogHeader struct {
	PackBaseHeader
	From    string
	Level   string
	LogTime string
	Error   string
	Content string // 日志的 content 在 header 里
}
```

**步骤 2: 运行测试确保无编译错误**

```bash
cd E:\work2024\golang\easy-con
go build .
```

预期: 编译成功，无错误

**步骤 3: 提交**

```bash
git add protocol.go
git commit -m "feat: add new protocol header structures"
```

---

## 任务 2: 修改 PackReq.Content 类型为 []byte

**文件:**
- 修改: `protocol.go` (PackReq 结构体，约第 69-76 行)

**步骤 1: 修改 Content 字段类型**

将 `PackReq` 结构体的 `Content` 字段类型从 `any` 改为 `[]byte`:

```go
// PackReq 请求
type PackReq struct {
	packBase
	From    string
	ReqTime string
	To      string
	Route   string
	Content []byte // 修改为 []byte
}
```

**步骤 2: 修改 PackNotice.Content 类型**

同样修改 `PackNotice` 的 `Content` 字段:

```go
type PackNotice struct {
	packBase
	From    string
	Route   string
	Retain  bool
	Content []byte // 修改为 []byte
}
```

**步骤 3: 运行测试查看失败情况**

```bash
cd E:\work2024\golang\easy-con
go test ./... -v 2>&1 | head -50
```

预期: 部分测试失败，因为接口签名改变了

**步骤 4: 提交**

```bash
git add protocol.go
git commit -m "feat: change Content field type to []byte"
```

---

## 任务 3: 实现新协议的序列化方法

**文件:**
- 修改: `protocol.go` (替换 PackReq.Raw() 方法，约第 97-99 行)

**步骤 1: 重写 PackReq.Raw() 方法**

```go
func (p *PackReq) Raw() ([]byte, error) {
	// 1. 构建 Header JSON
	header := PackReqHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:    p.From,
		ReqTime: p.ReqTime,
		To:      p.To,
		Route:   p.Route,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 2. Content 已经是 []byte
	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	// 3. 打包: [PackType][HeadLen(2)][Header JSON][Content]
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeReq
	buf[1] = byte(headLen >> 8)   // 大端序
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}
```

**步骤 2: 重写 PackResp.Raw() 方法**

```go
func (p *PackResp) Raw() ([]byte, error) {
	// 1. 构建 Header JSON
	header := PackRespHeader{
		PackReqHeader: PackReqHeader{
			PackBaseHeader: PackBaseHeader{
				PType: p.PType,
				Id:    p.Id,
			},
			From:    p.From,
			ReqTime: p.ReqTime,
			To:      p.To,
			Route:   p.Route,
		},
		RespTime: p.RespTime,
		RespCode: int(p.RespCode),
		Error:    p.Error,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 2. Content 已经是 []byte
	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	// 3. 打包
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeResp
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}
```

**步骤 3: 重写 PackNotice.Raw() 方法**

```go
func (p *PackNotice) Raw() ([]byte, error) {
	header := PackNoticeHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:  p.From,
		Route: p.Route,
		Retain: p.Retain,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeNotice
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}
```

**步骤 4: 重写 PackLog.Raw() 方法**

PackLog 比较特殊，Content 是 string 且在 header 里:

```go
func (p *PackLog) Raw() ([]byte, error) {
	header := PackLogHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:    p.From,
		Level:   string(p.Level),
		LogTime: p.LogTime,
		Error:   p.Error,
		Content: p.Content, // 日志的 content 在 header 里
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 日志包没有外部 Content，只有 Header
	headLen := len(headerJson)
	totalLen := 3 + headLen

	buf := make([]byte, totalLen)
	buf[0] = PackTypeLog
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)

	return buf, nil
}
```

**步骤 5: 编译检查**

```bash
cd E:\work2024\golang\easy-con
go build .
```

预期: 编译成功

**步骤 6: 提交**

```bash
git add protocol.go
git commit -m "feat: implement new protocol serialization"
```

---

## 任务 4: 实现新协议的反序列化方法

**文件:**
- 修改: `protocol.go` (修改 unmarshalPack 函数，约第 382-414 行)

**步骤 1: 添加协议识别函数**

```go
// detectProtocol 检测协议类型
func detectProtocol(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// 0x7B = '{' 表示旧 JSON 协议
	return data[0] == PackTypeOld
}
```

**步骤 2: 保留旧协议反序列化函数**

重命名旧的 `unmarshalPack` 为 `unmarshalPackOld`:

```go
func unmarshalPackOld(pType EPType, data []byte) (IPack, error) {
	switch pType {
	case EPTypeReq:
		var pack PackReq
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	case EPTypeResp:
		var pack PackResp
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	case EPTypeNotice:
		var pack PackNotice
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	case EPTypeLog:
		var pack PackLog
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	}
	return nil, fmt.Errorf("unknown pack type %s", pType)
}
```

**步骤 3: 实现新协议反序列化函数**

```go
func unmarshalPackNew(data []byte) (IPack, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("invalid pack length: %d", len(data))
	}

	packType := data[0]
	headLen := int(data[1])<<8 | int(data[2])

	if len(data) < 3+headLen {
		return nil, fmt.Errorf("invalid header length: dataLen=%d, headLen=%d", len(data), headLen)
	}

	headerBytes := data[3 : 3+headLen]
	contentBytes := data[3+headLen:]

	// 对于没有 content 的情况，返回空切片
	if len(contentBytes) == 0 {
		contentBytes = []byte{}
	}

	switch packType {
	case PackTypeReq:
		var header PackReqHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal REQ header: %w", err)
		}
		return &PackReq{
			packBase: packBase{PType: header.PType, Id: header.Id},
			From:     header.From,
			To:       header.To,
			Route:    header.Route,
			ReqTime:  header.ReqTime,
			Content:  contentBytes,
		}, nil

	case PackTypeResp:
		var header PackRespHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RESP header: %w", err)
		}
		return &PackResp{
			PackReq: PackReq{
				packBase: packBase{PType: header.PType, Id: header.Id},
				From:     header.From,
				To:       header.To,
				Route:    header.Route,
				ReqTime:  header.ReqTime,
				Content:  contentBytes,
			},
			RespTime: header.RespTime,
			RespCode: EResp(header.RespCode),
			Error:    header.Error,
		}, nil

	case PackTypeNotice:
		var header PackNoticeHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal NOTICE header: %w", err)
		}
		return &PackNotice{
			packBase: packBase{PType: header.PType, Id: header.Id},
			From:     header.From,
			Route:    header.Route,
			Retain:   header.Retain,
			Content:  contentBytes,
		}, nil

	case PackTypeLog:
		var header PackLogHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal LOG header: %w", err)
		}
		return &PackLog{
			packBase: packBase{PType: header.PType, Id: header.Id},
			From:     header.From,
			Level:    ELogLevel(header.Level),
			LogTime:  header.LogTime,
			Error:    header.Error,
			Content:  header.Content,
		}, nil

	default:
		return nil, fmt.Errorf("unknown pack type: 0x%02x", packType)
	}
}
```

**步骤 4: 添加统一入口函数**

```go
// UnmarshalPack 统一的反序列化入口，自动识别新旧协议
func UnmarshalPack(data []byte) (IPack, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// 根据第一个字节判断协议类型
	if data[0] == PackTypeOld { // '{'
		// 旧协议：需要先读取 PType 字段来确定类型
		// 旧协议的 JSON 第一个字段是 PType
		var typeOnly struct {
			PType EPType `json:"PType"`
		}
		if err := json.Unmarshal(data, &typeOnly); err != nil {
			return nil, fmt.Errorf("failed to detect old protocol type: %w", err)
		}
		return unmarshalPackOld(typeOnly.PType, data)
	}

	// 新协议
	return unmarshalPackNew(data)
}
```

**步骤 5: 编译检查**

```bash
cd E:\work2024\golang\easy-con
go build .
```

**步骤 6: 提交**

```bash
git add protocol.go
git commit -m "feat: implement new protocol deserialization with auto-detection"
```

---

## 任务 5: 修改接口回调签名

**文件:**
- 修改: `interface.go` (第 13-14 行)

**步骤 1: 修改回调函数签名**

```go
type ReqHandler func(pack PackReq) (EResp, []byte)  // any -> []byte
type OnReqHandler func(pack PackReq) // 保持不变
```

**步骤 2: 编译查看影响范围**

```bash
cd E:\work2024\golang\easy-con
go build .
```

预期: 编译失败，显示所有需要修改的地方

**步骤 3: 提交**

```bash
git add interface.go
git commit -m "feat: change ReqHandler return type to []byte"
```

---

## 任务 6: 修改 coreAdapter.go 适配新签名

**文件:**
- 修改: `coreAdapter.go`

**步骤 1: 修改 Req 相关方法**

找到 `Req` 和 `ReqWithTimeout` 方法，修改返回类型:

```go
func (adapter *coreAdapter) Req(module, route string, params any) PackResp {
	// ...
	if params == nil {
		p.Content = []byte{}
	} else if bytes, ok := params.([]byte); ok {
		p.Content = bytes
	} else {
		// 兼容旧代码，自动序列化
		jsonData, err := json.Marshal(params)
		if err != nil {
			return PackResp{
				PackReq: pack,
				RespCode: ERespBadReq,
				Error:    err.Error(),
			}
		}
		p.Content = jsonData
	}
	// ...
}
```

**步骤 2: 修改 onReqInner 方法**

```go
func (adapter *coreAdapter) onReqInner(req PackReq) PackResp {
	// ...
	code, content := adapter.setting.onReqRec(req)
	// content 现在是 []byte
	pack := newRespPack(req, code, content)
	// ...
}
```

**步骤 3: 编译检查**

```bash
cd E:\work2024\golang\easy-con
go build .
```

**步骤 4: 提交**

```bash
git add coreAdapter.go
git commit -m "feat: adapt coreAdapter to new content type"
```

---

## 任务 7: 修改 proxy.go 适配新协议

**文件:**
- 修改: `proxy.go`

**步骤 1: 修改 onReqA 方法**

```go
func (p *proxy) onReqA(pack PackReq) (EResp, []byte) { // 签名变更
	if pack.To == "Broker" {
		return ERespBypass, []byte{}
	}
	if strings.HasPrefix(pack.From, p.sa.Module) {
		return ERespBypass, []byte{}
	}

	// 直接转发原始字节，零拷贝
	topic := BuildReqTopic(p.sb.PreFix, pack.To)

	// 获取原始消息
	rawData, err := pack.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy pack raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return ERespError, []byte{}
	}

	// 直接发布原始字节
	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy req (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), pack.Id, err.Error())
		return ERespError, []byte{}
	}

	return ERespBypass, []byte{}
}
```

**步骤 2: 修改 onReqB 方法**

类似 onReqA 的修改

**步骤 3: 修改 Notice 转发**

```go
func (p *proxy) onNoticeA(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sa.Module) {
		return
	}
	notice.From = p.sb.Module + "/" + notice.From

	topic := BuildNoticeTopic(p.sb.PreFix, notice.Route)

	rawData, err := notice.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), notice.Id, err.Error())
	}
}
```

**步骤 4: 添加 PublishRaw 方法到 IAdapter 接口**

在 `interface.go` 中添加:

```go
type IAdapter interface {
	// ... 现有方法
	PublishRaw(topic string, isRetain bool, data []byte) error
	// ...
}
```

**步骤 5: 编译检查**

```bash
cd E:\work2024\golang\easy-con
go build .
```

**步骤 6: 提交**

```bash
git add proxy.go interface.go
git commit -m "feat: adapt proxy for zero-copy forwarding"
```

---

## 任务 8: 修改 mqttAdapter.go 使用新反序列化

**文件:**
- 修改: `mqttAdapter.go`

**步骤 1: 找到消息解析位置**

找到 MQTT 消息接收回调中的 `unmarshalPack` 调用

**步骤 2: 替换为新的 UnmarshalPack**

```go
// 将
pack, err := unmarshalPack(pType, payload)

// 改为
pack, err := UnmarshalPack(payload)
```

**步骤 3: 编译检查**

```bash
cd E:\work2024\golang\easy-con
go build .
```

**步骤 4: 提交**

```bash
git add mqttAdapter.go
git commit -m "feat: use new unmarshal with auto-detection"
```

---

## 任务 9: 编写协议单元测试

**文件:**
- 创建: `unitTest/protocol_test.go`

**步骤 1: 创建测试文件**

```go
package easyCon

import (
	"encoding/json"
	"testing"
)

func TestNewProtocolReqSerialization(t *testing.T) {
	req := PackReq{
		packBase: packBase{
			PType: EPTypeReq,
			Id:    12345,
		},
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "testRoute",
		ReqTime: "2024-01-16 10:00:00.000",
		Content: []byte("test content"),
	}

	data, err := req.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	// 验证格式
	if len(data) < 3 {
		t.Fatalf("Data too short: %d", len(data))
	}

	if data[0] != PackTypeReq {
		t.Errorf("PackType mismatch: got 0x%02x, want 0x%02x", data[0], PackTypeReq)
	}

	headLen := int(data[1])<<8 | int(data[2])
	t.Logf("Header length: %d", headLen)
}

func TestNewProtocolReqRoundTrip(t *testing.T) {
	original := PackReq{
		packBase: packBase{
			PType: EPTypeReq,
			Id:    99999,
		},
		From:    "TestModule",
		To:      "TargetModule",
		Route:   "getUserInfo",
		ReqTime: "2024-01-16 11:00:00.000",
		Content: []byte(`{"name":"test"}`),
	}

	data, err := original.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedReq, ok := decoded.(*PackReq)
	if !ok {
		t.Fatalf("Not a PackReq")
	}

	if decodedReq.Id != original.Id {
		t.Errorf("Id mismatch: got %d, want %d", decodedReq.Id, original.Id)
	}

	if decodedReq.From != original.From {
		t.Errorf("From mismatch: got %s, want %s", decodedReq.From, original.From)
	}

	if decodedReq.To != original.To {
		t.Errorf("To mismatch: got %s, want %s", decodedReq.To, original.To)
	}

	if string(decodedReq.Content) != string(original.Content) {
		t.Errorf("Content mismatch")
	}
}

func TestOldProtocolCompatibility(t *testing.T) {
	oldReq := PackReq{
		packBase: packBase{
			PType: EPTypeReq,
			Id:    11111,
		},
		From:    "OldModule",
		To:      "Target",
		Route:   "oldRoute",
		ReqTime: "2024-01-16 12:00:00.000",
		Content: []byte("old content"), // 新协议已经是 []byte
	}

	// 使用旧 JSON 格式序列化
	oldJson, err := json.Marshal(oldReq)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// 确保以 '{' 开头
	if oldJson[0] != '{' {
		t.Errorf("Old JSON should start with '{'")
	}

	// 新的 UnmarshalPack 应该能识别旧协议
	decoded, err := UnmarshalPack(oldJson)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed for old protocol: %v", err)
	}

	decodedReq, ok := decoded.(*PackReq)
	if !ok {
		t.Fatalf("Not a PackReq")
	}

	if decodedReq.Id != oldReq.Id {
		t.Errorf("Id mismatch: got %d, want %d", decodedReq.Id, oldReq.Id)
	}
}

func TestProtocolAutoDetection(t *testing.T) {
	// 新协议
	newReq := PackReq{
		packBase: packBase{PType: EPTypeReq, Id: 1},
		From:     "A", To: "B", Route: "r",
		Content:  []byte("data"),
	}
	newData, _ := newReq.Raw()

	pack, err := UnmarshalPack(newData)
	if err != nil {
		t.Errorf("Failed to unmarshal new protocol: %v", err)
	}
	if pack.GetType() != EPTypeReq {
		t.Error("Wrong type for new protocol")
	}

	// 旧协议
	oldReq := PackReq{
		packBase: packBase{PType: EPTypeReq, Id: 2},
		From:     "C", To: "D", Route: "s",
		Content:  []byte("old"),
	}
	oldData, _ := json.Marshal(oldReq)

	pack, err = UnmarshalPack(oldData)
	if err != nil {
		t.Errorf("Failed to unmarshal old protocol: %v", err)
	}
	if pack.GetType() != EPTypeReq {
		t.Error("Wrong type for old protocol")
	}
}
```

**步骤 2: 运行测试**

```bash
cd E:\work2024\golang\easy-con
go test -v -run TestNewProtocol
```

预期: 部分通过，部分可能失败（根据实现情况）

**步骤 3: 修复失败的测试**

根据测试输出修复代码问题

**步骤 4: 所有测试通过**

```bash
cd E:\work2024\golang\easy-con
go test -v unitTest/protocol_test.go protocol.go
```

**步骤 5: 提交**

```bash
git add unitTest/protocol_test.go
git commit -m "test: add protocol unit tests"
```

---

## 任务 10: 更新现有测试用例

**文件:**
- 修改: `unitTest/proxy/proxy_test.go`
- 修改: 其他测试文件

**步骤 1: 查看现有测试**

```bash
cd E:\work2024\golang\easy-con
go test ./unitTest/proxy -v
```

**步骤 2: 修改回调函数签名**

将所有 `onReq` 函数的返回值改为 `[]byte`:

```go
func onReqA(pack easyCon.PackReq) (easyCon.EResp, []byte) {
	return easyCon.ERespSuccess, []byte(`{"result":"ok"}`)
}
```

**步骤 3: 修改 Req 调用**

```go
// 旧的
resp := adapter.Req("Target", "route", param)

// 新的 - 传入 []byte
jsonData, _ := json.Marshal(param)
resp := adapter.Req("Target", "route", jsonData)
```

**步骤 4: 运行测试**

```bash
cd E:\work2024\golang\easy-con
go test ./unitTest/proxy -v
```

**步骤 5: 逐个修复所有测试文件**

```bash
go test ./unitTest/... -v
```

**步骤 6: 提交**

```bash
git add unitTest/
git commit -m "test: update test cases for new protocol"
```

---

## 任务 11: 性能基准测试

**文件:**
- 创建: `unitTest/benchmark_test.go`

**步骤 1: 创建基准测试**

```go
package easyCon

import (
	"encoding/json"
	"testing"
)

func BenchmarkNewProtocolMarshal(b *testing.B) {
	req := PackReq{
		packBase: packBase{PType: EPTypeReq, Id: 12345},
		From:     "ModuleA",
		To:       "ModuleB",
		Route:    "benchmarkRoute",
		ReqTime:  "2024-01-16 10:00:00.000",
		Content:  make([]byte, 1024), // 1KB content
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = req.Raw()
	}
}

func BenchmarkOldProtocolMarshal(b *testing.B) {
	req := map[string]any{
		"PType":   "REQ",
		"Id":      12345,
		"From":    "ModuleA",
		"To":      "ModuleB",
		"Route":   "benchmarkRoute",
		"ReqTime": "2024-01-16 10:00:00.000",
		"Content": make([]byte, 1024),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = json.Marshal(req)
	}
}

func BenchmarkNewProtocolUnmarshal(b *testing.B) {
	req := PackReq{
		packBase: packBase{PType: EPTypeReq, Id: 12345},
		From:     "ModuleA",
		To:       "ModuleB",
		Route:    "benchmarkRoute",
		ReqTime:  "2024-01-16 10:00:00.000",
		Content:  make([]byte, 1024),
	}
	data, _ := req.Raw()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UnmarshalPack(data)
	}
}
```

**步骤 2: 运行基准测试**

```bash
cd E:\work2024\golang\easy-con
go test -bench=Benchmark -benchmem
```

**步骤 3: 记录性能对比**

将结果记录到文档中

**步骤 4: 提交**

```bash
git add unitTest/benchmark_test.go
git commit -m "test: add performance benchmarks"
```

---

## 任务 12: 文档更新

**文件:**
- 修改: `README.md`
- 修改: `docs/plans/2024-01-16-protocol-optimization-design.md`

**步骤 1: 更新 README.md**

添加新协议使用示例:

```markdown
## 发送消息

```go
// 发送请求 - Content 统一使用 []byte
jsonData, _ := json.Marshal(userObj)
resp := adapter.Req("TargetModule", "getUserInfo", jsonData)

// 直接发送字符串
resp := adapter.Req("Logger", "log", []byte("hello world"))

// 直接发送二进制数据
resp := adapter.Req("Storage", "upload", fileData)
```

## 处理请求

```go
func onReq(pack PackReq) (EResp, []byte) {
    // pack.Content 是 []byte
    var params UserParams
    json.Unmarshal(pack.Content, &params)

    result := process(params)

    respData, _ := json.Marshal(result)
    return ERespSuccess, respData
}
```
```

**步骤 2: 更新设计文档状态**

将 `2024-01-16-protocol-optimization-design.md` 的状态改为"已实现"

**步骤 3: 提交**

```bash
git add README.md docs/plans/2024-01-16-protocol-optimization-design.md
git commit -m "docs: update documentation for new protocol"
```

---

## 任务 13: 最终验证

**步骤 1: 运行所有测试**

```bash
cd E:\work2024\golang\easy-con
go test ./... -v
```

预期: 所有测试通过

**步骤 2: 编译检查**

```bash
go build .
go build ./app/mqttProxy
```

预期: 编译成功，无警告

**步骤 3: 运行 expressTest**

```bash
go test ./unitTest/expressTest -v -count=10
```

预期: 高并发测试通过

**步骤 4: 运行 proxy 测试**

```bash
go test ./unitTest/proxy -v
```

预期: Proxy 转发测试通过

**步骤 5: 性能对比**

```bash
go test -bench=. -benchmem
```

验证性能提升符合预期

**步骤 6: 最终提交**

```bash
git add .
git commit -m "feat: complete protocol optimization implementation"
```

---

## 总结

完成以上任务后：

1. ✅ 新协议格式: `[PackType][HeadLen][Header JSON][Content Bytes]`
2. ✅ Content 统一为 `[]byte`，序列化由用户负责
3. ✅ 支持新旧协议自动识别
4. ✅ Proxy 零拷贝转发
5. ✅ 所有测试通过
6. ✅ 性能提升可验证

## 注意事项

- 修改过程中保持 TDD 原则，先写测试再实现
- 每个任务完成后立即提交，便于回滚
- 遇到问题及时回退到上一个可用状态
- 保持与现有 CGO 绑定的兼容性

---

## 更新日志

### 2026-01-20: 移除老协议兼容代码

**背景:** 经过内部沟通确认，不再需要向下兼容老协议，因此移除所有兼容代码。

**变更内容:**

1. **protocol.go**
   - 移除 `PackTypeOld` 常量
   - 移除 `unmarshalPackOld` 函数
   - 简化 `UnmarshalPack` 函数，移除旧协议检测逻辑
   - 移除已注释的 `serialize` 和 `deserialize` 函数
   - 将 `unmarshalPackNew` 重命名为 `unmarshalPack`

2. **unitTest/protocol_test.go**
   - 移除 `TestOldProtocolCompatibility` 测试
   - 简化 `TestProtocolAutoDetection` 测试，移除旧协议部分
   - 移除不再需要的 `encoding/json` 导入
   - 更新文件头部描述

3. **unitTest/benchmark_test.go**
   - 移除 `BenchmarkOldProtocolMarshal` 基准测试
   - 移除 `BenchmarkOldProtocolUnmarshal` 基准测试
   - 移除不再需要的 `encoding/json` 导入

**影响:** 此变更后，系统仅支持新的二进制协议格式，不再能够解析或处理旧的 JSON 格式消息。
