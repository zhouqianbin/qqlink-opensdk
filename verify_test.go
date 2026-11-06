package opensdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const paymentBody = `{"noticeType":"payment","data":{"orderNo":"O1","outOrderNo":"A1","status":"success","chainName":"TRC20","amount":"10.00000000","timestamp":"1760851231","nonce":"n1"}}`

const refundBody = `{"noticeType":"refund","data":{"orderNo":"O1","outOrderNo":"A1","refundNo":"R1","outRefundNo":"RF1","outItemNo":"I3","status":"success","chainName":"TRC20","amount":"1.50000000","refundedAmount":"3.50000000","orderAmount":"10.00000000","reason":"r|x","errorMsg":"","timestamp":"1","nonce":"n"}}`

const withdrawBody = `{"noticeType":"withdraw","data":{"withdrawNo":"W1","outOrderNo":"WO1","status":"success","chainName":"TRC20","amount":"5.00000000","errorMsg":"","timestamp":"1","nonce":"n"}}`

func TestVerifyNoticePayment(t *testing.T) {
	sig := SignNotice("sec", "APP1", []byte(paymentBody))
	if !verifyNoticeSignature("sec", "APP1", []byte(paymentBody), sig) {
		t.Fatal("合法签名应通过")
	}
	if verifyNoticeSignature("sec", "APP2", []byte(paymentBody), sig) {
		t.Fatal("appKey 不同应失败")
	}
	// 旧 "|" 字段拼接口径（HMAC(secret, "payment|APP1|...")）必须被拒绝。
	mac := hmac.New(sha256.New, []byte("sec"))
	mac.Write([]byte("payment|APP1|O1|A1|success|TRC20|10.00000000|1760851231|n1"))
	legacy := hex.EncodeToString(mac.Sum(nil))
	if verifyNoticeSignature("sec", "APP1", []byte(paymentBody), legacy) {
		t.Fatal("旧签名口径不应通过")
	}
}

func TestVerifyNoticeRejectsTampering(t *testing.T) {
	sig := SignNotice("sec", "APP1", []byte(paymentBody))
	cases := []string{
		strings.Replace(paymentBody, `"noticeType":"payment"`, `"noticeType":"refund"`, 1),
		strings.Replace(paymentBody, `"amount":"10.00000000"`, `"amount":"100.00000000"`, 1),
		strings.Replace(paymentBody, `"orderNo":"O1"`, `"orderNo":"O2"`, 1),
		strings.Replace(paymentBody, `"chainName":"TRC20"`, `"chainName":"ERC20"`, 1),
		`{"noticeType":"payment"}`,
	}
	for _, body := range cases {
		if verifyNoticeSignature("sec", "APP1", []byte(body), sig) {
			t.Fatalf("篡改后应失败: %s", body)
		}
	}
}

func TestVerifyNoticeRefundWithPipeInReason(t *testing.T) {
	// reason 含 "|" 时旧拼接口径会歧义；body 签名不受影响。
	sig := SignNotice("sec", "APP1", []byte(refundBody))
	if !verifyNoticeSignature("sec", "APP1", []byte(refundBody), sig) {
		t.Fatal("合法退款签名应通过")
	}
	tampered := strings.Replace(refundBody, `"reason":"r|x"`, `"reason":"r|y"`, 1)
	if verifyNoticeSignature("sec", "APP1", []byte(tampered), sig) {
		t.Fatal("篡改 reason 应失败")
	}
	p, err := NoticeRaw{Body: []byte(refundBody)}.Payload()
	if err != nil {
		t.Fatal(err)
	}
	rf, err := p.Refund()
	if err != nil || rf.RefundedAmount != "3.50000000" || rf.Reason != "r|x" {
		t.Fatalf("got %+v %v", rf, err)
	}
}

func TestVerifyNoticeWithdraw(t *testing.T) {
	sig := SignNotice("sec", "APP1", []byte(withdrawBody))
	if !verifyNoticeSignature("sec", "APP1", []byte(withdrawBody), sig) {
		t.Fatal("合法提现签名应通过")
	}
	if verifyNoticeSignature("sec", "APP1", []byte(strings.Replace(withdrawBody, `"status":"success"`, `"status":"failed"`, 1)), sig) {
		t.Fatal("篡改状态应失败")
	}
	p, err := NoticeRaw{Body: []byte(withdrawBody)}.Payload()
	if err != nil {
		t.Fatal(err)
	}
	w, err := p.Withdraw()
	if err != nil || w.WithdrawNo != "W1" {
		t.Fatalf("got %+v %v", w, err)
	}
}

func TestCodeMatchesServerNoticeVector(t *testing.T) {
	// 与 walletGo notify_test 固定向量一致。
	want := SignNotice("sec", "APP1", []byte(paymentBody))
	if !verifyNoticeSignature("sec", "APP1", []byte(paymentBody), want) {
		t.Fatal("与服务端向量不一致")
	}
}
