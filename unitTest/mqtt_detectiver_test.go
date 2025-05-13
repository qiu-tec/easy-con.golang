/**
* @Author: Joey
* @Description:
* @Create Date: 2024/12/9 14:15
 */

package unitTest

//
//import (
//	"fmt"
//	easyCon "github.com/qiu-tec/easy-con.golang"
//	"sync"
//	"testing"
//	"time"
//)
//
//func TestMqttDetective(t *testing.T) {
//	wg := &sync.WaitGroup{}
//	wg.Add(1)
//	addr := "ws://127.0.0.1:5002/ws"
//	setting := easyCon.NewSetting("ModuleDetective", addr, nil, onStatusChanged)
//
//	setting.LogMode = easyCon.ELogModeUpload
//	setting.WatchedModules = []string{"ModuleA", "ModuleB"}
//	setting.PreFix = "Test_"
//	setting.OnLog = func(log easyCon.PackLog) {
//		fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), log.Content)
//	}
//	detectiveTimes := 0
//	setting.OnRespDetected = func(pack easyCon.PackResp) {
//		fmt.Println("[onRespDetected]", pack)
//		detectiveTimes++
//		if detectiveTimes >= 10 {
//			wg.Done()
//		}
//	}
//	easyCon.NewMqttAdapter(setting)
//	setting.WatchedModules = nil
//	setting.DetectedRoutes = []string{"Ping1", "Ping2"}
//	setting.OnReq = func(pack easyCon.PackReq) (easyCon.EResp, any) {
//		switch pack.Route {
//		case "Ping1":
//			return easyCon.ERespSuccess, "Pong1"
//		case "Ping2":
//			return easyCon.ERespSuccess, "Pong2"
//		default:
//			return easyCon.ERespRouteNotFind, nil
//		}
//	}
//	setting.Module = "ModuleA"
//	moduleA := easyCon.NewMqttAdapter(setting)
//	setting.Module = "ModuleB"
//	setting.DetectedRoutes = []string{`^Ping[\S]+`}
//	moduleB := easyCon.NewMqttAdapter(setting)
//	moduleB.Debug("测试日志")
//	_ = moduleB.SendNotice("测试通知", "notice")
//	go func() {
//		for i := 0; i < 5; i++ {
//			resp := moduleA.Req("ModuleB", "Ping1", "Ping")
//			fmt.Println("ModuleA Req", resp)
//			time.Sleep(time.Second)
//			resp = moduleB.Req("ModuleA", "Ping2", "Ping")
//			fmt.Println("ModuleB Req", resp)
//			time.Sleep(time.Second)
//		}
//
//	}()
//	wg.Wait()
//
//}
//func onStatusChanged(status easyCon.EStatus) {
//	fmt.Printf("[%s]: %s %s \r\n", time.Now().Format("15:04:05.000"), "状态改为", status)
//}
