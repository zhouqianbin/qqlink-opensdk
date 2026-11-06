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

// ---------- 提现 ----------

// WithdrawCreateRequest 发起提现，应用结算户 → openid 对应用户。
type WithdrawCreateRequest struct {
	Openid     string `json:"openid"`     // 收款用户标识，须属于本应用
	OutOrderNo string `json:"outOrderNo"` // 你的提现单号，用于幂等
	Amount     string `json:"amount"`     // 金额，最多 8 位小数
}

// WithdrawQueryRequest 按我方提现单号查询。
type WithdrawQueryRequest struct {
	WithdrawNo string `json:"withdrawNo"` // 我方提现单号
}

// WithdrawResult 是单笔提现的对外结构，发起与查询共用。
type WithdrawResult struct {
	WithdrawNo  string `json:"withdrawNo"`  // 我方提现单号
	OutOrderNo  string `json:"outOrderNo"`  // 你的提现单号
	Status      string `json:"status"`      // success / failed
	Amount      string `json:"amount"`      // 提现金额，8 位小数
	ChainName   string `json:"chainName"`   // 实际链路
	SucceededAt string `json:"succeededAt"` // 成功时间
	ErrorMsg    string `json:"errorMsg"`    // 失败原因
}

// ---------- 消费 ----------

// ConsumeSyncRequest 批量同步用户消费记录。
// 记录放在 list 字段里；v2 对整段 JSON body 签名，嵌套结构不再单独格式化。
type ConsumeSyncRequest struct {
	List []ConsumeSyncItem `json:"list"` // 单次最多 50 条
}

// ConsumeSyncItem 一条消费记录。
// (openid, outOrderNo) 是幂等键：同一单号重复提交返回原单，不会重复建单。
type ConsumeSyncItem struct {
	Openid      string `json:"openid"`      // 用户在本应用下的标识
	OutOrderNo  string `json:"outOrderNo"`  // 消费单号
	Amount      string `json:"amount"`      // 消费金额，最多 8 位小数
	Fee         string `json:"fee"`         // 手续费，只记录不扣款；传空串按 0
	ConsumeTime string `json:"consumeTime"` // 2006-01-02 15:04:05 或毫秒时间戳
	Type        string `json:"type"`        // 消费类型
	Remark      string `json:"remark"`      // 备注

	// 字段一律不加 omitempty：缺字段会改变 JSON body，从而改变 v2 签名。
}

// ConsumeSyncResult 批量同步的逐条结果，顺序与请求一致。
type ConsumeSyncResult struct {
	List []ConsumeSyncItemResult `json:"list"`
}

type ConsumeSyncItemResult struct {
	Index      int    `json:"index"`      // 对应请求下标，从 0 开始
	OutOrderNo string `json:"outOrderNo"` // 消费单号
	OrderNo    string `json:"orderNo"`    // 我方消费单号
	Status     string `json:"status"`     // pending 待处理
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
	NoticeTypePayment  = "payment"  // 支付结果
	NoticeTypeRefund   = "refund"   // 退款终态（成功或失败）
	NoticeTypeWithdraw = "withdraw" // 提现终态（成功或失败）
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

// WithdrawNotice 是提现结果回调的 data 段。
type WithdrawNotice struct {
	WithdrawNo string `json:"withdrawNo"` // 我方提现单号
	OutOrderNo string `json:"outOrderNo"` // 你的提现单号
	Status     string `json:"status"`     // success / failed
	ChainName  string `json:"chainName"`  // 实际链路
	Amount     string `json:"amount"`     // 金额，8 位小数
	ErrorMsg   string `json:"errorMsg"`   // 失败原因
	Timestamp  string `json:"timestamp"`
	Nonce      string `json:"nonce"`
}

// Withdraw 取出提现回调的 data；通知类型不是 withdraw 时返回错误。
func (p *NoticePayload) Withdraw() (*WithdrawNotice, error) {
	if p.NoticeType != NoticeTypeWithdraw {
		return nil, fmt.Errorf("opensdk: 通知类型不是 withdraw：%q", p.NoticeType)
	}
	out := &WithdrawNotice{}
	if err := json.Unmarshal(p.Data, out); err != nil {
		return nil, err
	}
	return out, nil
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
// Body 必须是**未经改动的原始字节**：VerifyNotice 对 Body 做 HMAC（SignNotice），
// 若你自行改写或重新序列化 body，验签会失败。
//
// AppKey / Nonce / Timestamp 便于排查或与头字段对照；VerifyNotice 当前只使用 Body 与 Signature
//（appKey 取自客户端配置）。防重放须自行按业务单号幂等，见 VerifyNotice 文档。
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
