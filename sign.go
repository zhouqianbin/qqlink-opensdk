package opensdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// structToParams 把请求结构体转成签名用的键值表。
//
// 走 JSON 中转：与我方服务端「解析请求 body 得到 map」的做法一致，
// 保证双方对同一份请求算出同样的参数集合。
func structToParams(in any) map[string]string {
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]string{}
	}
	m := map[string]any{}
	if err = json.Unmarshal(raw, &m); err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		// 我方签名函数用 fmt.Sprintf("%v") 拼接，这里保持同样格式。
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// verifyNoticeSignature 按回调口径校验签名。
//
// 与请求签名口径**不同**：分隔符为 "|"，首段是 noticeType，
// 之后是 appKey 与按类型固定的字段。支付结果为：
//
//	payment|appKey|orderNo|outOrderNo|status|amount|timestamp|nonce
//
// 字段从解析后的 data 段取，而不是复用请求签名的参数表；
// nonce 取 body 里的 data.nonce，与签名串末尾一致。
func verifyNoticeSignature(secret, appKey string, body []byte, signature string) bool {
	payload := &NoticePayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		return false
	}
	fields, err := payload.noticeFields()
	if err != nil {
		return false
	}
	base := payload.NoticeType + "|" + appKey
	for _, f := range fields {
		base += "|" + f
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(base))
	expected := hex.EncodeToString(mac.Sum(nil))

	// 恒定时间比较，避免时序侧信道。
	return hmac.Equal([]byte(expected), []byte(signature))
}
