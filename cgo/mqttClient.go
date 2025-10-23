package main

/*
#include <string.h>
#include <stdio.h>
#include <stdint.h>

typedef int (*OnReqCallBack)(char*, int, char**, int*);
static int ReqHandler(OnReqCallBack cb, char* pack, int len, char** respJson, int* respLen) {
	return cb(pack,len,respJson,respLen);
}

typedef void (*OnLogCallBack)(char*, int);
static void LogHandler(OnLogCallBack cb, char* log, int len) {
	cb(log,len);
}

typedef void (*OnNoticeCallBack)(char*, int);
static void NoticeHandler(OnNoticeCallBack cb, char* log, int len) {
	cb(log,len);
}

typedef void (*OnStatusChangedCallBack)(char*);
static void StatusChangeHandler(OnStatusChangedCallBack cb, char* state) {
	cb(state);
}

*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"io"
	"os"
	"time"
	"unsafe"
)

func main() {

}

var (
	adapter easyCon.IAdapter
)

//export Start
func Start(settingJson *C.char, length C.int, onReqCallback C.OnReqCallBack,
	onLogCallback C.OnLogCallBack, onNoticeCallback C.OnNoticeCallBack,
	onStateChangedCallback C.OnStatusChangedCallBack) C.int {
	setting := easyCon.Setting{}
	str := C.GoBytes(unsafe.Pointer(settingJson), length)
	err := json.Unmarshal([]byte(string(str)), &setting)
	if err != nil {
		writeErrLog(err, string(str))
		return C.int(0)
	}
	// 时间转换
	setting.TimeOut = setting.TimeOut * time.Millisecond
	// 注册回调
	setting.OnReq = func(pack easyCon.PackReq) (easyCon.EResp, any) {
		if onReqCallback == nil {
			return easyCon.ERespError, "call req error"
		}
		data, _ := json.Marshal(pack)
		size := C.int(len(data))
		packStr := (*C.char)(C.CBytes(data))

		// 调用C回调函数
		var respLength C.int
		var respJson *C.char
		rs := C.ReqHandler(onReqCallback, packStr, size, &respJson, &respLength)
		if rs == 0 {
			return easyCon.ERespError, "call req error"
		}

		// Json反转...
		resp := easyCon.PackResp{}
		str := C.GoBytes(unsafe.Pointer(respJson), respLength)
		err := json.Unmarshal([]byte(string(str)), &resp)
		if err != nil {
			return easyCon.ERespError, err.Error()
		}

		return resp.RespCode, resp.Content
	}
	setting.OnLog = func(log easyCon.PackLog) {
		if onLogCallback == nil {
			return
		}
		data, _ := json.Marshal(log)
		size := C.int(len(data))
		packStr := (*C.char)(C.CBytes(data))
		C.LogHandler(onLogCallback, packStr, size)
	}
	setting.OnNotice = func(content easyCon.PackNotice) {
		if onNoticeCallback == nil {
			return
		}
		data, _ := json.Marshal(content)
		size := C.int(len(data))
		packStr := (*C.char)(C.CBytes(data))
		C.NoticeHandler(onNoticeCallback, packStr, size)
	}
	setting.StatusChanged = func(status easyCon.EStatus) {
		if onStateChangedCallback == nil {
			return
		}
		data := []byte(status)
		statusStr := (*C.char)(C.CBytes(data))
		C.StatusChangeHandler(onStateChangedCallback, statusStr)
	}

	// 创建对象
	adapter = easyCon.NewMqttAdapter(setting)

	// 返回id
	return C.int(1)
}

//export Stop
func Stop() {
	if adapter == nil {
		return
	}
	adapter.Stop()
}

//export Reset
func Reset() {
	if adapter == nil {
		return
	}
	adapter.Reset()
}

//export Req
func Req(moduleStr, route *C.char, paramsJson *C.char, length C.int, respJson **C.char, respLength *C.int) C.int {
	moduleGo := C.GoString(moduleStr)
	routeGo := C.GoString(route)
	var obj any
	str := C.GoBytes(unsafe.Pointer(paramsJson), length)
	err := json.Unmarshal([]byte(string(str)), &obj)
	if err != nil {
		writeErrLog(err, fmt.Sprintf("%s, %d", string(str), length))
		return C.int(0)
	}
	// 执行申请，等待响应
	resp := adapter.Req(moduleGo, routeGo, obj)

	// 转为json并返回
	data, _ := json.Marshal(resp)
	*respJson = (*C.char)(C.CBytes(data))
	*respLength = C.int(len(data))
	return C.int(1)
}

//export SendNotice
func SendNotice(route *C.char, paramsJson *C.char, length C.int) C.int {
	routeGo := C.GoString(route)
	var obj any
	str := C.GoBytes(unsafe.Pointer(paramsJson), length)
	err := json.Unmarshal([]byte(string(str)), &obj)
	if err != nil {
		return C.int(0)
	}
	err = adapter.SendNotice(routeGo, obj)
	if err != nil {
		return C.int(0)
	}
	return C.int(1)
}

//export LogDebug
func LogDebug(content *C.char, length C.int) C.int {
	str := C.GoBytes(unsafe.Pointer(content), length)
	adapter.Debug(string(str))

	return C.int(1)
}

//export LogError
func LogError(content *C.char, length C.int, error *C.char, errLen C.int) C.int {
	str := C.GoBytes(unsafe.Pointer(content), length)
	err := C.GoBytes(unsafe.Pointer(error), errLen)
	adapter.Err(string(str), errors.New(string(err)))

	return C.int(1)
}

//export LogWarn
func LogWarn(content *C.char, length C.int) C.int {
	str := C.GoBytes(unsafe.Pointer(content), length)
	adapter.Warn(string(str))

	return C.int(1)
}

func writeErrLog(err error, str string) {
	s := fmt.Sprintf("error: %s \n%s", err, str)
	f, _ := os.OpenFile(".\\error.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	defer f.Close()
	_, err = io.WriteString(f, s)
}
