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
	setting := easyCon.NewDefaultMqttSetting("ModuleA", addr, onReqRec, onStatusChanged)
	//if has uid and pwd
	setting.UID = "xxxx"
	setting.PWD = "xxxxx"

	moduleA := easyCon.NewMqttAdapter(setting)
	defer moduleA.Stop()

	setting.Module = "ModuleB"
	setting.OnNoticeRec = func(notice easyCon.PackNotice) {
		fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), notice.Content)
	}
	setting.OnLogRec = func(log easyCon.PackLog) {
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
func onStatusChanged(status easyCon.EStatus) {
	fmt.Println("StatusChanged", status)
}

func onReqRec(pack easyCon.PackReq) (easyCon.EResp, []byte) {
	switch pack.Route {
	case "hello":
		return easyCon.ERespSuccess, []byte("hello")
	}
	return easyCon.ERespRouteNotFind, []byte("Route Not Matched")
}
~~~

## 发送消息

```go
// 发送请求 - Content 统一使用 []byte
jsonData, _ := json.Marshal(userObj)
resp := adapter.Req("TargetModule", "getUserInfo", jsonData)

// 直接发送字符串
resp := adapter.Req("Logger", "log", []byte("hello world"))

// 直接发送二进制数据
resp := adapter.Req("Storage", "upload", fileData)
```

## 处理请求

```go
func onReq(pack PackReq) (EResp, []byte) {
    // pack.Content 是 []byte
    var params UserParams
    json.Unmarshal(pack.Content, &params)

    result := process(params)

    respData, _ := json.Marshal(result)
    return ERespSuccess, respData
}
```

just do it
