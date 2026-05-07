package logic

import (
	"context"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/tool"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 登录
func (l *LoginLogic) Login(in *user.LoginRequest) (*user.LoginResponse, error) {
	userDB, err := l.svcCtx.UserModel.FindOneByUsername(l.ctx, in.Username)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "根据 username 查询用户信息失败")
		}
		return nil, err
	}
	if !tool.CheckPassword(in.Password, userDB.Password) {
		return nil, errors.Wrap(xerr.NewErrMsg("账号密码错误"), "密码错误")
	}
	var resp user.LoginResponse
	_ = copier.Copy(&resp, userDB)
	return &resp, nil
}
