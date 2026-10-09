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
	"sort"
	"time"
)

// Config 是接入参数。
type Config struct {
	BaseURL   string        // 我方服务地址，如 https://api.qqlink.buzz
	AppKey    string        // 应用公钥（AK）
	AppSecret string        // 应用私钥（SK），仅服务端持有
	Timeout   time.Duration // 请求超时，默认 10 秒
}

// Client 封装 /open 接口的签名与调用。
type Client struct {
	cfg  Config
	http *http.Client
}

// New 创建客户端。
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.AppKey == "" || cfg.AppSecret == "" {
		return nil, errors.New("opensdk: BaseURL/AppKey/AppSecret 均为必填")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}, nil
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

// FriendList 查询用户好友中同样授权了本应用的部分。
func (c *Client) FriendList(ctx context.Context, in FriendListRequest) (*FriendListResult, error) {
	out := &FriendListResult{}
	return out, c.call(ctx, "/open/user/friends", in, out)
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
	req.Header.Set(HeaderSignature, c.SignParams(in, nonce))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 签名失败等鉴权错误由中间件直接返回，HTTP 状态码可能非 200，先看状态码。
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opensdk: http status %d body=%s", resp.StatusCode, string(respBody))
	}

	r := &response{}
	if err = json.Unmarshal(respBody, r); err != nil {
		return fmt.Errorf("opensdk: 解析响应失败: %w body=%s", err, string(respBody))
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

// SignParams 按约定的签名串计算签名。
//
// 与我方服务端口径一致：把参数按 key 排序后取 value，逐个以 & 连接，
// 再追加 timestamp、nonce 与 appKey，用 appSecret 做 HMAC-SHA256，输出 hex。
//
// 注意：结构体的 nil 指针字段会被跳过（不出现在签名串里），
// 因此请求体的字段请直接赋值，不要传 nil 指针。
func (c *Client) SignParams(in any, nonce string) string {
	params := structToParams(in)
	return Sign(c.cfg.AppSecret, params, c.cfg.AppKey, nonce)
}

// Sign 是签名算法的独立实现，回调验签也用它。
//
// timestamp 固定按 "0" 参与拼接：我方服务端不校验时间窗（跨国接入方时钟会漂移），
// 防重放由 nonce 去重与业务幂等兜底。
func Sign(appSecret string, params map[string]string, appKey, nonce string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramStr string
	for _, k := range keys {
		paramStr += params[k] + "&"
	}
	paramStr += fmt.Sprintf("%v&%v&%v", "0", nonce, appKey)

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(paramStr))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyNotice 校验回调签名，供你的服务端在收到我方通知时使用。
//
// 传原始 body 字节即可，SDK 会自行解析字段后按回调口径拼接。
// 回调签名串与请求签名口径不同，见 verifyNoticeSignature。
func (c *Client) VerifyNotice(raw NoticeRaw) bool {
	return verifyNoticeSignature(c.cfg.AppSecret, c.cfg.AppKey, raw.Nonce, raw.Body, raw.Signature)
}

func newNonce() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
