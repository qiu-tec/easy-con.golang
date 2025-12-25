/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/6/12 17:46
 */

package expressTest

import (
	easyCon "github.com/qiu-tec/easy-con.golang"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestExpress(t *testing.T) {
	setting := easyCon.NewDefaultMqttSetting("ModuleA", "ws://127.0.0.1:5002/ws", onReq, onStatusChanged)
	moduleA := easyCon.NewMqttAdapter(setting)
	setting.Module = "ModuleB"
	moduleB := easyCon.NewMqttAdapter(setting)
	wg := &sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		go func() {
			wg.Add(1)
			defer wg.Done()
			doTest(moduleA, moduleB)
		}()
	}
	wg.Wait()
}

func doTest(a easyCon.IAdapter, b easyCon.IAdapter) {
	for i := 0; i < 1000; i++ {
		a.Req("ModuleB", "Ping", "Ping from ModuleA")
		b.Req("ModuleA", "Ping", "Ping from ModuleB")
	}

}

func onReq(pack easyCon.PackReq) (easyCon.EResp, any) {
	rd := rand.Intn(100)

	time.Sleep(time.Duration(rd) * time.Millisecond)
	return easyCon.ERespSuccess, "Pong"
}

func onStatusChanged(status easyCon.EStatus) {

}
