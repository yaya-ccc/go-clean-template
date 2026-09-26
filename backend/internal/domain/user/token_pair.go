package user

// TokenPair 是一次签发同时产出的 access token 与 refresh token。
// refresh 明文只在签发瞬间出现一次，服务端持久层只存其哈希。
type TokenPair struct {
	Access  string
	Refresh string
}
