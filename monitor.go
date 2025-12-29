/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/22 16:39
 */

package easyCon

import "strings"

func NewMqttMonitor(setting MqttSetting, callback AdapterCallBack) IAdapter {
	//setting.IsRandomClientID = true
	setting.IsWaitLink = false
	//setting.Module = "Monitor"
	f := callback.OnLinked
	callback.OnLinked = func(adapter IAdapter) {
		if callback.OnNoticeRec != nil {
			adapter.SubscribeNotice("#", false)
		}
		if callback.OnRetainNoticeRec != nil {
			adapter.SubscribeNotice("#", true)
		}
		if callback.OnReqRec != nil {
			adapter.GetEngineCallback().OnSubscribe(BuildReqTopic(setting.PreFix, "#"), EPTypeReq, func(pack IPack) {
				callback.OnReqRec(*pack.(*PackReq))
			})
		}
		if callback.OnRespRec != nil {
			adapter.GetEngineCallback().OnSubscribe(BuildRespTopic(setting.PreFix, "#"), EPTypeResp, func(pack IPack) {
				callback.OnRespRec(*pack.(*PackResp))
			})
		}
		if f != nil {
			f(adapter)
		}
	}

	return newMqttAdapterInner(setting, callback)
}
func NewCGoMonitor(setting CoreSetting, callback AdapterCallBack, onWrite func([]byte) error) (IAdapter, func([]byte)) {
	setting.IsWaitLink = false
	f := callback.OnLinked
	callback.OnLinked = func(adapter IAdapter) {
		if callback.OnNoticeRec != nil {
			adapter.SubscribeNotice("#", false)
		}
		if callback.OnRetainNoticeRec != nil {
			adapter.SubscribeNotice("#", true)
		}
		if callback.OnReqRec != nil {
			adapter.GetEngineCallback().OnSubscribe(BuildReqTopic(setting.PreFix, "#"), EPTypeReq, func(pack IPack) {
				reqPack := *pack.(*PackReq)
				if strings.HasPrefix(reqPack.From, setting.Module) { //来自自己的就不需要再触发了
					return
				}

				callback.OnReqRec(reqPack)
			})
		}
		if callback.OnRespRec != nil {
			adapter.GetEngineCallback().OnSubscribe(BuildRespTopic(setting.PreFix, "#"), EPTypeResp, func(pack IPack) {
				resp := *pack.(*PackResp)
				if strings.HasPrefix(resp.From, setting.Module) { //来自自己的就不需要再触发了
					return
				}
				callback.OnRespRec(resp)
			})
		}

		if f != nil {
			f(adapter)
		}
	}
	return NewCgoAdapter(setting, callback, onWrite)
}
