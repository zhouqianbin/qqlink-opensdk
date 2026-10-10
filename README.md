# QQLink 生态服务端 SDK（Go）

用于在**你自己的服务端**调用 QQLink 生态（`/open`）接口的 Go SDK。

> **本 SDK 只应运行在你的服务端。**
> `app_secret` 绝不能下发到网页或客户端——泄漏即可被用来伪造你的全部请求。

---

## 安装

```
go get github.com/zhouqianbin/qqlink-opensdk
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/zhouqianbin/qqlink-opensdk"
)

func main() {
	client, err := opensdk.New(opensdk.Config{
		BaseURL:   "https://api.qqlink.buzz",
		AppKey:    "oa_xxxxxxxx",   // 入驻时分配的公钥
		AppSecret: "os_xxxxxxxx",   // 入驻时分配的私钥，仅服务端持有
	})
	if err != nil {
		log.Fatal(err)
	}

	// 用授权码换访问令牌
	result, err := client.AuthToken(context.Background(), opensdk.AuthTokenRequest{
		Code: codeFromYourFrontend,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Openid, result.AccessToken)
}
```

签名、nonce、请求头全部由 SDK 处理，你只需填参数。

---

## 接入前提

你需要先在 QQLink 完成入驻，拿到：

| 项 | 说明 |
| --- | --- |
| `AppKey` | 公钥，对外可见 |
| `AppSecret` | 私钥，**仅服务端持有** |
| 应用入口地址 | 用户在 QQLink App 内打开你应用时加载的页面 |
| 通知地址 `notice_url` | 接收资金结果回调的服务端地址 |

---

## 接口一览

| 方法 | 用途 |
| --- | --- |
| `AuthToken` | 用授权码换访问令牌 |
| `AuthRefresh` | 刷新访问令牌 |
| `UserInfo` | 查询用户信息（实时昵称/头像） |
| `AuthRevoke` | 解除授权 |
| `PayCreate` | 下单（**不扣款**） |
| `PayQuery` | 订单查询（对账兜底） |
| `FriendList` | 查询用户好友（仅返回同样授权了你应用的部分） |

### 错误处理

非业务成功的响应返回 `*opensdk.APIError`，可用类型断言取错误码：

```go
result, err := client.PayCreate(ctx, req)
if err != nil {
	var apiErr *opensdk.APIError
	if errors.As(err, &apiErr) {
		// 业务错误，apiErr.Code / apiErr.Message
	}
}
```

---

## 支付是两段式的（重要）

**下单与扣款是两次独立的动作，且发起方不同：**

| 阶段 | 谁发起 | 是否动钱 |
| --- | --- | --- |
| 下单 `PayCreate` | **你的服务端** | **不动钱** |
| 确认支付 | **QQLink App**（用户输交易密码） | 动钱 |

这意味着：

1. **下单不需要传链路与币种**——币种默认 USDT，**链路由用户在 QQLink App 内选**（他有哪条链的余额就用哪条）。你无从得知，也无需指定。
2. **你拿不到支付结果的同步响应**。**你的订单状态必须以回调为准**（见下）。
3. **即使 `app_secret` 泄漏，攻击者也无法扣用户的钱**——扣款必须由用户本人在 QQLink App 内输交易密码触发。这是对双方的保护。

下单后，把返回的 `orderInfo` 交给你的**网页**，网页会通过桥接唤起 QQLink App 的支付确认框。

---

## 接收回调

支付成功后，QQLink 会 `POST` 到你的 `notice_url`，请求体见 `NoticePayload`。

**你的服务端必须返回**：

```json
{ "success": true }
```

不返回或返回 `success` 非 `true`，都会被判为未确认并**重投**。请按 `OrderNo` 做幂等。

### 校验回调签名

**必须用原始 body 字节验签**，不要先反序列化再重新序列化：

```go
func noticeHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)   // 原始字节

	ok := client.VerifyNotice(opensdk.NoticeRaw{
		Body:      body,
		AppKey:    r.Header.Get(opensdk.HeaderOpenKey),
		Signature: r.Header.Get(opensdk.HeaderSignature),
		Nonce:     r.Header.Get(opensdk.HeaderNonce),
		Timestamp: r.Header.Get(opensdk.HeaderTimestamp),
	})
	if !ok {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}

	// 验签通过后再反序列化处理
	payload, _ := (&opensdk.NoticeRaw{Body: body}).Payload()
	_ = payload

	_ = json.NewEncoder(w).Encode(opensdk.AckResponse{Success: true})
}
```

> 回调的签名口径与请求签名**不同**，SDK 已分别实现，不要混用。

---

## 用户身份：openid

用户身份用 **openid** 表示——它是该用户在你这个应用下的**脱敏标识**。

- 在你应用内唯一；**同一用户在不同应用拿到的 openid 不同**。
- 你**拿不到**用户的内部 uid，也无法枚举。
- **`AuthToken` 返回的昵称/头像是授权那一刻的快照**；`UserInfo` 返回的是**实时值**。两者时点不同，请按需选择。
- 认证状态（是否实名）始终是实时的。

---

## 相关文档

- 完整接入指南（授权流程、错误码、接入清单）请向 QQLink 对接人索取。
- 网页侧需要接入桥接（唤起授权与支付），参见前端 SDK 相关文档。

---

## 版本

当前为 `v0.x`，接口仍在演进，可能存在不兼容变更。稳定后会发布 `v1.0.0`。
