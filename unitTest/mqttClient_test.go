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

	addr := "ws://172.18.200.21:16802/mqtt"
	//addr := "wss://uvms.urit.com/mqtt"
	setting := easyCon.NewSetting("ModuleA", addr, onReq, onStatusChanged)
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	//setting.LogMode = EasyCon.ELogModeNone
	setting.LogMode = easyCon.ELogModeUpload

	moduleA := easyCon.NewMqttAdapter(setting)
	setting.Module = "ModuleB"
	moduleB := easyCon.NewMqttAdapter(setting)
	defer func() {
		moduleA.Stop()
		moduleB.Stop()
	}()
	//time.Sleep(time.Second)
	for i := 0; i < 1; i++ {
		go func() {
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

		}()

	}
	for i := 0; i < 1; i++ {
		go func() {
			err := moduleA.SendNotice("I am ModuleA")
			if err != nil {
				fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送失败")
			} else {
				fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "通知发送成功")
			}
		}()

	}
	time.Sleep(time.Second * 10)
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
