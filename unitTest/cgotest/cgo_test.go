/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 16:22
 */

package cgotest

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"testing"
	"time"
)

var (
	testCount                = 1000
	reqSuccessCount          = 0
	reqDetectedCount         = 0
	respDetectedCount        = 0
	noticeSuccessCount       = 0
	retainNoticeSuccessCount = 0
	logSuccessCount          = 0
)

const (
	LogContent          = "I am log"
	ReqContent          = "Ping"
	RespContent         = "Pong"
	NoticeContent       = "I am notice"
	RetainNoticeContent = "I am retain notice"
	SleepTime           = time.Millisecond
)

func Test(t *testing.T) {
	testInner(t)
	//testProxy(t)
}
func testProxy(t *testing.T) {
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
		OnReqRec:          onReqRec,
		OnStatusChanged:   onStatusChangedA,
		OnNoticeRec:       onNoticeRecA,
		OnRetainNoticeRec: onRetainNoticeA,
		OnLogRec:          onLogRecA,
	}
	moduleA, fa := easyCon.NewCgoAdapter(setting, cba, broker.Publish)
	fmt.Println("moduleA:", moduleA)
	broker.RegClient(setting.Module, fa)

	sc := easyCon.NewDefaultMqttSetting("ModuleC", "ws://127.0.0.1:5002/ws")
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
		//ReTry:   1,
		PreFix:  "",
		TimeOut: time.Second * 3,
		Module:  "Proxy",
	}
	_, fp := easyCon.NewCgoMqttProxy(ps, broker.Publish)
	broker.RegClient("Proxy", fp)

	time.Sleep(SleepTime)

	reqSuccessCount = 0
	reqDetectedCount = 0
	respDetectedCount = 0
	noticeSuccessCount = 0
	retainNoticeSuccessCount = 0
	logSuccessCount = 0

	time.Sleep(SleepTime)
	for i := 0; i < testCount; i++ {
		resp := moduleA.Req("ModuleC", ReqContent, ReqContent)
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
		} else {
			reqSuccessCount++
			fmt.Printf("[%s]: %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求成功", reqSuccessCount, testCount)
		}
		time.Sleep(SleepTime)

		err := moduleA.SendNotice("Notice", NoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		}
		time.Sleep(SleepTime)

		err = moduleA.SendRetainNotice("RetainNotice", RetainNoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain通知发送失败")
		}
		time.Sleep(SleepTime)

		moduleA.Err(LogContent, fmt.Errorf("I am Error"))
		time.Sleep(SleepTime)
		//反向
		resp = mc.Req("ModuleA", ReqContent, ReqContent)
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "反向请求失败")
		} else {
			reqSuccessCount++
			fmt.Printf("[%s]: %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), resp.Id, "反向请求检测成功", reqSuccessCount, testCount)
		}
		time.Sleep(SleepTime)

		err = mc.SendNotice("Notice", NoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "反向通知发送失败")
		}
		time.Sleep(SleepTime)

		err = mc.SendRetainNotice("RetainNotice", RetainNoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "反向Retain通知发送失败")
		}
		time.Sleep(SleepTime)

		mc.Err(LogContent, fmt.Errorf("I am Error"))
		time.Sleep(SleepTime)

	}
	time.Sleep(time.Second)
	if reqSuccessCount != testCount*2 {
		t.Fatal("请求未通过")
	}
	if noticeSuccessCount != testCount*2 {
		t.Fatal("通知测试未通过")
	}
	if retainNoticeSuccessCount != testCount*2 {
		t.Fatal("Retain通知测试未通过")
	}
	if reqDetectedCount != testCount*2 {
		t.Fatal("请求检测未通过")
	}
	if respDetectedCount != testCount*2 {
		t.Fatal("响应检测未通过")
	}
	if logSuccessCount != testCount*2 {
		t.Fatal("日志检测未通过")
	}

}

func onLogRecA(log easyCon.PackLog) {
	fmt.Printf("[%s]:CGo Detected========== Log %d  %s From %s \r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content, log.From)
}

func onRetainNoticeA(notice easyCon.PackNotice) {
	fmt.Printf("[%s]:CGo Detected========== RetainNotice %d  %s From %s \r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From)
}

func onNoticeRecA(notice easyCon.PackNotice) {
	fmt.Printf("[%s]:CGo Detected========== Notice %d  %s From %s \r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From)
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

	reqSuccessCount = 0
	reqDetectedCount = 0
	respDetectedCount = 0
	noticeSuccessCount = 0
	retainNoticeSuccessCount = 0
	logSuccessCount = 0

	time.Sleep(SleepTime)
	for i := 0; i < testCount; i++ {
		resp := moduleA.Req("ModuleB", ReqContent, ReqContent)
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
		} else {
			reqSuccessCount++
			fmt.Printf("[%s]: %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求成功", reqSuccessCount, testCount)
		}
		time.Sleep(SleepTime)

		err := moduleA.SendNotice("Notice", NoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		}
		time.Sleep(SleepTime)

		err = moduleA.SendRetainNotice("RetainNotice", RetainNoticeContent)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain通知发送失败")
		}
		time.Sleep(SleepTime)

		moduleA.Err(LogContent, fmt.Errorf("I am Error"))
		time.Sleep(SleepTime)
	}
	time.Sleep(time.Second)
	if reqSuccessCount != testCount {
		t.Fatal("请求未通过")
	}
	if noticeSuccessCount != testCount {
		t.Fatal("通知测试未通过")
	}
	if retainNoticeSuccessCount != testCount {
		t.Fatal("Retain通知测试未通过")
	}
	if reqDetectedCount != testCount {
		t.Fatal("请求检测未通过")
	}
	if respDetectedCount != testCount {
		t.Fatal("响应检测未通过")
	}
	if logSuccessCount != testCount {
		t.Fatal("日志检测未通过")
	}
}

func onReqRecMonitor(pack easyCon.PackReq) (easyCon.EResp, any) {
	reqDetectedCount++
	fmt.Printf("[%s]:Detected========== REQ From %s To %s  %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), pack.From, pack.To, pack.Id, pack.Content, reqDetectedCount, testCount)
	return easyCon.ERespBypass, nil
}

func onRespRec(pack easyCon.PackResp) {
	respDetectedCount++
	fmt.Printf("[%s]:Detected========== RESP From %s To %s  %d %s [%d/%d] \r\n", time.Now().Format("15:04:05.000"), pack.To, pack.From, pack.Id, pack.Content, respDetectedCount, testCount)
}
func onStatusChangedA(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module A status changed to ", statusStr)
}

func onRetainNotice(notice easyCon.PackNotice) {
	retainNoticeSuccessCount++
	fmt.Printf("[%s]:Detected========== RetainNotice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From, retainNoticeSuccessCount, testCount)
}

func onNoticeRec(notice easyCon.PackNotice) {
	if notice.Content == NoticeContent {
		noticeSuccessCount++
	}
	fmt.Printf("[%s]:Detected========== Notice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From, noticeSuccessCount, testCount)
}

func onLogRec(log easyCon.PackLog) {
	if log.Content == LogContent {
		logSuccessCount++
	}
	fmt.Printf("[%s]:Detected========== Log %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content, log.From, logSuccessCount, testCount)
}
func onStatusChangedB(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module B status changed to ", statusStr)
}

func onReqRec(_ easyCon.PackReq) (easyCon.EResp, any) {
	return easyCon.ERespSuccess, RespContent
}
