/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/1 19:57
 */

package proxy

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"testing"
	"time"
)

func TestMqttProxy(t *testing.T) {

	fmt.Println("正向代理测试开始")
	//正向代理模式
	forwardMode()
	//time.Sleep(time.Second * 5)
	//fmt.Println("反向代理测试开始")
	////反向代理模式
	//reverseMode()
}

func forwardMode() {
	proxy := easyCon.NewMqttProxy(
		easyCon.MqttProxySetting{
			Addr:    "ws://127.0.0.1:5002/ws",
			PreFix:  "",
			TimeOut: time.Second * 3,
		},
		easyCon.MqttProxySetting{
			Addr:    "ws://127.0.0.1:5002/ws",
			PreFix:  "B.",
			TimeOut: time.Second * 3,
		},
		true, true, true,
		[]string{"ModuleA"})

	setting := easyCon.NewDefaultMqttSetting("ModuleA", "ws://127.0.0.1:5002/ws")
	setting.LogMode = easyCon.ELogModeUpload
	cba := easyCon.AdapterCallBack{
		OnReqRec:          onReqA,
		OnLogRec:          onLogA,
		OnNoticeRec:       onNoticeA,
		OnRetainNoticeRec: onRetainNoticeA,
		//OnLinked:          nil,
		//OnExiting:         nil,
		//OnGetVersion:      nil,
	}
	modA := easyCon.NewMqttAdapter(setting, cba)

	setting.PreFix = "B."
	setting.Module = "ModuleB"
	cbb := easyCon.AdapterCallBack{
		OnReqRec:          onReqB,
		OnLogRec:          onLogB,
		OnNoticeRec:       onNoticeB,
		OnRetainNoticeRec: onRetainNoticeB,
		//OnLinked:          nil,
		//OnExiting:         nil,
		//OnGetVersion:      nil,
	}
	modB := easyCon.NewMqttAdapter(setting, cbb)

	//for i := 0; i < 2; i++ {
	//	err := modB.SendNotice("Notice", "I am ModuleB Notice")
	//	if err != nil {
	//		fmt.Println(err)
	//	}
	//
	//	time.Sleep(time.Second)
	//}
	//
	//time.Sleep(time.Second)
	//_ = modB.SendRetainNotice("RetainNotice", "I am ModuleB RetainNotice")
	//time.Sleep(time.Second)
	//_ = modB.CleanRetainNotice()
	//time.Sleep(time.Second)
	//modB.Debug("I am ModuleB Debug")
	//time.Sleep(time.Second)

	for i := 0; i < 5; i++ {
		//time.Sleep(time.Second)
		res := modA.Req("ModuleB", "Who are you", "")
		fmt.Println(res)
		time.Sleep(time.Second)
	}
	for i := 0; i < 5; i++ {
		//time.Sleep(time.Second)
		res := modB.Req("ModuleA", "Who are you", "")
		fmt.Println(res)
		time.Sleep(time.Second)
	}
	time.Sleep(time.Second * 1)
	modA.Stop()
	modB.Stop()
	proxy.Stop()
}

func reverseMode() {
	proxy := easyCon.NewMqttProxy(
		easyCon.MqttProxySetting{
			Addr:    "ws://127.0.0.1:5002/ws",
			PreFix:  "A.",
			TimeOut: time.Second * 3,
		},
		easyCon.MqttProxySetting{
			Addr:    "ws://127.0.0.1:5002/ws",
			PreFix:  "",
			TimeOut: time.Second * 3,
		},
		true, true, true,
		[]string{"ModuleB"})

	setting := easyCon.NewDefaultMqttSetting("ModuleA", "ws://127.0.0.1:5002/ws")
	setting.PreFix = "A."
	setting.LogMode = easyCon.ELogModeUpload
	cba := easyCon.AdapterCallBack{
		OnReqRec:          onReqA,
		OnLogRec:          onLogA,
		OnNoticeRec:       onNoticeA,
		OnRetainNoticeRec: onRetainNoticeA,
		//OnLinked:          nil,
		//OnExiting:         nil,
		//OnGetVersion:      nil,
	}
	modA := easyCon.NewMqttAdapter(setting, cba)
	time.Sleep(time.Second)
	setting.PreFix = ""
	setting.Module = "ModuleB"
	cbb := easyCon.AdapterCallBack{
		OnReqRec:          onReqB,
		OnLogRec:          onLogB,
		OnNoticeRec:       onNoticeB,
		OnRetainNoticeRec: onRetainNoticeB,
		//OnLinked:          nil,
		//OnExiting:         nil,
		//OnGetVersion:      nil,
	}
	modB := easyCon.NewMqttAdapter(setting, cbb)
	_ = modA.SendNotice("Notice", "I am ModuleA Notice")
	time.Sleep(time.Second)
	_ = modA.SendRetainNotice("RetainNotice", "I am ModuleA RetainNotice")
	time.Sleep(time.Second)
	_ = modA.CleanRetainNotice("RetainNotice")

	time.Sleep(time.Second)
	modA.Debug("I am ModuleA Debug")
	time.Sleep(time.Second)
	for i := 0; i < 10; i++ {
		res := modB.Req("ModuleA", "Who are you", "")
		fmt.Println(res)
		time.Sleep(time.Second)
	}
	time.Sleep(time.Second * 1)
	modA.Stop()
	modB.Stop()
	proxy.Stop()
}

func onRetainNoticeB(notice easyCon.PackNotice) {
	fmt.Println(time.Now().Format("15:04:05.000"), notice, "B")
}

func onRetainNoticeA(notice easyCon.PackNotice) {
	fmt.Println(time.Now().Format("15:04:05.000"), notice, "A")
}

func onNoticeA(notice easyCon.PackNotice) {
	fmt.Println(time.Now().Format("15:04:05.000"), notice, "A")
}
func onNoticeB(notice easyCon.PackNotice) {
	fmt.Println(time.Now().Format("15:04:05.000"), notice, "B")
}
func onLogA(log easyCon.PackLog) {
	fmt.Println(time.Now().Format("15:04:05.000"), log, "A")
}
func onLogB(log easyCon.PackLog) {
	fmt.Println(time.Now().Format("15:04:05.000"), log, "B")
}
func onReqB(_ easyCon.PackReq) (easyCon.EResp, []byte) {
	return easyCon.ERespSuccess, []byte("I'm ModuleB")
}

func onReqA(_ easyCon.PackReq) (easyCon.EResp, []byte) {
	return easyCon.ERespSuccess, []byte("I'm ModuleA")
}
