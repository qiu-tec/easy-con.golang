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

var monitor easyCon.IMonitor

func TestMqttDetective(t *testing.T) {
	monitorSetting := easyCon.NewMonitorSetting(
		easyCon.NewSetting("Monitor", "ws://127.0.0.1:5002/ws", onReq, onStatus),
		[]string{"ModuleCloud"},
		onReqDetected,
		onRespDetected,
	)
	monitorSetting.OnLog = onLog
	monitorSetting.OnRetainNotice = onRetainNotice
	monitorSetting.OnNotice = onNotice
	monitor = easyCon.NewMqttMonitor(monitorSetting)
	setting := easyCon.NewSetting("ModuleA", "ws://127.0.0.1:5002/ws", onReq, onStatus)
	setting.LogMode = easyCon.ELogModeUpload

	time.Sleep(time.Second)
	moduleA := easyCon.NewMqttAdapter(setting)
	setting.Module = "ModuleB"
	time.Sleep(time.Second)
	moduleB := easyCon.NewMqttAdapter(setting)
	defer func() {
		monitor.Stop()
		moduleA.Stop()
		moduleB.Stop()
	}()
	for i := 0; i < 2; i++ {
		_ = moduleA.SendNotice("Notice", "I am ModuleA Notice")
		time.Sleep(time.Second)
		_ = moduleA.SendRetainNotice("RetainNotice", "I am ModuleA RetainNotice")
		time.Sleep(time.Second)
		moduleB.Debug("moduleB日志测试")

	}
	for i := 0; i < 5; i++ {

		res := moduleA.Req("ModuleB", "PING", "I am ModuleA")
		if res.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求失败")
		}
		time.Sleep(time.Second)
		res = moduleB.Req("ModuleA", "PING", "I am ModuleB")
		if res.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求失败")
		}
		time.Sleep(time.Second)
		//请求云服务的PING
		res = moduleA.Req("ModuleCloud", "PING", "I am ModuleA")
		if res.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求失败")
		}
		time.Sleep(time.Second)
	}

	moduleA.Err("moduleA日志测试", errors.New("ModuleA log error"))
	time.Sleep(time.Second)
	moduleB.Debug("moduleB日志测试")
	time.Sleep(time.Second)

	//moduleA.Reset()
	//time.Sleep(time.Second)
	//moduleB.Reset()
	//time.Sleep(time.Second)

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

func onReqDetected(pack easyCon.PackReq) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected request=====", pack)
	if pack.To == "ModuleCloud" {
		monitor.MonitorResp(pack, easyCon.ERespSuccess, "pong")
	}
}
func onReq(pack easyCon.PackReq) (easyCon.EResp, any) {
	switch pack.Route {
	case "PING":
		return easyCon.ERespSuccess, "PONG"
	case "PONG":
		return easyCon.ERespBadReq, nil
	case "ERR":
		return easyCon.ERespError, errors.New("error")
	default:
		return easyCon.ERespRouteNotFind, nil
	}
}
