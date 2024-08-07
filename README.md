# how to use it?

## first in shell:
~~~shell
go get github.com/qiu-tec/easy-con.golang
~~~
## second in your golang code like this
~~~go
package main

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
)
func main() {
	//create a default setting
	addr := "ws://xxxxxx.xxx.xx/mqtt"
	setting := easyCon.NewSetting("ModuleA", addr, onReq, onStatusChanged)
	//if has uid and pwd
	setting.UID = "xxxx"
	setting.PWD = "xxxxx"

	moduleA := easyCon.NewMqttAdapter(setting)
	defer moduleA.Stop()

	setting.Module = "ModuleB"
	setting.OnNotice = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), notice.Content)
	}
	setting.OnLog = func(log easyCon.PackLog) {
		fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), log.Content)
	}
	moduleB := easyCon.NewMqttAdapter(setting)
	defer moduleB.Stop()

	moduleA.Req("ModuleB", "hello", nil)
	err :=moduleA.SendNotice("debug","hello world")
	if err != nil {
		fmt.Println(err.Error())
	}

	moduleA.Debug("this is a log")
	moduleB.Req("ModuleA", "hello", nil)
  
}
func onStatusChanged(adapter easyCon.IAdapter, status easyCon.EStatus) {
	fmt.Println("StatusChanged", status)
}

func onReq(pack easyCon.PackReq) (easyCon.EResp, any) {
	switch pack.Route {
	case "hello":
		return easyCon.ERespSuccess, "hello"
	}
	return easyCon.ERespRouteNotFind, "Route Not Matched"
}
~~~

just do it
