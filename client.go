// Package opensdk 是 QQLink 生态（/open）接口的服务端 SDK，供接入方在自己的服务端使用。
//
// 用法：
//
//	client, err := opensdk.New(opensdk.Config{
//	    BaseURL:   "https://api.qqlink.buzz",
//	    AppKey:    "oa_xxx",
//	    AppSecret: "os_xxx",
//	})
//	result, err := client.AuthToken(ctx, opensdk.AuthTokenRequest{Code: code})
//
// 本 SDK 只应运行在**你的服务端**：AppSecret 绝不能下发到网页或客户端。
// 泄漏后可伪造提现/退款等请求并动用应用结算户，详见 README。
package opensdk
import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config 是接入参数。
type Config struct {
	BaseURL   string        // 我方服务地址，如 https://api.qqlink.buzz
	AppKey    string        // 应用公钥（AK）
	AppSecret string        // 应用私钥（SK），仅服务端持有
	Timeout   time.Duration // 请求超时，默认 10 秒

	// InsecureAllowHTTP 仅用于本地联调：允许 http:// BaseURL。
	// 生产环境必须使用 https，勿开启。
	InsecureAllowHTTP bool
}

// Client 封装 /open 接口的签名与调用。
type Client struct {
	cfg  Config
	http *http.Client
}

// maxResponseBody 是单次响应体读取上限，防止异常大包撑爆内存。
const maxResponseBody = 2 << 20 // 2 MiB

// maxErrorBodySnippet 是写入 error 字符串的响应摘要长度。
const maxErrorBodySnippet = 256

// New 创建客户端。
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.AppKey == "" || cfg.AppSecret == "" {
		return nil, errors.New("opensdk: BaseURL/AppKey/AppSecret 均为必填")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("opensdk: BaseURL 非法")
	}
	if u.Scheme != "https" && !cfg.InsecureAllowHTTP {
		return nil, errors.New("opensdk: BaseURL 必须使用 https（本地调试可设 InsecureAllowHTTP）")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	// 禁止自动跟随重定向：默认 Client 会把 X-Signature 等自定义头带到 Location，
	// 若 BaseURL 被劫持或返回跨站 3xx，签名材料可能泄露。
	return &Client{cfg: cfg, http: &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("opensdk: 拒绝跟随 HTTP 重定向")
		},
	}}, nil
}

// ---------- 对外接口 ----------

// AuthToken 用授权码换访问令牌。
func (c *Client) AuthToken(ctx context.Context, in AuthTokenRequest) (*AuthTokenResult, error) {
	out := &AuthTokenResult{}
	return out, c.call(ctx, "/open/auth/token", in, out)
}

// AuthRefresh 刷新访问令牌。
func (c *Client) AuthRefresh(ctx context.Context, in AuthRefreshRequest) (*AuthRefreshResult, error) {
	out := &AuthRefreshResult{}
	return out, c.call(ctx, "/open/auth/refresh", in, out)
}

// UserInfo 查询用户信息（实时昵称/头像）。
func (c *Client) UserInfo(ctx context.Context, in UserInfoRequest) (*UserInfoResult, error) {
	out := &UserInfoResult{}
	return out, c.call(ctx, "/open/user/info", in, out)
}

// AuthRevoke 解除授权。
func (c *Client) AuthRevoke(ctx context.Context, in AuthRevokeRequest) (*AuthRevokeResult, error) {
	out := &AuthRevokeResult{}
	return out, c.call(ctx, "/open/auth/revoke", in, out)
}

// PayCreate 下单（不扣款）。
func (c *Client) PayCreate(ctx context.Context, in PayCreateRequest) (*PayCreateResult, error) {
	out := &PayCreateResult{}
	return out, c.call(ctx, "/open/pay/create", in, out)
}

// PayQuery 订单查询，对账兜底。
func (c *Client) PayQuery(ctx context.Context, in PayQueryRequest) (*PayQueryResult, error) {
	out := &PayQueryResult{}
	return out, c.call(ctx, "/open/pay/query", in, out)
}

// PayRefund 发起退款。
func (c *Client) PayRefund(ctx context.Context, in PayRefundRequest) (*PayRefundResult, error) {
	out := &PayRefundResult{}
	return out, c.call(ctx, "/open/pay/refund", in, out)
}

// PayRefundQuery 查询退款。
func (c *Client) PayRefundQuery(ctx context.Context, in PayRefundQueryRequest) (*PayRefundQueryResult, error) {
	out := &PayRefundQueryResult{}
	return out, c.call(ctx, "/open/pay/refundQuery", in, out)
}

// FriendList 查询用户好友中同样授权了本应用的部分。
func (c *Client) FriendList(ctx context.Context, in FriendListRequest) (*FriendListResult, error) {
	out := &FriendListResult{}
	return out, c.call(ctx, "/open/user/friends", in, out)
}

// InviteCode 按 openid 查询用户邀请码。
func (c *Client) InviteCode(ctx context.Context, in InviteCodeRequest) (*InviteCodeResult, error) {
	out := &InviteCodeResult{}
	return out, c.call(ctx, "/open/user/inviteCode", in, out)
}

// WithdrawCreate 发起提现：应用结算户 → openid 对应用户。
func (c *Client) WithdrawCreate(ctx context.Context, in WithdrawCreateRequest) (*WithdrawResult, error) {
	out := &WithdrawResult{}
	return out, c.call(ctx, "/open/withdraw/create", in, out)
}

// WithdrawQuery 按我方提现单号查询。
func (c *Client) WithdrawQuery(ctx context.Context, in WithdrawQueryRequest) (*WithdrawResult, error) {
	out := &WithdrawResult{}
	return out, c.call(ctx, "/open/withdraw/query", in, out)
}

// ConsumeSync 批量同步用户消费记录（消费返佣）。
func (c *Client) ConsumeSync(ctx context.Context, in ConsumeSyncRequest) (*ConsumeSyncResult, error) {
	out := &ConsumeSyncResult{}
	return out, c.call(ctx, "/open/consume/sync", in, out)
}

// ---------- 内部实现 ----------

// response 是我方统一响应外壳。
type response struct {
	ErrCode int             `json:"ErrCode"`
	ErrMsg  string          `json:"ErrMsg"`
	ResData json.RawMessage `json:"ResData"`
}

// APIError 表示服务端返回的业务错误。
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("open api error %d: %s", e.Code, e.Message)
}

// call 完成一次签名请求，解开响应外壳后写入 out。
func (c *Client) call(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}

	nonce, err := newNonce()
	if err != nil {
		return err
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderOpenKey, c.cfg.AppKey)
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, SignBody(c.cfg.AppSecret, body, http.MethodPost, path, c.cfg.AppKey, nonce, timestamp))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return err
	}
	if len(respBody) > maxResponseBody {
		return fmt.Errorf("opensdk: 响应体超过 %d 字节上限", maxResponseBody)
	}

	// 签名失败等鉴权错误由中间件直接返回，HTTP 状态码可能非 200，先看状态码。
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opensdk: http status %d body=%s", resp.StatusCode, truncateForError(respBody))
	}

	r := &response{}
	if err = json.Unmarshal(respBody, r); err != nil {
		return fmt.Errorf("opensdk: 解析响应失败: %w body=%s", err, truncateForError(respBody))
	}
	if r.ErrCode != 0 {
		return &APIError{Code: r.ErrCode, Message: r.ErrMsg}
	}
	if out == nil {
		return nil
	}
	if len(r.ResData) == 0 || string(r.ResData) == "null" {
		return nil
	}
	return json.Unmarshal(r.ResData, out)
}

// SignParams 将 in JSON 序列化后按指定 path 签名（method 固定为 POST，与 call 一致）。
// path 须与实际请求路径一致，如 "/open/pay/create"；timestamp 为 Unix 秒字符串。
func (c *Client) SignParams(path string, in any, nonce, timestamp string) string {
	body, err := json.Marshal(in)
	if err != nil {
		return ""
	}
	return SignBody(c.cfg.AppSecret, body, http.MethodPost, path, c.cfg.AppKey, nonce, timestamp)
}

// SignBody 对原始 HTTP body 字节做 HMAC-SHA256（hex）。
//
// 待签串（字面量换行分隔）：
//
//	v3\n{method}\n{path}\n{timestamp}\n{nonce}\n{appKey}\n{rawBody}
//
// method 一般为 "POST"；path 为 URL 路径（不含 query），如 "/open/pay/create"。
// timestamp 为 Unix 秒（UTC 纪元），须与头 X-Timestamp 一致；服务端校验 ±300 秒。
// 双方主机应 NTP 校时。防重放另靠 nonce 去重与业务幂等。
func SignBody(appSecret string, body []byte, method, path, appKey, nonce, timestamp string) string {
	var b strings.Builder
	b.Grow(3 + len(method) + len(path) + len(timestamp) + len(nonce) + len(appKey) + len(body) + 6)
	b.WriteString("v3\n")
	b.WriteString(method)
	b.WriteByte('\n')
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString(timestamp)
	b.WriteByte('\n')
	b.WriteString(nonce)
	b.WriteByte('\n')
	b.WriteString(appKey)
	b.WriteByte('\n')
	b.Write(body)

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(b.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyNotice 校验回调签名，供你的服务端在收到我方通知时使用。
//
// 对原始 Body 按 SignNotice 口径验签（notice\n{appKey}\n{rawBody}），与请求 SignBody 一样
// 签原始字节；切勿先反序列化再 Marshal。
//
// 重要——验签 ≠ 防重放：
//
//   - 本方法只验证 HMAC：证明 body 未被篡改，且签名由持有 AppSecret 的一方生成。
//   - 不校验 data.timestamp 是否过期，也不记录/消费 nonce。
//   - NoticeRaw 中的 Nonce、Timestamp、AppKey 头字段不参与判断（验签以 Body + Signature 为准，
//     appKey 使用客户端配置的 AppKey）。
//   - 同一条合法回调被再次 POST 时，VerifyNotice 仍可能返回 true。
//
// 因此你必须在业务层做幂等，例如按 orderNo / refundNo / withdrawNo（或 data.nonce）
// 去重后再改订单状态、入账；平台未收到 {success:true} 时也会重投，幂等同样必要。
func (c *Client) VerifyNotice(raw NoticeRaw) bool {
	return verifyNoticeSignature(c.cfg.AppSecret, c.cfg.AppKey, raw.Body, raw.Signature)
}

func newNonce() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func truncateForError(b []byte) string {
	if len(b) <= maxErrorBodySnippet {
		return string(b)
	}
	return string(b[:maxErrorBodySnippet]) + "...(truncated)"
}
