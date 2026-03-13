package user

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type DetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取用户信息
func NewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DetailLogic {
	return &DetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DetailLogic) Detail(req *types.UserInfoReq) (resp *types.CommonResponse, err error) {
	var request userclient.UserInfoRequest
	request.Id, err = middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.UserRPC.UserInfo(l.ctx, &request)
	if err != nil {
		return nil, err
	}
	var userInfo types.UserInfo
	userInfo.Username = rpcResp.User.Username
	userInfo.Phone = rpcResp.User.Phone
	userInfo.IntroduceSign = rpcResp.User.IntroduceSign
	userInfoResp := &types.UserInfoResp{
		UserInfo: userInfo,
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success", 
		Data:       userInfoResp,
	}

	return resp, nil
}
