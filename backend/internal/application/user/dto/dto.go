// Package dto 定义当前用户资料用例的输入，不承载 HTTP 协议细节。
package dto

// GetMeInput 的用户标识来自已认证身份，不能采用客户端自报的用户标识。
type GetMeInput struct {
	UserID int64
}
