# 协议优化设计文档

**日期**: 2024-01-16
**状态**: 已实现 (2024-01-19)
**性能提升**: 序列化 5.6x，反序列化 5.1x
**优先级**: 高

## 背景与问题

当前 easy-con 使用 JSON 序列化整个消息包，存在以下问题：

1. **转发性能差**：Proxy 转发时需要对整个包进行反序列化再序列化，内存开销大
2. **Content 二次编码**：
   - 如果 Content 是 JSON，会被转义再包装一层
   - 如果 Content 是 `[]byte`，会被 base64 编码，体积增大约 33%
3. **解析不必要的内容**：Proxy 只需要路由信息（From, To, Route, Id），不需要解析 Content

## 设计目标

1. **性能提升**：减少序列化开销，提高转发效率
2. **职责分离**：easy-con 只负责传输，内容序列化由用户决定
3. **兼容性**：支持新旧协议共存，逐步迁移
4. **简洁性**：协议格式简单，易于理解和维护

## 新协议设计

### 协议格式

```
┌──────────┬───────────┬─────────────┬────────────────┐
│ PackType │  HeadLen  │ Header JSON │ Content Bytes  │
│ 1 byte   │ 2 bytes   │ N bytes     │ M bytes        │
└──────────┴───────────┴─────────────┴────────────────┘
```

### 字段说明

| 字段 | 大小 | 说明 |
|------|------|------|
| PackType | 1 byte | 包类型：0x01=REQ, 0x02=RESP, 0x03=NOTICE, 0x04=LOG |
| HeadLen | 2 bytes | Header JSON 长度，大端序（网络字节序） |
| Header JSON | N bytes | 元数据，JSON 格式，大驼峰命名 |
| Content Bytes | M bytes | 原始字节数组，零拷贝透传 |

### Header JSON 结构

利用嵌套关系精简结构定义：

```go
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
    Content string  // 日志的 content 在 header 里
}
```

**JSON 示例（自动大驼峰）：**
```json
{
  "PType": "REQ",
  "Id": 12345,
  "From": "ModuleA",
  "To": "ModuleB",
  "Route": "getUserInfo",
  "ReqTime": "2024-01-16 10:30:00.123"
}
```

### 协议识别机制

通过第一个字节自动识别新旧协议：

| 第一个字节 | 协议类型 |
|-----------|---------|
| `0x7B` (`{`) | 旧 JSON 协议 |
| `0x01-0x04` | 新混合协议 |

## 用户接口设计

### 发送接口

```go
// 基础方法 - Content 统一为 []byte
adapter.Req(module, route string, data []byte)

// 用户自行序列化
jsonData, _ := json.Marshal(userObj)
adapter.Req("ModuleB", "updateUser", jsonData)

// 发送字符串
adapter.Req("ModuleB", "log", []byte("hello"))

// 发送原始字节
adapter.Req("ModuleB", "upload", fileData)
```

### 接收回调

```go
// 回调函数签名
func onReq(pack PackReq) (EResp, []byte) {
    // pack.Content 是 []byte
    // 用户根据业务逻辑自行解析

    var user User
    json.Unmarshal(pack.Content, &user)

    respData, _ := json.Marshal(result)
    return ERespSuccess, respData
}
```

## Proxy 转发流程

### 转发逻辑

1. **读取头部**：读取 PackType (1 byte) + HeadLen (2 bytes)
2. **解析 Header**：解析 Header JSON，获取 From, To, Route, Id
3. **路由决策**：根据 To 字段决定转发目标
4. **直接转发**：整个消息（包括 Content）直接透传，零拷贝

### 性能优势

- **无需解析 Content**：转发时完全不涉及 Content 的序列化/反序列化
- **内存高效**：Content 使用原始字节切片，可使用 `data[3+headLen:]` 零拷贝
- **CPU 高效**：只需要解析很小的 Header JSON

## 兼容性策略

### 新旧协议共存

- **自动识别**：通过第一个字节自动判断协议类型
- **渐进迁移**：模块可以逐个升级，Proxy 同时支持两种协议
- **配置无关**：不需要手动配置协议版本

### 迁移路径

1. **第一阶段**：Proxy 支持新旧协议自动识别和转发
2. **第二阶段**：业务模块逐步升级到新协议
3. **第三阶段**：所有模块升级完成后，移除旧协议支持

## 设计原则

1. **YAGNI**：不需要的功能一律不加（如 ContentType）
2. **职责分离**：easy-con 是通信库，不负责内容序列化
3. **零拷贝**：Content 尽可能使用切片，避免内存复制
4. **向后兼容**：支持新旧协议平滑过渡

## 待讨论事项

- [ ] Proxy 转发的具体实现细节
- [ ] 性能基准测试方案
- [ ] 错误处理机制
- [ ] 日志和调试支持
- [ ] 迁移时间表和计划

## 相关文件

- 当前实现：`protocol.go`, `mqttAdapter.go`, `proxy.go`
- 测试文件：`unitTest/proxy/`, `unitTest/expressTest/`
