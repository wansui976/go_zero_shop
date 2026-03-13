package user

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"
	//"github.com/wansui976/go_zero_shop/pkg/result"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 用户登录
func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.CommonResponse, err error) {
	// 1. 入参校验（防御性编程，提前拦截无效请求）
	if req.Username == "" || req.Password == "" {
		return &types.CommonResponse{
			ResultCode: 400,
			Msg:        "用户名或密码不能为空",
			Data:       nil,
		}, nil
	}

	// 2. 调用 User RPC 服务进行登录验证
	loginResp, err := l.svcCtx.UserRPC.Login(l.ctx, &userclient.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		l.Errorf("用户登录失败（RPC调用异常）: %v", err)
		// 返回统一错误格式，前端可识别
		return &types.CommonResponse{
			ResultCode: 401,
			Msg:        "用户名或密码错误", // 对外隐藏具体错误原因，更安全
			Data:       nil,
		}, nil
	}

	// 3. 生成 JWT Token（优化：使用 go-zero 配置，避免硬编码）
	now := time.Now().Unix()
	cfg := l.svcCtx.Config.JwtAuth
	accessExpire := cfg.AccessExpire
	accessSecret := []byte(cfg.AccessSecret)

	// 组装 Token 载荷（仅保留必要字段，减少 Token 体积）
	claims := jwt.MapClaims{
		"exp":      now + accessExpire, // 过期时间（秒级）
		"iat":      now,                // 签发时间（秒级）
		"uid":      loginResp.Id,       // 用户 ID（与 RPC 响应字段对齐）
		"username": loginResp.Username, // 用户名
	}

	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	accessToken, err := token.SignedString(accessSecret)
	if err != nil {
		l.Errorf("生成 JWT Token 失败: %v", err)
		return &types.CommonResponse{
			ResultCode: 500,
			Msg:        "登录失败，请稍后重试",
			Data:       nil,
		}, nil
	}

	// 4. 组装响应（核心优化：字段名对齐+时间戳转毫秒）
	response := &types.LoginResp{
		AccessToken:  accessToken,                 // 与前端 localStorage 字段名一致
		AccessExpire: (now + accessExpire) * 1000, // 转换为毫秒级（前端 Date 需此格式）
		//Username:     loginResp.Username,              // 可选：返回用户名，前端直接渲染
		//UserId:       loginResp.Id,                    // 可选：返回用户 ID，减少前端二次查询
	}

	// 5. 包裹全局统一响应结构体（适配前端拦截器）
	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success", // 与前端拦截器期望的 "success" 一致
		Data:       response,
	}

	return resp, nil
}
