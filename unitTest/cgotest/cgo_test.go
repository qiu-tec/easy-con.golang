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
	testCount                = 10
	reqSuccessCount          = 0
	reqDetectedCount         = 0
	respDetectedCount        = 0
	noticeSuccessCount       = 0
	retainNoticeSuccessCount = 0
	logSuccessCount          = 0
)

func Test(t *testing.T) {
	//testInner(t)
	testProxy(t)
}
func testProxy(t *testing.T) {
	broker := easyCon.NewCgoBroker()
	setting := easyCon.CoreSetting{
		Module:            "ModuleA",
		TimeOut:           time.Second * 3,
		ReTry:             0,
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

	sc := easyCon.NewDefaultMqttSetting("ModuleB", "ws://127.0.0.1:5002/ws")
	cbc := easyCon.AdapterCallBack{
		OnReqRec:        onReqRec,
		OnStatusChanged: onStatusChangedC,
	}
	mc := easyCon.NewMqttAdapter(sc, cbc)
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
		Addr:    "ws://127.0.0.1:5002/ws",
		ReTry:   0,
		PreFix:  "",
		TimeOut: time.Second * 3,
	}
	_, fp := easyCon.NewCgoMqttProxy(ps, broker.Publish)
	broker.RegClient("Proxy", fp)

	time.Sleep(time.Second)

	reqSuccessCount = 0
	reqDetectedCount = 0
	respDetectedCount = 0
	noticeSuccessCount = 0
	retainNoticeSuccessCount = 0
	logSuccessCount = 0

	time.Sleep(time.Second)
	for i := 0; i < testCount; i++ {
		resp := moduleA.Req("ModuleC", "PING", "I am ModuleA")
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
		} else {
			reqSuccessCount++
			fmt.Printf("[%s]: %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求成功", reqSuccessCount, testCount)
		}
		time.Sleep(time.Second)

		resp = mc.Req("ModuleA", "PING", "I am ModuleC")
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "反向请求失败")
		} else {
			reqDetectedCount++
			fmt.Printf("[%s]: %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), resp.Id, "反向请求检测成功", reqDetectedCount, testCount)
		}
		time.Sleep(time.Second)

		err := moduleA.SendNotice("Notice", "I am Notice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		}
		time.Sleep(time.Second)

		err = moduleA.SendRetainNotice("RetainNotice", "I am RetainNotice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain通知发送失败")
		}
		time.Sleep(time.Second)

		moduleA.Err("I am Error", fmt.Errorf("I am Error"))
		time.Sleep(time.Second)
	}
	time.Sleep(time.Second * 1)
	if reqSuccessCount != testCount*2 {
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

func onStatusChangedC(status easyCon.EStatus) {
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module C status changed to ", easyCon.GetStatusName(status))
}

func testInner(t *testing.T) {
	broker := easyCon.NewCgoBroker()
	setting := easyCon.CoreSetting{
		Module:            "ModuleA",
		TimeOut:           time.Second * 3,
		ReTry:             0,
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

	time.Sleep(time.Second)

	reqSuccessCount = 0
	reqDetectedCount = 0
	respDetectedCount = 0
	noticeSuccessCount = 0
	retainNoticeSuccessCount = 0
	logSuccessCount = 0

	time.Sleep(time.Second)
	for i := 0; i < testCount; i++ {
		resp := moduleA.Req("ModuleB", "PING", "I am ModuleA")
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
		} else {
			reqSuccessCount++
			fmt.Printf("[%s]: %d %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求成功", reqSuccessCount, testCount)
		}
		time.Sleep(time.Second)

		err := moduleA.SendNotice("Notice", "I am Notice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		}
		time.Sleep(time.Second)

		err = moduleA.SendRetainNotice("RetainNotice", "I am RetainNotice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain通知发送失败")
		}
		time.Sleep(time.Second)

		moduleA.Err("I am Error", fmt.Errorf("I am Error"))
		time.Sleep(time.Second)
	}
	time.Sleep(time.Second * 1)
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

	reqSuccessCount = 0
	reqDetectedCount = 0
	respDetectedCount = 0
	noticeSuccessCount = 0
	retainNoticeSuccessCount = 0
	logSuccessCount = 0

	for i := 0; i < testCount; i++ {
		resp := moduleA.Req("ModuleA", "PING", "I am ModuleC")
		if resp.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), resp.Id, "请求失败")
		}
		time.Sleep(time.Second)

		err := moduleA.SendNotice("Notice", "I am Notice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		}
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
	noticeSuccessCount++
	fmt.Printf("[%s]:Detected========== Notice %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content, notice.From, noticeSuccessCount, testCount)
}

func onLogRec(log easyCon.PackLog) {
	logSuccessCount++
	fmt.Printf("[%s]:Detected========== Log %d  %s From %s [%d/%d]\r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content, log.From, logSuccessCount, testCount)
}
func onStatusChangedB(status easyCon.EStatus) {
	statusStr := easyCon.GetStatusName(status)
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "module B status changed to ", statusStr)
}

func onReqRec(_ easyCon.PackReq) (easyCon.EResp, any) {
	return easyCon.ERespSuccess, "Pong"
}
