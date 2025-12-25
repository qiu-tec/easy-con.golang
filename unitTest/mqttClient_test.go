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

type Info struct {
	Msg  string
	ID   int
	User string
}

func TestMqttClient(t *testing.T) {

	addr := "ws://127.0.0.1:5002/ws"
	setting := easyCon.NewDefaultMqttSetting("ModuleA", addr)
	//sync mode test
	//setting.EProtocol = easyCon.EProtocolMQTT
	//setting.IsWaitLink = false
	setting.LogMode = easyCon.ELogModeUpload
	cb := easyCon.AdapterCallBack{
		OnReqRec: onReq,
		OnLogRec: func(log easyCon.PackLog) {
			fmt.Printf("[%s]:Log %d received %s \r\n", time.Now().Format("15:04:05.000"), log.Id, log.Content)
		},
		OnNoticeRec: func(notice easyCon.PackNotice) {
			fmt.Printf("[%s]:Notice %d received %s \r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content)
		},
		OnRetainNoticeRec: func(notice easyCon.PackNotice) {
			fmt.Printf("[%s]:RetainNotice %d received %s \r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Content)
		},
		OnLinked:     nil,
		OnExiting:    nil,
		OnGetVersion: nil,
		OnStatusChanged: func(status easyCon.EStatus) {
			fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "status changed", status)
		},
	}
	moduleA := easyCon.NewMqttAdapter(setting, cb)
	setting.Module = "ModuleB"

	moduleB := easyCon.NewMqttAdapter(setting, cb)
	defer func() {
		moduleA.Stop()
		moduleB.Stop()
	}()
	moduleA.Debug(fmt.Sprintf("===============test start at %s", time.Now().Format("15:04:05.000")))

	moduleA.Reset()

	moduleB.Reset()
	//time.Sleep(time.Second)
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("I am ModuleA %d", i)
		//go func() {
		res := moduleA.Req("ModuleB", "PING", content)
		if res.RespCode != easyCon.ERespSuccess {
			fmt.Printf("[%s]: %d %s \r\n", time.Now().Format("15:04:05.000"), res.Id, "request failed")
		}
		//}()
		time.Sleep(time.Millisecond * 100)
		//go func() {
		err := moduleA.SendNotice("HereWeGo", content)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "notice send failed")
		}
		err = moduleA.SendRetainNotice("YaHaHa", content)
		if err != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "retain notice send failed")
		}
		//}()
	}
	_ = moduleA.SendNotice("TestNotice", Info{Msg: "I am ModuleA Notice", ID: rand.Int(), User: "Joey"})
	_ = moduleA.SendNotice("TestNotice", []Info{
		{Msg: "I am ModuleA RetainNotice", ID: rand.Int(), User: "Joey"},
		{Msg: "I am ModuleA RetainNotice", ID: rand.Int(), User: "Joey"},
		{Msg: "I am ModuleA RetainNotice", ID: rand.Int(), User: "Joey"}})
	_ = moduleA.SendNotice("TestFloatNotice", 1.1)
	_ = moduleA.SendNotice("TestBytesNotice", []byte{1, 2, 3, 4})
	_ = moduleA.Req("ModuleB", "Test", Info{Msg: "I am ModuleA Notice", ID: rand.Int(), User: "Joey"})

	time.Sleep(time.Second * 5)
}

func onReq(pack easyCon.PackReq) (easyCon.EResp, any) {
	fmt.Printf("[%s]:Req %d Received %s \r\n", time.Now().Format("15:04:05.000"), pack.Id, pack.Content)
	n := rand.Intn(5) * 100

	switch pack.Route {
	case "PING":
		time.Sleep(time.Millisecond * time.Duration(n))
		return easyCon.ERespSuccess, "PONG"
	case "Test":
		time.Sleep(time.Millisecond * time.Duration(n))
		return easyCon.ERespSuccess, pack.Content
	default:
		return easyCon.ERespRouteNotFind, nil
	}
}
