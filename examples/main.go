// 本文件演示如何在自有服务端使用该 SDK。
// 它不在 SDK 包内，引用时请把 module 路径换成你自己的。
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/zhouqianbin/qqlink-opensdk"
)

func main() {
	client, err := opensdk.New(opensdk.Config{
		BaseURL:   "https://api.qqlink.buzz",
		AppKey:    "oa_xxxxxxxx",
		AppSecret: "os_xxxxxxxx",
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// 1. 用授权码换令牌（授权码由你方网页经 QQLink App 桥接取得后传到你的服务端）
	token, err := client.AuthToken(ctx, opensdk.AuthTokenRequest{Code: "wallet_code_xxxxxxxx"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("openid=%s accessToken=%s\n", token.Openid, token.AccessToken)

	// 2. 下单（不扣款）
	order, err := client.PayCreate(ctx, opensdk.PayCreateRequest{
		Openid:     token.Openid,
		OutOrderNo: "YOUR-ORDER-001", // 你的业务单号，用于幂等
		Amount:     "10.50000000", // 字符串，最多 8 位小数
	})
	if err != nil {
		log.Fatal(err)
	}
	// order.OrderInfo 交给网页，由网页通过桥接唤起 QQLink App 的支付确认框
	fmt.Printf("orderNo=%s amount=%s\n", order.OrderNo, order.Amount)

	// 3. 查询订单（对账兜底）
	query, err := client.PayQuery(ctx, opensdk.PayQueryRequest{OrderNo: order.OrderNo})
	if err != nil {
		// 业务错误可断言出错误码
		var apiErr *opensdk.APIError
		if errors.As(err, &apiErr) {
			fmt.Printf("api error code=%d message=%s\n", apiErr.Code, apiErr.Message)
		}
		return
	}
	fmt.Printf("status=%s chainName=%s\n", query.Status, query.ChainName)

	// 4. 部分退款（用户免确认）；成功与失败都会以 noticeType=refund 回调
	refund, err := client.PayRefund(ctx, opensdk.PayRefundRequest{
		OutOrderNo:  "YOUR-ORDER-001",
		OutRefundNo: "YOUR-REFUND-001",
		Amount:      "1.50000000",
		Reason:      "部分退款",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("refundNo=%s status=%s refunded=%s\n", refund.RefundNo, refund.Status, refund.RefundedAmount)

	// 5. 查询好友（仅返回同样授权了本应用的部分）
	friends, err := client.FriendList(ctx, opensdk.FriendListRequest{Openid: token.Openid})
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range friends.List {
		fmt.Printf("friend openid=%s nickname=%s\n", f.Openid, f.Nickname)
	}

	// 6. 查询邀请码
	invite, err := client.InviteCode(ctx, opensdk.InviteCodeRequest{Openid: token.Openid})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("inviteCode=%s\n", invite.InviteCode)
}
