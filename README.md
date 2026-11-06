# QQLink 生态服务端 SDK（Go）

用于在**你自己的服务端**调用 QQLink 生态（`/open`）接口的 Go SDK。

> **本 SDK 只应运行在你的服务端。**
> `AppSecret` 绝不能下发到网页或客户端。
> 泄漏后攻击者可伪造你的全部 `/open` 请求，包括**提现、免确认退款、解授权、伪造消费记录**——
> 可能动用的是**你的应用结算户**，不是用户钱包里需输交易密码才能划走的余额。
> 请按支付级密钥保管（环境变量/密钥管理，勿入库、勿进日志）；入驻后如支持，请配置服务端 IP 白名单。

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
		BaseURL:   "https://api.qqlink.buzz", // 必须 https；本地 http 联调需设 InsecureAllowHTTP
		AppKey:    "oa_xxxxxxxx",             // 入驻时分配的公钥
		AppSecret: "os_xxxxxxxx",             // 入驻时分配的私钥，仅服务端持有
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

请求签名对原始 JSON body 做 HMAC-SHA256，待签串为
`v3\n{method}\n{path}\n{timestamp}\n{nonce}\n{appKey}\n{rawBody}`
（如 `POST` + `/open/pay/create`）。不要自行改写 body 后再算签。

**时间与时钟（重要）：**

- `timestamp` / 头 `X-Timestamp` 为 **Unix 秒**（自 1970-01-01 00:00:00 **UTC** 起算），与机器显示时区（如东八区）无关；SDK 使用 `time.Now().Unix()` 生成。
- 服务端按同样的 UTC Unix 秒校验，允许偏差 **±300 秒**。超出则拒绝（时间戳无效或已过期）。
- **双方主机都应 NTP 对齐到可靠时间源**。协议用的是 UTC 时间戳，但不保证某台机器时钟本身准确；若你方服务器快慢超过约 5 分钟，即使用对了签名算法也会失败。
- 一般**不需要**向我方拉取服务器时间；先做好 NTP。仅在校时后仍频繁因时间窗失败时，再联系对接人排查。

---

## 接入前提

你需要先在 QQLink 完成入驻，拿到：

| 项 | 说明 |
| --- | --- |
| `AppKey` | 公钥，对外可见 |
| `AppSecret` | 私钥，**仅服务端持有**；泄漏≈可动结算户资金，见文首说明 |
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
| `PayRefund` | 发起退款（部分/多次，用户免确认） |
| `PayRefundQuery` | 按退款单号查询退款 |
| `FriendList` | 查询用户好友（仅返回同样授权了你应用的部分） |
| `InviteCode` | 按 openid 查询用户邀请码 |
| `WithdrawCreate` | 发起提现（应用结算户 → 用户） |
| `WithdrawQuery` | 按我方提现单号查询 |
| `ConsumeSync` | 批量同步用户消费记录（消费返佣） |

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
3. **用户钱包扣款仍受交易密码保护**：即便 `AppSecret` 泄漏，攻击者也**不能**单靠接口直接划走用户余额——支付确认必须在 QQLink App 内由用户完成。
4. **但结算户不受这一保护**：同一把 `AppSecret` 即可发起 `WithdrawCreate` / `PayRefund`（用户免确认）等，泄漏后可能掏空或挪用**应用结算户**。保管与白名单要求见文首。

下单后，把返回的 `orderInfo` 交给你的**网页**，网页会通过桥接唤起 QQLink App 的支付确认框。

---

## 接收回调

支付 / 退款 / 提现结果，QQLink 会 `POST` 到你的 `notice_url`。回调分两层：顶层是 `noticeType`，业务字段放在 `data` 里，结构随类型不同。支付结果示例：

**你的服务端必须返回**：

```json
{
  "noticeType": "payment",
  "data": {
    "orderNo": "O1837...",
    "outOrderNo": "YOUR-ORDER-001",
    "status": "success",
    "chainName": "TRC20",
    "amount": "10.50000000",
    "timestamp": "1760851231",
    "nonce": "a1b2c3d4e5f60718"
  }
}
```

`data` 用 `NoticePayload.Payment()` 取；金额是固定 8 位小数的字符串。

**你的服务端必须返回**：

```json
{ "success": true }
```

不返回或返回 `success` 非 `true`，都会被判为未确认并**重投**。请按 `orderNo` 做幂等。

### 校验回调签名

**必须用原始 body 字节验签**，不要先反序列化再重新序列化。

**`VerifyNotice` 只验签，不防重放：**

- 通过 = **原始 body** 未被篡改，且签名由持有 `AppSecret` 的一方生成（待签串 `notice\n{appKey}\n{rawBody}`）。
- **不**校验 `data.timestamp` 是否过期，**不**消费 `nonce`；同一条合法回调再次 POST 时仍可能验签成功。
- 请求头里的 `X-Nonce` / `X-Timestamp` / `X-Open-Key` 即使填进 `NoticeRaw` 也不参与判断（以 Body + `X-Signature` 为准；appKey 用客户端配置）。
- 你必须在业务层幂等：支付按 `orderNo`、退款按 `refundNo`（或 `outRefundNo`）、提现按 `withdrawNo`（或 `outOrderNo`）去重后再改状态/入账。平台未收到 `{success:true}` 时会重投，幂等同样必要。

```go
func noticeHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)   // 原始字节

	ok := client.VerifyNotice(opensdk.NoticeRaw{
		Body:      body,
		Signature: r.Header.Get(opensdk.HeaderSignature),
	})
	if !ok {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}

	// 验签通过后再反序列化；处理业务前先按单号幂等去重
	payload, _ := opensdk.NoticeRaw{Body: body}.Payload()
	switch payload.NoticeType {
	case opensdk.NoticeTypePayment:
		notice, err := payload.Payment()
		if err == nil {
			_ = notice.OrderNo // 已处理过则直接 ack，勿重复入账
		}
	case opensdk.NoticeTypeRefund:
		notice, err := payload.Refund()
		if err == nil {
			_ = notice.RefundNo
		}
	case opensdk.NoticeTypeWithdraw:
		notice, err := payload.Withdraw()
		if err == nil {
			_ = notice.WithdrawNo
		}
	}

	_ = json.NewEncoder(w).Encode(opensdk.AckResponse{Success: true})
}
```

> 回调与请求都签**原始 body 字节**，但前缀不同：请求为
> `v3\n{method}\n{path}\n{timestamp}\n{nonce}\n{appKey}\n{rawBody}`，
> 回调为 `notice\n{appKey}\n{rawBody}`（`SignNotice` / `VerifyNotice`）。不要混用，也不要先改写 body 再验签。

---

## 退款

退款由**你的服务端**发起，`PayRefund`，**用户无需确认**；资金从你的应用结算户按原支付链、原币种退回付款用户。一个订单可以多次部分退款，累计退款不能超过订单金额。因此 `AppSecret` 必须按可动用结算资金的密钥级别保管。

- `outRefundNo` 是幂等键：同一退款单号重复提交返回第一次的结果。
- 结算户余额不足时接口返回错误码 `30006`（应用账户余额不足），退款记录落为 `failed`，换新的 `outRefundNo` 重试。
- 成功和失败都会回调，`noticeType` 为 `refund`：

```json
{
  "noticeType": "refund",
  "data": {
    "orderNo": "O1837...",
    "outOrderNo": "YOUR-ORDER-001",
    "refundNo": "R1837...",
    "outRefundNo": "RF-0001",
    "outItemNo": "ITEM-03",
    "status": "success",
    "chainName": "TRC20",
    "amount": "1.50000000",
    "refundedAmount": "3.50000000",
    "orderAmount": "10.00000000",
    "reason": "用户申请部分退款",
    "errorMsg": "",
    "timestamp": "1760851231",
    "nonce": "a1b2c3d4e5f60718"
  }
}
```

验签与支付回调同一套流程（`VerifyNotice` 对原始 body）；取 data 用 `payload.Refund()`。

请按 `refundNo`（或 `outRefundNo`）幂等处理。

## 提现

提现由**你的服务端**发起，`WithdrawCreate`：从你的应用结算户把金额打到指定 `openid` 对应用户（**仅需 `AppSecret`，无用户二次确认**）。成功和失败都会回调，`noticeType` 为 `withdraw`。

- `outOrderNo` 是幂等键：同一提现单号重复提交返回第一次的结果。
- 超出单笔或当日限额时返回错误码 `30008`。
- 结算户余额不足时记录落为 `failed`，并通过回调通知。
- 可用 `WithdrawQuery` 按我方 `withdrawNo` 查询兜底。

```json
{
  "noticeType": "withdraw",
  "data": {
    "withdrawNo": "W1837...",
    "outOrderNo": "YOUR-WITHDRAW-001",
    "status": "success",
    "chainName": "TRC20",
    "amount": "5.00000000",
    "errorMsg": "",
    "timestamp": "1760851231",
    "nonce": "a1b2c3d4e5f60718"
  }
}
```

取 data 用 `payload.Withdraw()`；验签仍用原始 body 的 `VerifyNotice`。

请按 `withdrawNo`（或 `outOrderNo`）幂等处理。

## 消费同步（返佣）

用 `ConsumeSync` 批量上报用户消费记录，单次最多 50 条。`(openid, outOrderNo)` 是幂等键：同一单号重复提交返回原单，不会重复建单。

- 金额、手续费均为字符串，最多 8 位小数；`fee` 传空串按 0。
- `consumeTime` 支持 `2006-01-02 15:04:05` 或毫秒时间戳。
- 返回逐条结果，顺序与请求一致，状态一般为 `pending`。

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

历史不兼容变更：回调体改为 `{noticeType, data}` 两层；`PayCreate` 的金额改为字符串；回调签名现为对原始 body 的 HMAC（见上文）。

请求签名改为对原始 body 做 HMAC，并纳入 method/path（待签串
`v3\n{method}\n{path}\n{timestamp}\n{nonce}\n{appKey}\n{rawBody}`），校验 ±300s 时间窗。
回调签名改为对原始 body 做 HMAC（`notice\n{appKey}\n{rawBody}`），不再用 `|` 拼字段。
