package result

// 统一响应结构体（适配前端：resultCode + message + data）
// 成功/失败统一用一个结构体，避免前端处理不同格式
type Response struct {
	ResultCode uint32      `json:"resultCode"` // 对应前端 resultCode
	Message    string      `json:"message"`    // 对应前端 message
	Data       interface{} `json:"data"`       // 业务数据（成功有值，失败为nil）
}

// 空JSON结构体（无业务数据时使用）
type NullJson struct{}

// Success：成功响应（返回 resultCode=200 + message=success + 业务数据）
func Success(data interface{}) *Response {
	return &Response{
		ResultCode: 200,
		Message:    "success",
		Data:       data,
	}
}

// Error：失败响应（返回自定义错误码 + 错误信息 + data=nil）
func Error(errCode uint32, errMsg string) *Response {
	return &Response{
		ResultCode: errCode,
		Message:    errMsg,
		Data:       nil, // 失败时 data 设为 nil，前端无需处理
	}
}

// 可选：常用错误码快捷函数（简化使用）
func ErrorParam() *Response { // 参数错误
	return Error(400, "参数格式错误")
}

func ErrorTokenExpire() *Response { // Token失效
	return Error(401, "Token已过期，请重新登录")
}

func ErrorNotLogin() *Response { // 未登录
	return Error(416, "请先登录")
}

func ErrorDataNotFound() *Response { // 数据不存在
	return Error(404, "请求数据不存在")
}

func ErrorServer() *Response { // 服务端异常
	return Error(500, "服务端异常，请稍后重试")
}
