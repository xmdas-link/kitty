package kitty

// Context 获取登录信息
type Context interface{
	CurrentUID() string
}