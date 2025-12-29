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
)

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
	//testInner(t)
	//time.Sleep(time.Second * 1)
	testProxy(t)
}

// ✅ 全局错误日志文件
var errorLogFile *os.File
var errorLogMutex sync.Mutex

func logError(format string, args ...interface{}) {
	errorLogMutex.Lock()
	defer errorLogMutex.Unlock()
	if errorLogFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		fmt.Fprintf(errorLogFile, "[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	}
}
func testProxy(t *testing.T) {
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
	if atomic.LoadInt64(&respDetectedCount) != testCount*2 {
		t.Fatal("响应检测未通过")
	}
	if atomic.LoadInt64(&logSuccessCount) != testCount*2 {
		t.Fatal("日志检测未通过")
	}
	_ = mc.CleanRetainNotice("RetainNotice")
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

	if atomic.LoadInt64(&reqSuccessCount) != testCount {
		t.Fatal("请求未通过", atomic.LoadInt64(&reqSuccessCount), testCount)
	}
	if atomic.LoadInt64(&noticeSuccessCount) != testCount {
		t.Fatal("通知测试未通过")
	}
	if atomic.LoadInt64(&retainNoticeSuccessCount) != testCount {
		t.Fatal("Retain通知测试未通过")
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

func onReqRecMonitor(pack easyCon.PackReq) (easyCon.EResp, any) {
	currentCount := atomic.AddInt64(&reqDetectedCount, 1)
	// 只在达到间隔或最后一次时打印
	if currentCount%printInterval == 0 || currentCount == testCount {
		fmt.Printf("[%s]:Detected========== REQ From %s To %s  %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), pack.From, pack.To, pack.Id, pack.Content, currentCount, testCount)
	}
	return easyCon.ERespBypass, nil
}

func onRespRec(pack easyCon.PackResp) {
	currentCount := atomic.AddInt64(&respDetectedCount, 1)
	// 只在达到间隔或最后一次时打印
	if currentCount%printInterval == 0 || currentCount == testCount {
		fmt.Printf("[%s]:Detected========== RESP From %s To %s  %d %s [%d/%d] \r\n", time.Now().Format("15:04:05.000"), pack.To, pack.From, pack.Id, pack.Content, currentCount, testCount)
	}
}
func onStatusChangedA(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module A status changed to ", statusStr)
}

func onRetainNotice(notice easyCon.PackNotice) {
	currentCount := atomic.AddInt64(&retainNoticeSuccessCount, 1)
	// 只在达到间隔或最后一次时打印
	if currentCount%printInterval == 0 || currentCount == testCount {
		fmt.Printf("[%s]:Detected========== RetainNotice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From, currentCount, testCount)
	}
}

func onNoticeRec(notice easyCon.PackNotice) {
	if notice.Content == NoticeContent {
		atomic.AddInt64(&noticeSuccessCount, 1)
	}
	currentCount := atomic.LoadInt64(&noticeSuccessCount)
	// 只在达到间隔或最后一次时打印
	if currentCount%printInterval == 0 || currentCount == testCount {
		fmt.Printf("[%s]:Detected========== Notice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From, currentCount, testCount)
	}
}

func onLogRec(log easyCon.PackLog) {
	if log.Content == LogContent {
		atomic.AddInt64(&logSuccessCount, 1)
	}
	currentCount := atomic.LoadInt64(&logSuccessCount)
	// 只在达到间隔或最后一次时打印
	if currentCount%printInterval == 0 || currentCount == testCount {
		fmt.Printf("[%s]:Detected========== Log %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content, log.From, currentCount, testCount)
	}
}
func onStatusChangedB(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module B status changed to ", statusStr)
}

func onReqRec(_ easyCon.PackReq) (easyCon.EResp, any) {
	return easyCon.ERespSuccess, RespContent
}
