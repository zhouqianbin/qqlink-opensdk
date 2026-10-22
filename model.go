package opensdk

import (
	"encoding/json"
	"fmt"
)

// 本文件是 SDK 的请求/响应模型，与 /open 接口一一对应。
//
// 服务端返回统一外壳 { errCode, errMsg, resData }，SDK 已解开外壳，
// 调用方直接拿到下面的 Res 结构；失败时返回 *APIError。

// ---------- 授权 ----------

// AuthTokenRequest 用授权码换访问令牌。
// 应用身份由 SDK 通过 X-Open-Key 头携带，无需在参数里传。
type AuthTokenRequest struct {
	Code string `json:"code"`
}

type AuthTokenResult struct {
	AccessToken  string `json:"accessToken"`  // 访问令牌，有效期 20 分钟
	ExpiresIn    int    `json:"expiresIn"`    // 剩余有效秒数
	RefreshToken string `json:"refreshToken"` // 刷新令牌
	Openid       string `json:"openid"`       // 用户在该应用下的标识
	Nickname     string `json:"nickname"`     // 昵称快照（授权那一刻）
	Avatar       string `json:"avatar"`       // 头像快照（授权那一刻）
}

// AuthRefreshRequest 刷新访问令牌。
type AuthRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type AuthRefreshResult struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
}

// UserInfoRequest 查询用户信息，返回实时资料。
type UserInfoRequest struct {
	AccessToken string `json:"accessToken"`
}

type UserInfoResult struct {
	Openid    string `json:"openid"`
	Nickname  string `json:"nickname"` // 实时昵称
	Avatar    string `json:"avatar"`   // 实时头像
	Certified bool   `json:"certified"`
}

// AuthRevokeRequest 解除授权。
type AuthRevokeRequest struct {
	Openid string `json:"openid"`
}

type AuthRevokeResult struct {
	Ok bool `json:"ok"`
}

// ---------- 支付 ----------

// PayCreateRequest 下单，不扣款。
//
// 不需要传链路与币种：币种默认 USDT，链路由用户在 App 内确认支付时自己选。
// 金额为字符串，最多 8 位小数，例如 "10.50"；用字符串避免浮点误差。
type PayCreateRequest struct {
	Openid     string `json:"openid"`
	OutOrderNo string `json:"outOrderNo"` // 你的业务单号，用于幂等
	Amount     string `json:"amount"`     // 金额，最多 8 位小数
}

type PayCreateResult struct {
	OrderNo   string `json:"orderNo"`   // 我方单号
	Amount    string `json:"amount"`    // 订单金额，8 位小数
	OrderInfo string `json:"orderInfo"` // 订单信息，交给网页用于唤起支付
}

// PayQueryRequest 订单查询，对账兜底。只支持按我方单号查询。
type PayQueryRequest struct {
	OrderNo string `json:"orderNo"`
}

type PayQueryResult struct {
	OrderNo     string `json:"orderNo"`
	OutOrderNo  string `json:"outOrderNo"`
	Status      string `json:"status"`
	Amount      string `json:"amount"`
	ChainName   string `json:"chainName"`
	CompletedAt string `json:"completedAt"`
}

// PayRefundRequest 发起退款，按聚合单逐笔部分退，用户免确认。
// 资金从你的应用结算户退回付款用户；用户在哪里选的链、付的币种，就按原路退回。
type PayRefundRequest struct {
	OutOrderNo  string `json:"outOrderNo"`
	OutRefundNo string `json:"outRefundNo"` // 你的退款单号，幂等键
	OutItemNo   string `json:"outItemNo"`   // 可选，子项单号，原样透传
	Amount      string `json:"amount"`      // 金额
	Reason      string `json:"reason"`
}

type PayRefundResult struct {
	RefundNo       string `json:"refundNo"`
	OutRefundNo    string `json:"outRefundNo"`
	OutOrderNo     string `json:"outOrderNo"`
	OrderNo        string `json:"orderNo"`
	Status         string `json:"status"`         // success / failed
	Amount         string `json:"amount"`         // 本次退款金额
	RefundedAmount string `json:"refundedAmount"` // 订单累计已退（含本次）
	OrderAmount    string `json:"orderAmount"`
	SucceededAt    string `json:"succeededAt"`
	ErrorMsg       string `json:"errorMsg"`
}

// PayRefundQueryRequest 按第三方退款单号查询。
type PayRefundQueryRequest struct {
	OutRefundNo string `json:"outRefundNo"`
}

type PayRefundQueryResult = PayRefundResult

// ---------- 用户数据 ----------

// FriendListRequest 查询指定用户的好友中同样授权了本应用的部分。
type FriendListRequest struct {
	Openid string `json:"openid"`
}

type FriendListResult struct {
	List []FriendItem `json:"list"`
}

type FriendItem struct {
	Openid   string `json:"openid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// InviteCodeRequest 按 openid 查询该用户的邀请码。
// openid 须属于本应用（已授权且有效）。
type InviteCodeRequest struct {
	Openid string `json:"openid"` // 用户在本应用下的标识
}

// InviteCodeResult 邀请码查询结果。
type InviteCodeResult struct {
	InviteCode string `json:"inviteCode"` // 用户邀请码；用户不存在或未生成时为空串
}

// ---------- 回调 ----------

// NoticePayload 是我方资金结果回调的请求体。
//
// 回调分两层：顶层只有 noticeType 与 data，data 的结构随 noticeType 变化。
// 你的服务端收到回调后应返回 AckResponse。
type NoticePayload struct {
	NoticeType string          `json:"noticeType"` // 通知类型：payment
	Data       json.RawMessage `json:"data"`       // 按 noticeType 对应的结构
}

// 回调通知类型，对应信封顶层 noticeType。
const (
	NoticeTypePayment = "payment" // 支付结果
	NoticeTypeRefund  = "refund"  // 退款终态（成功或失败）
)

// PaymentNotice 是支付结果回调的 data 段。
// Amount 固定 8 位小数，例如 "10.00000000"。
type PaymentNotice struct {
	OrderNo    string `json:"orderNo"`    // 我方单号
	OutOrderNo string `json:"outOrderNo"` // 你的业务单号
	Status     string `json:"status"`     // 订单状态：success
	ChainName  string `json:"chainName"`  // 链路
	Amount     string `json:"amount"`     // 金额，8 位小数
	Timestamp  string `json:"timestamp"`
	Nonce      string `json:"nonce"`
}

// Payment 取出支付回调的 data；通知类型不是 payment 时返回错误。
func (p *NoticePayload) Payment() (*PaymentNotice, error) {
	if p.NoticeType != NoticeTypePayment {
		return nil, fmt.Errorf("opensdk: 未知通知类型 %q", p.NoticeType)
	}
	out := &PaymentNotice{}
	if err := json.Unmarshal(p.Data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// RefundNotice 是退款结果回调的 data 段。
type RefundNotice struct {
	OrderNo        string `json:"orderNo"`
	OutOrderNo     string `json:"outOrderNo"`
	RefundNo       string `json:"refundNo"`
	OutRefundNo    string `json:"outRefundNo"`
	OutItemNo      string `json:"outItemNo"`
	Status         string `json:"status"` // success / failed
	ChainName      string `json:"chainName"`
	Amount         string `json:"amount"`
	RefundedAmount string `json:"refundedAmount"` // 订单累计已退（含本次）
	OrderAmount    string `json:"orderAmount"`
	Reason         string `json:"reason"`
	ErrorMsg       string `json:"errorMsg"`
	Timestamp      string `json:"timestamp"`
	Nonce          string `json:"nonce"`
}

// Refund 取出退款回调的 data；通知类型不是 refund 时返回错误。
func (p *NoticePayload) Refund() (*RefundNotice, error) {
	if p.NoticeType != NoticeTypeRefund {
		return nil, fmt.Errorf("opensdk: 通知类型不是 refund：%q", p.NoticeType)
	}
	out := &RefundNotice{}
	if err := json.Unmarshal(p.Data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// noticeFields 返回参与回调签名的字段，timestamp、nonce 固定在末尾两位。
// 新增通知类型时在这里加分支（与请求端口径无关，勿复用 Sign）。
func (p *NoticePayload) noticeFields() ([]string, error) {
	switch p.NoticeType {
	case NoticeTypePayment:
		n, err := p.Payment()
		if err != nil {
			return nil, err
		}
		return []string{n.OrderNo, n.OutOrderNo, n.Status, n.Amount, n.Timestamp, n.Nonce}, nil
	case NoticeTypeRefund:
		r, err := p.Refund()
		if err != nil {
			return nil, err
		}
		return []string{r.OrderNo, r.OutOrderNo, r.RefundNo, r.OutRefundNo, r.OutItemNo,
			r.Status, r.Amount, r.Timestamp, r.Nonce}, nil
	}
	return nil, fmt.Errorf("opensdk: 未知通知类型 %q", p.NoticeType)
}

// AckResponse 是回调必须返回的确认结构。
// 只有 Success 为 true 才算 ack 成功，否则我方会重投。
type AckResponse struct {
	Success bool `json:"success"`
}

// 回调请求头名称，供调用方取值（与 body 内字段同值，验签以 body 为准）。
const (
	HeaderOpenKey   = "X-Open-Key"
	HeaderSignature = "X-Signature"
	HeaderNonce     = "X-Nonce"
	HeaderTimestamp = "X-Timestamp"
)

// NoticeRaw 是回调的原始请求内容，供验签使用。
//
// Body 必须是**未经改动的原始字节**：验签会从中解析出 noticeType 与 data 各字段做拼接，
// 若你自行改写或重新序列化 body，字段值可能与签名时不一致，验签会失败。
type NoticeRaw struct {
	Body      []byte
	AppKey    string
	Signature string
	Nonce     string
	Timestamp string
}

// 便于调用方直接反序列化回调 body。
func (r NoticeRaw) Payload() (*NoticePayload, error) {
	p := &NoticePayload{}
	if err := json.Unmarshal(r.Body, p); err != nil {
		return nil, err
	}
	return p, nil
}
