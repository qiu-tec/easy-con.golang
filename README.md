//how to use it?
first in sh:
go get github.com/qiu-tec/easy-con.golang
//in your function
func main() {
 	addr := "ws://172.18.200.21:16802/mqtt"
	setting := easyCon.NewSetting("ModuleA", addr, onReq, onStatusChanged)
	setting.UID = "admin"
	setting.PWD = "ams@urit2024"
	moduleA := easyCon.NewMqttAdapter(setting)
	defer moduleA.Stop()
	setting.Module = "ModuleB"
	moduleB := easyCon.NewMqttAdapter(setting)
	defer moduleB.Stop()
	moduleA.Req("ModuleB", "hello", nil)
	err :=moduleA.SendNotice("hello world")
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
