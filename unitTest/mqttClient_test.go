/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/11 20:06
 */

package unitTest

import (
	"errors"
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"testing"
	"time"
)

func TestMqttClient(t *testing.T) {

	addr := "ws://127.0.0.1:38083/ws"
	setting := easyCon.NewSetting("ModuleA", addr, onReq, onStatusChanged)
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	//setting.LogMode = EasyCon.ELogModeNone
	//setting.LogMode = easyCon.ELogModeConsole
	setting.LogMode = easyCon.ELogModeUpload
	moduleA := easyCon.NewMqttAdapter(setting)
	setting.Module = "ModuleB"
	setting.OnNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]:Notice %s \r\n", time.Now().Format("15:04:05.000"), notice.Content)
	}
	setting.OnRetainNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]:RetainNotice %s \r\n", time.Now().Format("15:04:05.000"), notice.Content)
	}
	setting.OnLog = func(log easyCon.PackLog) {
		fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), log.Content)
	}

	moduleB := easyCon.NewMqttAdapter(setting)
	defer func() {
		moduleA.Stop()
		moduleB.Stop()
	}()

	moduleA.Reset()
	moduleA.Reset()
	moduleA.Reset()
	moduleB.Reset()
	//断线重连测试 需要手动断开服务端连接或网线
	//time.Sleep(time.Second * 30)

	for i := 0; i < 5; i++ {

		//go func() {
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

		//}()

	}

	for i := 0; i < 10; i++ {
		//go func() {
		err := moduleA.SendNotice("debugLog", "I am ModuleA Notice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
		} else {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送成功")
		}
		err = moduleA.SendRetainNotice("debugLog", "I am ModuleA Notice")
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "保留通知发送失败")
		} else {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "保留通知发送成功")
		}
		//}()
		time.Sleep(time.Second)

	}
	moduleA.Err("日志测试", errors.New("ModuleA log error"))
	moduleB.Err("日志测试", errors.New("ModuleB log error"))
	//time.Sleep(time.Second * 10)
}

func onStatusChanged(adapter easyCon.IAdapter, status easyCon.EStatus) {
	adapter.Debug(string(status))
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
