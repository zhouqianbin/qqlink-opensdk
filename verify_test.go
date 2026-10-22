package opensdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const paymentBody = `{"noticeType":"payment","data":{"orderNo":"O1","outOrderNo":"A1","status":"success","chainName":"TRC20","amount":"10.00000000","timestamp":"1760851231","nonce":"n1"}}`

func signStr(secret, base string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(base))
	return hex.EncodeToString(m.Sum(nil))
}

func TestVerifyNoticePayment(t *testing.T) {
	sig := signStr("sec", "payment|APP1|O1|A1|success|10.00000000|1760851231|n1")
	if !verifyNoticeSignature("sec", "APP1", []byte(paymentBody), sig) {
		t.Fatal("合法签名应通过")
	}
	if verifyNoticeSignature("sec", "APP2", []byte(paymentBody), sig) {
		t.Fatal("appKey 不同应失败")
	}
	// 旧口径（& 分隔、无 noticeType）必须被拒绝，防止对方误用旧实现。
	legacy := signStr("sec", "APP1&O1&A1&success&10.00000000&1760851231&n1")
	if verifyNoticeSignature("sec", "APP1", []byte(paymentBody), legacy) {
		t.Fatal("旧签名口径不应通过")
	}
}

func TestVerifyNoticeRejectsTampering(t *testing.T) {
	sig := signStr("sec", "payment|APP1|O1|A1|success|10.00000000|1760851231|n1")
	cases := []string{
		strings.Replace(paymentBody, `"noticeType":"payment"`, `"noticeType":"refund"`, 1),
		strings.Replace(paymentBody, `"amount":"10.00000000"`, `"amount":"100.00000000"`, 1),
		strings.Replace(paymentBody, `"orderNo":"O1"`, `"orderNo":"O2"`, 1),
		`{"noticeType":"payment"}`,
	}
	for _, body := range cases {
		if verifyNoticeSignature("sec", "APP1", []byte(body), sig) {
			t.Fatalf("篡改后应失败: %s", body)
		}
	}
}

func TestCodeMatchesServer(t *testing.T) {
	// 与 walletGo internal/logic/open/notify_test.go 的固定向量一致。
	d := PaymentNotice{OrderNo: "O1", OutOrderNo: "A1", Status: "success", ChainName: "TRC20",
		Amount: "10.00000000", Timestamp: "1760851231", Nonce: "n1"}
	body := `{"noticeType":"payment","data":{"orderNo":"` + d.OrderNo + `","outOrderNo":"` + d.OutOrderNo +
		`","status":"` + d.Status + `","chainName":"` + d.ChainName + `","amount":"` + d.Amount +
		`","timestamp":"` + d.Timestamp + `","nonce":"` + d.Nonce + `"}}`
	if !verifyNoticeSignature("sec", "APP1", []byte(body), signStr("sec", "payment|APP1|O1|A1|success|10.00000000|1760851231|n1")) {
		t.Fatal("与服务端向量不一致")
	}
}

const refundBody = `{"noticeType":"refund","data":{"orderNo":"O1","outOrderNo":"A1","refundNo":"R1","outRefundNo":"RF1","outItemNo":"I3","status":"success","chainName":"TRC20","amount":"1.50000000","refundedAmount":"3.50000000","orderAmount":"10.00000000","reason":"r","errorMsg":"","timestamp":"1","nonce":"n"}}`

func TestVerifyNoticeRefund(t *testing.T) {
	sig := signStr("sec", "refund|APP1|O1|A1|R1|RF1|I3|success|1.50000000|1|n")
	if !verifyNoticeSignature("sec", "APP1", []byte(refundBody), sig) {
		t.Fatal("合法退款签名应通过")
	}
	if verifyNoticeSignature("sec", "APP1", []byte(strings.Replace(refundBody, `"status":"success"`, `"status":"failed"`, 1)), sig) {
		t.Fatal("篡改状态应失败")
	}
	p, err := NoticeRaw{Body: []byte(refundBody)}.Payload()
	if err != nil {
		t.Fatal(err)
	}
	rf, err := p.Refund()
	if err != nil || rf.RefundedAmount != "3.50000000" {
		t.Fatalf("got %+v %v", rf, err)
	}
	if _, err = p.Payment(); err == nil {
		t.Fatal("refund 体不应能按 payment 解析")
	}
}
