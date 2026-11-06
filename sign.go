package opensdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SignNotice 计算回调签名：对原始通知 body 做 HMAC-SHA256（hex）。
//
// 待签串（字面量换行分隔）：
//
//	notice\n{appKey}\n{rawBody}
//
// 与请求 SignBody 一样签原始字节，避免字段用 "|" 拼接时的边界歧义。
// 接入方验签必须使用未经改动的原始 body。
func SignNotice(appSecret, appKey string, body []byte) string {
	var b strings.Builder
	b.Grow(7 + len(appKey) + len(body) + 2)
	b.WriteString("notice\n")
	b.WriteString(appKey)
	b.WriteByte('\n')
	b.Write(body)

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(b.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyNoticeSignature 按回调口径校验签名（恒定时间比较）。
func verifyNoticeSignature(secret, appKey string, body []byte, signature string) bool {
	expected := SignNotice(secret, appKey, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}
