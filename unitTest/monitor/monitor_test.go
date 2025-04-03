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

func TestMqttDetective(t *testing.T) {
	monitorSetting := easyCon.NewMonitorSetting(
		"ws://127.0.0.1:5002/ws",
		"monitor",
		[]string{"ModuleA", "ModuleB"},
		onReq,
		onResp,
		onNotice,
		onRetainNotice,
		onLog,
		onStatus)
	monitor := easyCon.NewMqttMonitor(monitorSetting)
	setting := easyCon.NewSetting("ModuleA", "ws://127.0.0.1:5002/ws", onReqInvoke, onStatus)
	setting.OnNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]:Notice %s \r\n", time.Now().Format("15:04:05.000"), notice.Content)
	}
	setting.OnRetainNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]:RetainNotice %s \r\n", time.Now().Format("15:04:05.000"), notice.Content)
	}
	setting.OnLog = func(log easyCon.PackLog) {
		fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), log.Content)
	}

	moduleA := easyCon.NewMqttAdapter(setting)
	setting.Module = "ModuleB"
	moduleB := easyCon.NewMqttAdapter(setting)
	defer func() {
		monitor.Stop()
		moduleA.Stop()
		moduleB.Stop()
	}()

	for i := 0; i < 5; i++ {

		res := moduleA.Req("ModuleB", "PING", "I am ModuleA")
		if res.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求失败")
		} else {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求成功")
		}
		res = moduleB.Req("ModuleA", "PING", "I am ModuleB")
		if res.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求失败")
		} else {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "请求成功")
		}

	}

	for i := 0; i < 10; i++ {
		_ = moduleA.SendNotice("Notice", "I am ModuleA Notice")
		time.Sleep(time.Second)
		_ = moduleA.SendRetainNotice("debugLog", "I am ModuleA Notice")
		time.Sleep(time.Second)

	}
	moduleA.Err("moduleA日志测试", errors.New("ModuleA log error"))
	time.Sleep(time.Second)
	moduleB.Debug("moduleB日志测试")
	time.Sleep(time.Second)

	moduleA.Reset()
	time.Sleep(time.Second)
	moduleB.Reset()
	time.Sleep(time.Second)

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

func onResp(pack easyCon.PackResp) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected response=====", pack)
}

func onReq(pack easyCon.PackReq) {
	fmt.Println(time.Now().Format("15:04:05.000"), "Monitor detected request=====", pack)
}
func onReqInvoke(pack easyCon.PackReq) (easyCon.EResp, any) {
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
