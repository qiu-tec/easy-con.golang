/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 16:22
 */

package cgotest

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	testCount                int64 = 1000000
	reqSuccessCount          int64 = 0
	reqDetectedCount         int64 = 0
	respDetectedCount        int64 = 0
	noticeSuccessCount       int64 = 0
	retainNoticeSuccessCount int64 = 0
	logSuccessCount          int64 = 0
	consistencyCheckCount    int64 = 0 // 一致性检测计数
)

// PackSnapshot 用于记录发送的pack快照
type PackSnapshot struct {
	PackType string // "REQ", "RESP", "NOTICE", "LOG", "RETAIN_NOTICE"
	Id       uint64
	From     string
	To       string
	Route    string
	Content  []byte
}

// sentPackets 存储发送的pack快照 (key: packType+packId, value: PackSnapshot)
// 使用复合键避免不同类型包的ID冲突
var sentPackets sync.Map

// makeSnapshotKey 创建快照键（类型+ID）
func makeSnapshotKey(packType string, id uint64) string {
	return packType + "_" + fmt.Sprintf("%d", id)
}

const (
	LogContent          = "I am log"
	ReqContent          = "Ping"
	RespContent         = "Pong"
	NoticeContent       = "I am notice"
	RetainNoticeContent = "I am retain notice"
	SleepTime           = time.Millisecond
	printInterval       = 100 // 每100次打印一次进度
)

func Test(t *testing.T) {
	testInner(t)
	// testProxy 需要 MQTT broker 连接，暂时跳过
	// time.Sleep(time.Second * 1)
	// testProxy(t)
}

// ✅ 全局错误日志文件
var errorLogFile *os.File
var errorLogMutex sync.Mutex

func logError(format string, args ...interface{}) {
	errorLogMutex.Lock()
	defer errorLogMutex.Unlock()
	if errorLogFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		_, _ = fmt.Fprintf(errorLogFile, "[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	}
}
func testProxy(t *testing.T) {
	// ✅ 清空快照存储，避免与testInner的数据冲突
	sentPackets = sync.Map{}

	// ✅ 打开错误日志文件
	var err error
	errorLogFile, err = os.OpenFile("error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("无法打开error.log文件: %v", err)
	}
	defer errorLogFile.Close()
	logError("========== 测试开始 ==========")

	broker := easyCon.NewCgoBroker()
	setting := easyCon.CoreSetting{
		Module:            "ModuleA",
		TimeOut:           time.Second * 3,
		ReTry:             3,
		LogMode:           easyCon.ELogModeUpload,
		PreFix:            "",
		ChannelBufferSize: 100,
		ConnectRetryDelay: time.Second,
		IsWaitLink:        false,
		IsSync:            false,
	}
	cba := easyCon.AdapterCallBack{
		OnReqRec:        onReqRec,
		OnStatusChanged: onStatusChangedA,
		//OnNoticeRec:       onNoticeRecA,
		//OnRetainNoticeRec: onRetainNoticeA,
		//OnLogRec:          onLogRecA,
	}
	moduleA, fa := easyCon.NewCgoAdapter(setting, cba, broker.Publish)
	fmt.Println("moduleA:", moduleA)
	broker.RegClient(setting.Module, fa)

	sc := easyCon.NewDefaultMqttSetting("ModuleC", "ws://127.0.0.1:5002/ws")
	//sc := easyCon.NewDefaultMqttSetting("ModuleC", "ws://172.19.14.199:8083/mqtt")
	sc.LogMode = easyCon.ELogModeUpload
	cbc := easyCon.AdapterCallBack{
		OnReqRec:        onReqRec,
		OnStatusChanged: onStatusChangedC,
	}
	mc := easyCon.NewMqttAdapter(sc, cbc)
	fmt.Println("moduleC:", mc)
	sc.Module = "Monitor"

	cbMonitor := easyCon.AdapterCallBack{
		OnReqRec:          onReqRecMonitor,
		OnRespRec:         onRespRec,
		OnLogRec:          onLogRec,
		OnNoticeRec:       onNoticeRec,
		OnRetainNoticeRec: onRetainNotice,
	}
	_ = easyCon.NewMqttMonitor(sc, cbMonitor)

	ps := easyCon.MqttProxySetting{
		Addr: "ws://127.0.0.1:5002/ws",
		//Addr: "ws://172.19.14.199:8083/mqtt",
		//ReTry:   1,
		PreFix:  "",
		TimeOut: time.Second * 3,
		Module:  "Proxy",
	}
	_, fp := easyCon.NewCgoMqttProxy(ps, broker.Publish)
	broker.RegClient("Proxy", fp)

	// ✅ 等待所有模块连接成功
	time.Sleep(SleepTime)
	_ = mc.CleanRetainNotice("RetainNotice")

	// ✅ 等待 ModuleC 和 Proxy 完全连接并订阅完成
	fmt.Println("等待 MQTT 连接建立...")
	time.Sleep(time.Second * 2) // 给 MQTT 连接足够的时间建立

	// ✅ 发送测试请求验证连接
	fmt.Println("验证连接状态...")
	testResp := moduleA.Req("ModuleC", "Ping", "Test")
	if testResp.RespCode != easyCon.ERespSuccess {
		fmt.Printf("⚠ 连接测试失败，继续等待... (Code: %d)\n", testResp.RespCode)
		time.Sleep(time.Second * 3)
	} else {
		fmt.Println("✓ 连接验证成功")
	}

	// ✅ 发送几个预热请求
	fmt.Println("发送预热请求...")
	for i := int64(0); i < 10; i++ {
		moduleA.Req("ModuleC", "WarmUp", "Test")
		_ = moduleA.SendNotice("Notice", NoticeContent)
	}
	time.Sleep(time.Millisecond * 500) // 等待预热请求处理完成

	// ✅ 重置计数器
	atomic.StoreInt64(&reqSuccessCount, 0)
	atomic.StoreInt64(&reqDetectedCount, 0)
	atomic.StoreInt64(&respDetectedCount, 0)
	atomic.StoreInt64(&noticeSuccessCount, 0)
	atomic.StoreInt64(&retainNoticeSuccessCount, 0)
	atomic.StoreInt64(&logSuccessCount, 0)
	atomic.StoreInt64(&consistencyCheckCount, 0)
	sentPackets = sync.Map{} // 清空快照存储

	time.Sleep(SleepTime)

	startTime := time.Now()
	for i := int64(0); i < testCount; i++ {
		resp := moduleA.Req("ModuleC", ReqContent, ReqContent)
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
			logError("请求失败 ID=%d Code=%d Error=%s Route=%s", resp.Id, resp.RespCode, resp.Error, resp.Route)
		} else {
			currentCount := atomic.AddInt64(&reqSuccessCount, 1)
			// 只在达到间隔时打印
			if currentCount%printInterval == 0 || currentCount == testCount*2 {
				elapsed := time.Since(startTime).Milliseconds()
				avgTime := elapsed / currentCount
				fmt.Printf("[%s]: %d %s [%d/%d] - 总耗时: %dms, 平均: %dms/次\r\n",
					time.Now().Format("15:04:05.000"), resp.Id, "请求成功", currentCount, testCount*2, elapsed, avgTime)
			}
		}

		err := moduleA.SendNotice("Notice", NoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
			logError("正向通知发送失败 err=%v", err)
		}

		err = moduleA.SendRetainNotice("RetainNotice", RetainNoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain通知发送失败")
			logError("正向Retain通知发送失败 err=%v", err)
		}

		moduleA.Err(LogContent, fmt.Errorf("I am Error"))

		//反向
		resp = mc.Req("ModuleA", ReqContent, ReqContent)
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "反向请求失败")
			logError("反向请求失败 ID=%d Code=%d Error=%s Route=%s", resp.Id, resp.RespCode, resp.Error, resp.Route)
		} else {
			currentCount := atomic.AddInt64(&reqSuccessCount, 1)
			// 只在达到间隔时打印
			if currentCount%printInterval == 0 || currentCount == testCount*2 {
				elapsed := time.Since(startTime).Milliseconds()
				avgTime := elapsed / currentCount
				fmt.Printf("[%s]: %d %s [%d/%d] - 总耗时: %dms, 平均: %dms/次\r\n",
					time.Now().Format("15:04:05.000"), resp.Id, "反向请求检测成功", currentCount, testCount*2, elapsed, avgTime)
			}
		}

		err = mc.SendNotice("Notice", NoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "反向通知发送失败")
			logError("反向通知发送失败 err=%v", err)
		}

		err = mc.SendRetainNotice("RetainNotice", RetainNoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "反向Retain通知发送失败")
			logError("反向Retain通知发送失败 err=%v", err)
		}

		mc.Err(LogContent, fmt.Errorf("I am Error"))

	}
	fmt.Printf("testProxy 测试完成! 总耗时: %dms, 平均: %dms/次\n", time.Since(startTime).Milliseconds(), time.Since(startTime).Milliseconds()/testCount*2)
	logError("========== 测试完成 ==========")
	time.Sleep(time.Second)
	if atomic.LoadInt64(&reqSuccessCount) != testCount*2 {
		t.Fatal("请求未通过", atomic.LoadInt64(&reqSuccessCount), testCount*2)
	}
	if atomic.LoadInt64(&noticeSuccessCount) != testCount*2 {
		t.Fatal("通知测试未通过")
	}
	if atomic.LoadInt64(&retainNoticeSuccessCount) < testCount*2 {
		t.Fatal("Retain通知测试未通过", atomic.LoadInt64(&retainNoticeSuccessCount), testCount*2)
	}
	if atomic.LoadInt64(&reqDetectedCount) != testCount*2 {
		t.Fatal("请求检测未通过")
	}
	// 一致性检测报告（在断言前打印）
	consistencyChecks := atomic.LoadInt64(&consistencyCheckCount)
	fmt.Printf("========== testProxy 一致性检测报告 ==========\n")
	fmt.Printf("一致性验证通过: %d 次\n", consistencyChecks)
	fmt.Printf("预期验证次数: ~%d 次 (REQ+RESP+NOTICE+LOG+RETAIN_NOTICE)\n", testCount*5)
	if consistencyChecks > 0 {
		fmt.Printf("✓ 包一致性检测正常工作\n")
	} else {
		fmt.Printf("⚠ 警告: 没有一致性检测通过（可能需要检查快照记录逻辑）\n")
	}
	fmt.Printf("==========================================\n")
	logError("========== testProxy 一致性检测: %d 次通过 ==========", consistencyChecks)

	if atomic.LoadInt64(&reqSuccessCount) != testCount*2 {
		t.Fatal("请求未通过", atomic.LoadInt64(&reqSuccessCount), testCount*2)
	}
	if atomic.LoadInt64(&noticeSuccessCount) != testCount*2 {
		t.Fatal("通知测试未通过")
	}
	if atomic.LoadInt64(&retainNoticeSuccessCount) < testCount*2 {
		t.Fatal("Retain通知测试未通过", atomic.LoadInt64(&retainNoticeSuccessCount), testCount*2)
	}
	if atomic.LoadInt64(&reqDetectedCount) != testCount*2 {
		t.Fatal("请求检测未通过")
	}
	if atomic.LoadInt64(&respDetectedCount) != testCount*2 {
		t.Fatal("响应检测未通过")
	}
	if atomic.LoadInt64(&logSuccessCount) != testCount*2 {
		t.Fatal("日志检测未通过")
	}
	_ = mc.CleanRetainNotice("RetainNotice")

	// 停止适配器，清理 goroutine
	moduleA.Stop()
	mc.Stop()
	fmt.Println("testProxy: 已停止所有适配器")
}

func onStatusChangedC(status easyCon.EStatus) {
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module C status changed to ", easyCon.GetStatusName(status))
}

func testInner(t *testing.T) {
	broker := easyCon.NewCgoBroker()
	setting := easyCon.CoreSetting{
		Module:            "ModuleA",
		TimeOut:           time.Second * 3,
		ReTry:             1,
		LogMode:           easyCon.ELogModeUpload,
		PreFix:            "",
		ChannelBufferSize: 100,
		ConnectRetryDelay: time.Second,
		IsWaitLink:        false,
		IsSync:            false,
	}
	cba := easyCon.AdapterCallBack{
		OnReqRec:        onReqRec,
		OnStatusChanged: onStatusChangedA,
	}
	moduleA, fa := easyCon.NewCgoAdapter(setting, cba, broker.Publish)
	broker.RegClient(setting.Module, fa)
	setting.Module = "ModuleB"
	cbb := easyCon.AdapterCallBack{
		OnReqRec:        onReqRec,
		OnStatusChanged: onStatusChangedB,
		//OnLogRec:          onLogRecB,
		//OnNoticeRec:       onNoticeRecB,
		//OnRetainNoticeRec: onRetainNoticeB,
		//OnLinked:          onLinkedB,
		//OnExiting:         onExitingB,
		//OnGetVersion:      onGetVersionB,
	}
	_, fb := easyCon.NewCgoAdapter(setting, cbb, broker.Publish)
	broker.RegClient(setting.Module, fb)
	setting.Module = "Monitor"
	cbMonitor := easyCon.AdapterCallBack{
		OnReqRec:          onReqRecMonitor,
		OnRespRec:         onRespRec,
		OnLogRec:          onLogRec,
		OnNoticeRec:       onNoticeRec,
		OnRetainNoticeRec: onRetainNotice,
	}
	_, fm := easyCon.NewCGoMonitor(setting, cbMonitor, broker.Publish)
	broker.RegClient(setting.Module, fm)

	time.Sleep(SleepTime)

	atomic.StoreInt64(&reqSuccessCount, 0)
	atomic.StoreInt64(&reqDetectedCount, 0)
	atomic.StoreInt64(&respDetectedCount, 0)
	atomic.StoreInt64(&noticeSuccessCount, 0)
	atomic.StoreInt64(&retainNoticeSuccessCount, 0)
	atomic.StoreInt64(&logSuccessCount, 0)
	atomic.StoreInt64(&consistencyCheckCount, 0)
	sentPackets = sync.Map{} // 清空快照存储

	time.Sleep(SleepTime)
	startTime := time.Now()
	for i := int64(0); i < testCount; i++ {
		resp := moduleA.Req("ModuleB", ReqContent, ReqContent)
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
		} else {
			currentCount := atomic.AddInt64(&reqSuccessCount, 1)
			// 只在达到间隔或最后一次时打印
			if currentCount%printInterval == 0 || currentCount == testCount {
				elapsed := time.Since(startTime).Milliseconds()
				avgTime := elapsed / currentCount
				fmt.Printf("[%s]: %d %s [%d/%d] - 总耗时: %dms, 平均: %dms/次\r\n",
					time.Now().Format("15:04:05.000"), resp.Id, "请求成功", currentCount, testCount, elapsed, avgTime)
			}
		}

		err := moduleA.SendNotice("Notice", NoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		}

		err = moduleA.SendRetainNotice("RetainNotice", RetainNoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain通知发送失败")
		}

		moduleA.Err(LogContent, fmt.Errorf("I am Error"))
	}
	fmt.Printf("测试完成! 总耗时: %dms, 平均: %dms/次\n", time.Since(startTime).Milliseconds(), time.Since(startTime).Milliseconds()/testCount)

	time.Sleep(time.Second)

	// 一致性检测报告（在断言前打印）
	consistencyChecks := atomic.LoadInt64(&consistencyCheckCount)
	fmt.Printf("========== testInner 一致性检测报告 ==========\n")
	fmt.Printf("一致性验证通过: %d 次\n", consistencyChecks)
	fmt.Printf("预期验证次数: ~%d 次 (REQ+RESP+NOTICE+LOG+RETAIN_NOTICE)\n", testCount*4)
	if consistencyChecks > 0 {
		fmt.Printf("✓ 包一致性检测正常工作\n")
	} else {
		fmt.Printf("⚠ 警告: 没有一致性检测通过（可能需要检查快照记录逻辑）\n")
	}
	fmt.Printf("==========================================\n")

	// 打印各计数器值
	fmt.Printf("计数器状态: reqSuccess=%d, noticeSuccess=%d, retainNoticeSuccess=%d, logSuccess=%d\n",
		atomic.LoadInt64(&reqSuccessCount),
		atomic.LoadInt64(&noticeSuccessCount),
		atomic.LoadInt64(&retainNoticeSuccessCount),
		atomic.LoadInt64(&logSuccessCount))

	if atomic.LoadInt64(&reqSuccessCount) != testCount {
		t.Fatal("请求未通过", atomic.LoadInt64(&reqSuccessCount), testCount)
	}
	if atomic.LoadInt64(&noticeSuccessCount) != testCount {
		t.Fatal("通知测试未通过", atomic.LoadInt64(&noticeSuccessCount), testCount)
	}
	if atomic.LoadInt64(&retainNoticeSuccessCount) != testCount {
		t.Fatal("Retain通知测试未通过", atomic.LoadInt64(&retainNoticeSuccessCount), testCount)
	}
	if atomic.LoadInt64(&reqDetectedCount) != testCount {
		t.Fatal("请求检测未通过")
	}
	if atomic.LoadInt64(&respDetectedCount) != testCount {
		t.Fatal("响应检测未通过")
	}
	if atomic.LoadInt64(&logSuccessCount) != testCount {
		t.Fatal("日志检测未通过")
	}
}

func onReqRecMonitor(pack easyCon.PackReq) (easyCon.EResp, []byte) {
	currentCount := atomic.AddInt64(&reqDetectedCount, 1)
	// 只在达到间隔或最后一次时打印
	if (currentCount%printInterval == 0 || currentCount == testCount) && currentCount > 0 {
		fmt.Printf("[%s]:Detected========== REQ From %s To %s  %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), pack.From, pack.To, pack.Id, string(pack.Content), currentCount, testCount)
	}
	// 记录发送的请求快照（使用复合键避免ID冲突）
	snapshot := PackSnapshot{
		PackType: "REQ",
		Id:       pack.Id,
		From:     pack.From,
		To:       pack.To,
		Route:    pack.Route,
		Content:  pack.Content,
	}
	sentPackets.Store(makeSnapshotKey("REQ", pack.Id), snapshot)
	return easyCon.ERespBypass, nil
}

func onRespRec(pack easyCon.PackResp) {
	currentCount := atomic.AddInt64(&respDetectedCount, 1)
	// 验证包一致性
	verifyPackConsistency(pack.Id, "RESP", pack.From, pack.To, pack.Route, pack.Content)
	// 只在达到间隔或最后一次时打印
	if (currentCount%printInterval == 0 || currentCount == testCount) && currentCount > 0 {
		fmt.Printf("[%s]:Detected========== RESP From %s To %s  %d %s [%d/%d] \r\n", time.Now().Format("15:04:05.000"), pack.To, pack.From, pack.Id, string(pack.Content), currentCount, testCount)
	}
}
func onStatusChangedA(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module A status changed to ", statusStr)
}

func onRetainNotice(notice easyCon.PackNotice) {
	currentCount := atomic.AddInt64(&retainNoticeSuccessCount, 1)
	// 验证包一致性
	verifyPackConsistency(notice.Id, "RETAIN_NOTICE", notice.From, "", notice.Route, notice.Content)
	// 只在达到间隔或最后一次时打印
	if (currentCount%printInterval == 0 || currentCount == testCount) && currentCount > 0 {
		fmt.Printf("[%s]:Detected========== RetainNotice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, string(notice.Content), notice.From, currentCount, testCount)
	}
}

func onNoticeRec(notice easyCon.PackNotice) {
	if string(notice.Content) == NoticeContent {
		currentCount := atomic.AddInt64(&noticeSuccessCount, 1)
		// 验证包一致性
		verifyPackConsistency(notice.Id, "NOTICE", notice.From, "", notice.Route, notice.Content)
		// 只在达到间隔或最后一次时打印，且count > 0
		if (currentCount%printInterval == 0 || currentCount == testCount) && currentCount > 0 {
			fmt.Printf("[%s]:Detected========== Notice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, string(notice.Content), notice.From, currentCount, testCount)
		}
	}
}

func onLogRec(log easyCon.PackLog) {
	if string(log.Content) == LogContent {
		currentCount := atomic.AddInt64(&logSuccessCount, 1)
		// 验证包一致性 (LOG包没有To和Route)
		verifyPackConsistency(log.Id, "LOG", log.From, "", "", []byte(log.Content))
		// 只在达到间隔或最后一次时打印
		if (currentCount%printInterval == 0 || currentCount == testCount) && currentCount > 0 {
			fmt.Printf("[%s]:Detected========== Log %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content, log.From, currentCount, testCount)
		}
	}
}
func onStatusChangedB(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module B status changed to ", statusStr)
}

func onReqRec(_ easyCon.PackReq) (easyCon.EResp, []byte) {
	return easyCon.ERespSuccess, []byte(RespContent)
}

// verifyPackConsistency 验证接收的pack与发送的快照是否一致
func verifyPackConsistency(id uint64, packType string, from, to, route string, content []byte) bool {
	// 使用复合键查找快照（查找对应的请求快照）
	lookupType := packType
	if packType == "RESP" {
		// RESP 需要查找对应的 REQ 快照
		lookupType = "REQ"
	}

	value, ok := sentPackets.Load(makeSnapshotKey(lookupType, id))

	if !ok {
		// 没有找到对应的发送快照
		// 对于 Notice/Log/RetainNotice，由于我们无法在发送前知道ID，
		// 这里只做基本的内容验证
		expectedContent := ""
		switch packType {
		case "NOTICE":
			expectedContent = NoticeContent
		case "LOG":
			expectedContent = LogContent
		case "RETAIN_NOTICE":
			expectedContent = RetainNoticeContent
		default:
			// 对于 REQ/RESP，没有快照说明有问题
			return false
		}

		if string(content) == expectedContent {
			atomic.AddInt64(&consistencyCheckCount, 1)
			return true
		}
		logError("内容验证失败 ID=%d Type=%s: 期望=%s, 实际=%s", id, packType, expectedContent, string(content))
		return false
	}

	snapshot := value.(PackSnapshot)

	// 对于RESP，需要验证是否与对应的REQ匹配
	// 对于其他类型，验证类型是否匹配
	if packType != "RESP" && snapshot.PackType != packType {
		logError("包类型不一致 ID=%d: 期望=%s, 实际=%s", id, snapshot.PackType, packType)
		return false
	}

	// 验证From字段
	hasError := false
	if snapshot.From != from {
		logError("From字段不一致 ID=%d: 期望=%s, 实际=%s", id, snapshot.From, from)
		hasError = true
	}

	// 验证To字段（如果是RESP类型，需要反向比较）
	expectedTo := snapshot.To
	if packType == "RESP" {
		// RESP的To应该是原请求的From
		expectedTo = snapshot.From
	}
	if expectedTo != to {
		logError("To字段不一致 ID=%d: 期望=%s, 实际=%s", id, expectedTo, to)
		hasError = true
	}

	// 验证Route字段
	if snapshot.Route != route {
		logError("Route字段不一致 ID=%d: 期望=%s, 实际=%s", id, snapshot.Route, route)
		hasError = true
	}

	// 验证Content
	if string(snapshot.Content) != string(content) {
		logError("Content不一致 ID=%d: 期望=%s, 实际=%s", id, string(snapshot.Content), string(content))
		hasError = true
	}

	if !hasError {
		atomic.AddInt64(&consistencyCheckCount, 1)
	}

	return !hasError
}
