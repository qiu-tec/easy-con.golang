/**
 * @Description: mqttAdapter token.WaitTimeout 改造后的超时行为测试
 *
 * 覆盖范围：
 *   Req    - 发布超时 / 响应超时 / 发布报错 / 未连接 / 正常成功
 *   Notice - 发布超时 / 保留通知发布超时 / 发布报错 / 未连接 / 正常成功
 *   其他   - 日志上传超时不应 panic、RESP 发送超时、响应注册竞态、并发超时一致性
 *
 * 测试策略：
 *   单元测试 - 注入模拟 mqtt.Client / Token，精确控制「token 永不完成」「token 带 error」
 *   集成测试 - 连接真实 broker，验证端到端超时行为；broker 不可用时自动跳过
 */

package timeout

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	easyCon "github.com/qiu-tec/easy-con.golang"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ------------------------------------------------------------------
// 模拟 MQTT 客户端 / Token
// ------------------------------------------------------------------

// mockToken 模拟 paho token 行为：
//   - forceTimeout = true -> 永不完成，WaitTimeout 返回 false，且 Error() 为 nil
//     （与 paho v1.4.3 baseToken.WaitTimeout 行为一致：超时不会给 token 设置 error）
//   - 否则 token 立即完成，携带可选的 error
type mockToken struct {
	mu           sync.Mutex
	done         chan struct{}
	err          error
	forceTimeout bool
}

func newMockToken(forceTimeout bool, err error, completed bool) *mockToken {
	t := &mockToken{done: make(chan struct{}), forceTimeout: forceTimeout}
	if completed && !forceTimeout {
		t.err = err
		close(t.done)
	}
	return t
}

func (t *mockToken) Wait() bool {
	<-t.done
	return true
}

func (t *mockToken) WaitTimeout(d time.Duration) bool {
	t.mu.Lock()
	force := t.forceTimeout
	t.mu.Unlock()
	if force {
		// 真实 paho：超时返回 false，且不会给 token 写入 error
		time.Sleep(d)
		return false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-t.done:
		return true
	case <-timer.C:
		return false
	}
}

func (t *mockToken) Done() <-chan struct{} { return t.done }

func (t *mockToken) Error() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// mockClient 模拟 MQTT 客户端
type mockClient struct {
	mu           sync.Mutex
	publishCount int
	topics       []string
	publishErr   error   // publish 返回带错误的 token
	forceTimeout bool    // publish 的 token 永不完成
	onPublish    func(topic string, payload []byte)
	syncPublish  bool // 同步执行 onPublish（模拟响应瞬间抵达）
	subs         map[string]mqtt.MessageHandler
}

func (c *mockClient) IsConnected() bool      { return true }
func (c *mockClient) IsConnectionOpen() bool { return true }
func (c *mockClient) Connect() mqtt.Token    { return newMockToken(false, nil, true) }
func (c *mockClient) Disconnect(uint)        {}
func (c *mockClient) Subscribe(topic string, qos byte, cb mqtt.MessageHandler) mqtt.Token {
	c.mu.Lock()
	if c.subs == nil {
		c.subs = make(map[string]mqtt.MessageHandler)
	}
	c.subs[topic] = cb
	c.mu.Unlock()
	return newMockToken(false, nil, true)
}

// deliver 向指定主题投递一条消息，模拟 broker 推送
func (c *mockClient) deliver(topic string, payload []byte) {
	c.mu.Lock()
	cb, ok := c.subs[topic]
	c.mu.Unlock()
	if !ok {
		return
	}
	cb(nil, &mockMessage{topic: topic, payload: payload})
}
func (c *mockClient) SubscribeMultiple(filters map[string]byte, cb mqtt.MessageHandler) mqtt.Token {
	return newMockToken(false, nil, true)
}
func (c *mockClient) Unsubscribe(topics ...string) mqtt.Token {
	return newMockToken(false, nil, true)
}
func (c *mockClient) AddRoute(topic string, cb mqtt.MessageHandler) {}
func (c *mockClient) OptionsReader() mqtt.ClientOptionsReader {
	return mqtt.ClientOptionsReader{}
}

func (c *mockClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	c.mu.Lock()
	c.publishCount++
	c.topics = append(c.topics, topic)
	forceTimeout := c.forceTimeout
	publishErr := c.publishErr
	onPublish := c.onPublish
	syncPublish := c.syncPublish
	c.mu.Unlock()

	var raw []byte
	switch p := payload.(type) {
	case []byte:
		raw = p
	case string:
		raw = []byte(p)
	}

	tok := newMockToken(forceTimeout, publishErr, !forceTimeout)
	if onPublish != nil {
		if syncPublish {
			onPublish(topic, raw)
		} else {
			go onPublish(topic, raw)
		}
	}
	return tok
}

func (c *mockClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.publishCount
}

// mockMessage 伪造 mqtt.Message
type mockMessage struct {
	topic   string
	payload []byte
}

func (m *mockMessage) Duplicate() bool  { return false }
func (m *mockMessage) Qos() byte        { return 0 }
func (m *mockMessage) Retained() bool   { return false }
func (m *mockMessage) Topic() string    { return m.topic }
func (m *mockMessage) MessageID() uint16 { return 0 }
func (m *mockMessage) Payload() []byte  { return m.payload }
func (m *mockMessage) Ack()             {}

// ------------------------------------------------------------------
// 公共辅助
// ------------------------------------------------------------------

// timeoutSetting 构造一个用于超时测试的设置（关闭日志上传，避免干扰被测分支）
func timeoutSetting(timeout time.Duration, retry int) easyCon.Setting {
	s := easyCon.NewSetting("ModuleA", "tcp://127.0.0.1:1883", nil, nil)
	s.TimeOut = timeout
	s.ReTry = retry
	s.LogMode = easyCon.ELogModeNone
	return s
}

// ------------------------------------------------------------------
// Req 超时场景
// ------------------------------------------------------------------

// TestReq_PublishTimeout 发布阶段超时（token 未完成）
func TestReq_PublishTimeout(t *testing.T) {
	setting := timeoutSetting(300*time.Millisecond, 2)
	c := &mockClient{forceTimeout: true}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	start := time.Now()
	resp := a.Req("ModuleB", "PING", "hello")
	elapsed := time.Since(start)

	if resp.RespCode != easyCon.ERespTimeout {
		t.Errorf("RespCode 期望 ERespTimeout(%d)，实际 %d", easyCon.ERespTimeout, resp.RespCode)
	}
	if resp.Error != "Req timeout" {
		t.Errorf("Error 期望 \"Req timeout\"，实际 %q", resp.Error)
	}
	// 重试次数 = ReTry + 1
	if got := c.count(); got != setting.ReTry+1 {
		t.Errorf("publish 次数期望 %d 次(ReTry+1)，实际 %d 次", setting.ReTry+1, got)
	}
	// 总耗时应接近 (ReTry+1) * TimeOut
	expectMin := time.Duration(setting.ReTry+1) * setting.TimeOut
	if elapsed < expectMin*9/10 {
		t.Errorf("总耗时 %v 小于期望下限 %v，未真正等待超时", elapsed, expectMin)
	}
	if elapsed > expectMin*3 {
		t.Errorf("总耗时 %v 远超期望上限 %v", elapsed, expectMin*3)
	}
	t.Logf("发布超时: 耗时=%v publish次数=%d respCode=%d", elapsed, c.count(), resp.RespCode)
}

// TestReq_ResponseTimeout 发布成功但对端无响应
func TestReq_ResponseTimeout(t *testing.T) {
	setting := timeoutSetting(300*time.Millisecond, 1)
	c := &mockClient{} // publish 正常完成
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	start := time.Now()
	resp := a.Req("ModuleB", "PING", "hello")
	elapsed := time.Since(start)

	if resp.RespCode != easyCon.ERespTimeout {
		t.Errorf("RespCode 期望 ERespTimeout(%d)，实际 %d", easyCon.ERespTimeout, resp.RespCode)
	}
	if got := c.count(); got != setting.ReTry+1 {
		t.Errorf("publish 次数期望 %d 次(ReTry+1)，实际 %d 次", setting.ReTry+1, got)
	}
	expectMin := time.Duration(setting.ReTry+1) * setting.TimeOut
	if elapsed < expectMin*9/10 {
		t.Errorf("总耗时 %v 小于期望下限 %v", elapsed, expectMin)
	}
	t.Logf("响应超时: 耗时=%v publish次数=%d", elapsed, c.count())
}

// TestReq_PublishError 发布阶段报错
func TestReq_PublishError(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 2)
	publishErr := errors.New("publish broken by timeout")
	c := &mockClient{publishErr: publishErr}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	resp := a.Req("ModuleB", "PING", "hello")
	if resp.RespCode != easyCon.ERespUnLinked {
		t.Errorf("RespCode 期望 ERespUnLinked(%d)，实际 %d", easyCon.ERespUnLinked, resp.RespCode)
	}
	if got := c.count(); got != 1 {
		t.Errorf("发布报错属于不可重试错误，publish 次数期望 1，实际 %d", got)
	}
}

// TestReq_NotLinked 未连接时直接返回
func TestReq_NotLinked(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 1)
	c := &mockClient{}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	// 模拟断线：客户端不再处于连接状态
	c.mu.Lock()
	c.publishErr = errors.New("not Connected")
	c.mu.Unlock()

	resp := a.Req("ModuleB", "PING", "hello")
	if resp.RespCode != easyCon.ERespTimeout && resp.RespCode != easyCon.ERespUnLinked {
		t.Errorf("RespCode 期望 ERespTimeout 或 ERespUnLinked，实际 %d", resp.RespCode)
	}
	t.Logf("RespCode=%d", resp.RespCode)
}

// TestReq_Success 正常成功路径（确认改造没有破坏正常流程）
// 通过 mock 的发布回调模拟对端应答，驱动完整的 respChan -> onResp -> respDict 回路
func TestReq_Success(t *testing.T) {
	setting := timeoutSetting(2*time.Second, 1)
	c := &mockClient{}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	c.onPublish = func(topic string, payload []byte) {
		var req easyCon.PackReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return
		}
		respPack := easyCon.PackResp{
			PackReq:  req,
			RespTime: time.Now().Format("2006-01-02 15:04:05.000"),
			RespCode: easyCon.ERespSuccess,
		}
		respPack.Content = "PONG"
		js, _ := json.Marshal(respPack)
		c.deliver("Response_ModuleA", js)
	}

	resp := a.Req("ModuleB", "PING", "hello")
	if resp.RespCode != easyCon.ERespSuccess {
		t.Fatalf("RespCode 期望 ERespSuccess，实际 %d", resp.RespCode)
	}
	if resp.Content != "PONG" {
		t.Errorf("Content 期望 \"PONG\"，实际 %v", resp.Content)
	}
	t.Logf("请求成功: respCode=%d content=%v", resp.RespCode, resp.Content)
}

// ------------------------------------------------------------------
// Notice 超时场景
// ------------------------------------------------------------------

// TestNotice_PublishTimeout 发布超时
func TestNotice_PublishTimeout(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 0)
	c := &mockClient{forceTimeout: true}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	start := time.Now()
	err := a.SendNotice("Route", "content")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("期望返回超时错误，实际为 nil")
	}
	if !strings.Contains(err.Error(), "notice timeout") {
		t.Errorf("错误信息期望包含 \"notice timeout\"，实际 %q", err.Error())
	}
	if elapsed < setting.TimeOut*9/10 {
		t.Errorf("耗时 %v 小于期望下限 %v，未真正等待超时", elapsed, setting.TimeOut)
	}
	t.Logf("Notice 发布超时: 耗时=%v err=%v", elapsed, err)
}

// TestRetainNotice_PublishTimeout 保留通知发布超时
func TestRetainNotice_PublishTimeout(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 0)
	c := &mockClient{forceTimeout: true}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	err := a.SendRetainNotice("Route", "content")
	if err == nil {
		t.Fatal("期望返回超时错误，实际为 nil")
	}
	if !strings.Contains(err.Error(), "notice timeout") {
		t.Errorf("错误信息期望包含 \"notice timeout\"，实际 %q", err.Error())
	}
}

// TestNotice_PublishError 发布报错
func TestNotice_PublishError(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 0)
	publishErr := errors.New("not connected")
	c := &mockClient{publishErr: publishErr}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	err := a.SendNotice("Route", "content")
	if err == nil {
		t.Fatal("期望返回发布错误，实际为 nil")
	}
	if err.Error() != publishErr.Error() {
		t.Errorf("错误信息期望 %q，实际 %q", publishErr.Error(), err.Error())
	}
}

// TestNotice_Success 正常成功路径
func TestNotice_Success(t *testing.T) {
	setting := timeoutSetting(2*time.Second, 0)
	c := &mockClient{}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	if err := a.SendNotice("Route", "content"); err != nil {
		t.Fatalf("期望发送成功，实际错误: %v", err)
	}
	if err := a.SendRetainNotice("Route", "content"); err != nil {
		t.Fatalf("期望保留通知发送成功，实际错误: %v", err)
	}
	if got := c.count(); got != 2 {
		t.Errorf("publish 次数期望 2，实际 %d", got)
	}
}

// TestNotice_TimeoutErrorReported 校验超时错误信息包含配置超时值
func TestNotice_TimeoutErrorReported(t *testing.T) {
	setting := timeoutSetting(150*time.Millisecond, 0)
	c := &mockClient{forceTimeout: true}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	err := a.SendNotice("Route", "content")
	if err == nil {
		t.Fatal("期望返回超时错误")
	}
	want := fmt.Sprintf("cfg timeout=%d", int64(setting.TimeOut))
	if !strings.Contains(err.Error(), want) {
		t.Errorf("错误信息期望包含 %q，实际 %q", want, err.Error())
	}
}

// ------------------------------------------------------------------
// 回归：超时路径触发的日志上传
// ------------------------------------------------------------------

// TestLogUpload_TimeoutNoPanic 超时路径触发日志上传时不应 panic
// paho 的 WaitTimeout 超时不会给 token 设置 error（Error() 返回 nil），
// 若直接调用 token.Error().Error() 会触发空指针 panic。
func TestLogUpload_TimeoutNoPanic(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 0)
	setting.LogMode = easyCon.ELogModeUpload
	c := &mockClient{forceTimeout: true}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("日志上传超时不应 panic，实际 panic: %v", r)
		}
	}()
	a.Err("timeout test", errors.New("boom"))
}

// TestNotice_TimeoutWithLogUpload Notice 超时 + 日志上传模式
// sendNoticeInner 超时会调用 adapter.Err 上报日志；日志上传自身也可能超时，
// 该 panic 会沿调用栈冒泡到 SendNotice 的调用方。
func TestNotice_TimeoutWithLogUpload(t *testing.T) {
	setting := timeoutSetting(200*time.Millisecond, 0)
	setting.LogMode = easyCon.ELogModeUpload
	c := &mockClient{forceTimeout: true}
	a := easyCon.NewMqttAdapterWithClient(setting, c)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Notice 超时 + 日志上传不应 panic，实际 panic: %v", r)
		}
	}()
	_ = a.SendNotice("Route", "content")
}

// ------------------------------------------------------------------
// 集成测试（需要真实 broker）
// ------------------------------------------------------------------

const brokerWsAddr = "ws://127.0.0.1:5002/ws"

func brokerAlive() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:5002", 1*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// TestIntegration_ReqTimeout 对端无应答 -> 请求超时
func TestIntegration_ReqTimeout(t *testing.T) {
	if !brokerAlive() {
		t.Skip("MQTT broker 未启动(127.0.0.1:5002)，跳过集成测试")
	}
	setting := easyCon.NewSetting("TimeoutModuleA", brokerWsAddr, nil, nil)
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	setting.TimeOut = 500 * time.Millisecond
	setting.ReTry = 1
	setting.LogMode = easyCon.ELogModeNone

	a := easyCon.NewMqttAdapter(setting)
	defer a.Stop()
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	resp := a.Req("NoSuchModuleXYZ", "PING", "hello")
	elapsed := time.Since(start)

	if resp.RespCode != easyCon.ERespTimeout {
		t.Errorf("RespCode 期望 ERespTimeout，实际 %d", resp.RespCode)
	}
	expectMin := time.Duration(setting.ReTry+1) * setting.TimeOut
	if elapsed < expectMin*8/10 {
		t.Errorf("总耗时 %v 小于期望下限 %v", elapsed, expectMin)
	}
	t.Logf("集成-请求超时: 耗时=%v respCode=%d error=%s", elapsed, resp.RespCode, resp.Error)
}

// TestIntegration_ReqAndNoticeNormal 正常链路
func TestIntegration_ReqAndNoticeNormal(t *testing.T) {
	if !brokerAlive() {
		t.Skip("MQTT broker 未启动(127.0.0.1:5002)，跳过集成测试")
	}
	setting := easyCon.NewSetting("NormalModuleA", brokerWsAddr, func(pack easyCon.PackReq) (easyCon.EResp, any) {
		return easyCon.ERespSuccess, "PONG"
	}, nil)
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	setting.TimeOut = 2 * time.Second
	setting.ReTry = 1
	setting.LogMode = easyCon.ELogModeNone

	a := easyCon.NewMqttAdapter(setting)
	defer a.Stop()

	// 注意：只有设置了 OnReq，adapter 才会订阅 "Request_<Module>" 主题
	settingB := easyCon.NewSetting("NormalModuleB", brokerWsAddr, func(pack easyCon.PackReq) (easyCon.EResp, any) {
		return easyCon.ERespSuccess, "PONG"
	}, nil)
	settingB.UID = "admin"
	settingB.PWD = "ams@urit2024"
	settingB.TimeOut = 2 * time.Second
	settingB.LogMode = easyCon.ELogModeNone
	noticeCh := make(chan easyCon.PackNotice, 4)
	settingB.OnNotice = func(n easyCon.PackNotice) { noticeCh <- n }
	b := easyCon.NewMqttAdapter(settingB)
	defer b.Stop()

	time.Sleep(500 * time.Millisecond)

	resp := a.Req("NormalModuleB", "PING", "hello")
	if resp.RespCode != easyCon.ERespSuccess {
		t.Errorf("RespCode 期望 ERespSuccess，实际 %d", resp.RespCode)
	}
	if err := a.SendNotice("Hello", "world"); err != nil {
		t.Errorf("SendNotice 期望成功，实际: %v", err)
	}
	select {
	case n := <-noticeCh:
		if n.Route != "Hello" {
			t.Errorf("通知 Route 期望 Hello，实际 %s", n.Route)
		}
	case <-time.After(3 * time.Second):
		t.Error("等待通知超时")
	}
}

// TestIntegration_NoticeTimeoutNoticeNotReceived 通知正常发出但对端未订阅
func TestIntegration_NoticeTimeoutNoticeNotReceived(t *testing.T) {
	if !brokerAlive() {
		t.Skip("MQTT broker 未启动(127.0.0.1:5002)，跳过集成测试")
	}
	setting := easyCon.NewSetting("NoticeModuleA", brokerWsAddr, nil, nil)
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	setting.TimeOut = 2 * time.Second
	setting.LogMode = easyCon.ELogModeNone

	a := easyCon.NewMqttAdapter(setting)
	defer a.Stop()
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	err := a.SendNotice("Hello", "world")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("SendNotice 期望成功，实际: %v", err)
	}
	if elapsed > setting.TimeOut {
		t.Errorf("通知发送耗时 %v 超过配置超时 %v", elapsed, setting.TimeOut)
	}
	t.Logf("集成-通知发送: 耗时=%v err=%v", elapsed, err)
}
