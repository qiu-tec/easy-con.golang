/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/4/3 17:54
 */

package monitor

import (
	"errors"
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"testing"
	"time"
)

var monitor easyCon.IAdapter

func TestMqttDetective(t *testing.T) {
	mqttSetting := easyCon.NewDefaultMqttSetting("Monitor", "ws://127.0.0.1:5002/ws")
	cb := easyCon.AdapterCallBack{
		OnReqRec:          onReqDetected,
		OnRespRec:         onRespDetected,
		OnNoticeRec:       onNotice,
		OnLogRec:          onLog,
		OnRetainNoticeRec: onRetainNotice,
	}
	//monitor = easyCon.NewMqttMonitor(mqttSetting, cb, true, []string{"ModuleA", "ModuleB"})
	monitor = easyCon.NewMqttMonitor(mqttSetting, cb)
	setting := easyCon.NewDefaultMqttSetting("ModuleA", "ws://127.0.0.1:5002/ws")
	setting.LogMode = easyCon.ELogModeUpload
	adapterCb := easyCon.AdapterCallBack{
		OnReqRec: onReq,
		//OnLogRec:          nil,
		//OnNoticeRec:       nil,
		//OnRetainNoticeRec: nil,
		//OnLinked:          nil,
		//OnExiting:         nil,
		//OnGetVersion:      nil,
		OnStatusChanged: onStatus,
	}
	adapterCb.OnStatusChanged = nil
	moduleA := easyCon.NewMqttAdapter(setting, adapterCb)

	setting.Module = "ModuleB"

	moduleB := easyCon.NewMqttAdapter(setting, adapterCb)
	defer func() {
		monitor.Stop()
		moduleA.Stop()
		moduleB.Stop()
	}()
	for i := 0; i < 5; i++ {
		_ = moduleA.SendNotice("Notice", []byte("I am ModuleA Notice"))
		time.Sleep(time.Second)
		_ = moduleA.SendRetainNotice("RetainNotice", []byte("I am ModuleA RetainNotice"))
		time.Sleep(time.Second)
		_ = moduleA.CleanRetainNotice("RetainNotice")
		//moduleB.Debug("moduleB日志测试")

	}

	moduleA.Err("moduleA日志测试", errors.New("ModuleA log error"))
	time.Sleep(time.Second)
	//moduleB.Debug("moduleB日志测试")
	for i := 0; i < 5; i++ {
		moduleA.Req("ModuleB", "Ping", []byte("Ping from ModuleA"))
		time.Sleep(time.Second)
		moduleB.Req("ModuleA", "Ping", []byte("Ping from ModuleB"))
		time.Sleep(time.Second)
	}
	fmt.Println("Job finished")

	time.Sleep(time.Second * 100)
}
func onStatus(status easyCon.EStatus) {
	fmt.Println(time.Now().Format("15:04:05.000"), status)
}

func onLog(log easyCon.PackLog) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected log=====", log)
}

func onRetainNotice(notice easyCon.PackNotice) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected retain notice=====", notice)
}

func onNotice(notice easyCon.PackNotice) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected notice=====", notice)
}

func onRespDetected(pack easyCon.PackResp) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected response=====", pack)
}

func onReqDetected(pack easyCon.PackReq) (easyCon.EResp, []byte) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected request=====", pack)
	return easyCon.ERespBypass, nil
}
func onReq(pack easyCon.PackReq) (easyCon.EResp, []byte) {
	switch pack.Route {
	case "PING":
		return easyCon.ERespSuccess, []byte("PONG")
	case "PONG":
		return easyCon.ERespBadReq, nil
	case "ERR":
		return easyCon.ERespError, nil
	default:
		return easyCon.ERespRouteNotFind, nil
	}
}
