/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/11 20:06
 */

package unitTest

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"math/rand"
	"testing"
	"time"
)

func TestMqttClient(t *testing.T) {

	addr := "ws://127.0.0.1:5002/ws"
	setting := easyCon.NewSetting("ModuleA", addr, onReq, onStatusChanged)
	//sync mode test
	setting.EProtocol = easyCon.EProtocolMQTTSync
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	setting.LogMode = easyCon.ELogModeUpload
	moduleA := easyCon.NewMqttAdapter(setting)
	setting.Module = "ModuleB"
	setting.OnNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]:Notice %d received %s \r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content)
	}
	setting.OnRetainNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]:RetainNotice %d received %s \r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content)
	}
	setting.OnLog = func(log easyCon.PackLog) {
		fmt.Printf("[%s]:Log %d received %s \r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content)
	}

	moduleB := easyCon.NewMqttAdapter(setting)
	defer func() {
		moduleA.Stop()
		moduleB.Stop()
	}()
	moduleA.Debug(fmt.Sprintf("===============test start at %s", time.Now().Format("15:04:05.000")))

	moduleA.Reset()

	moduleB.Reset()

	for i := 0; i < 20; i++ {
		content := fmt.Sprintf("I am ModuleA %d", i)
		go func() {
			res := moduleA.Req("ModuleB", "PING", content)
			if res.RespCode != easyCon.ERespSuccess {
				fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "request failed")
			}
		}()
		time.Sleep(time.Millisecond * 100)
		go func() {
			err := moduleA.SendNotice("HereWeGo", content)
			if err != nil {
				fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "notice send failed")
			}
			err = moduleA.SendRetainNotice("YaHaHa", content)
			if err != nil {
				fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "retain notice send failed")
			}
		}()
	}

	/*
		for i := 0; i < 10; i++ {
			content := fmt.Sprintf("I am ModuleA Notice %d", i)
			go func() {
				err := moduleA.SendNotice("debugLog", content)
				if err != nil {
					fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "notice send failed")
				}
				err = moduleA.SendRetainNotice("debugLog", "I am ModuleA Notice")
				if err != nil {
					fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "retain notice send failed")
				}
			}()
		}
		for i := 0; i < 10; i++ {
			content := fmt.Sprintf("I am ModuleA Log %d", i)
			go func() {
				moduleA.Err(content, errors.New(content))
			}()
		}
	*/

	time.Sleep(time.Second * 5)
}

func onStatusChanged(status easyCon.EStatus) {
	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "status changed", status)
}

func onReq(pack easyCon.PackReq) (easyCon.EResp, any) {
	fmt.Printf("[%s]:Req %d Received %s \r\n", time.Now().Format("15:04:05.000"), pack.Id, pack.Content)
	n := rand.Intn(5) * 100

	switch pack.Route {
	case "PING":
		time.Sleep(time.Millisecond * time.Duration(n))
		return easyCon.ERespSuccess, "PONG"
	default:
		return easyCon.ERespRouteNotFind, nil
	}
}
