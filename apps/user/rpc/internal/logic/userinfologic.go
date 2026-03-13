package logic

import (
	"context"

	"github.com/pkg/errors"
	//"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserInfoLogic {
	return &UserInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取用户信息
func (l *UserInfoLogic) UserInfo(in *user.UserInfoRequest) (*user.UserInfoResponse, error) {
	// 1. 入参校验（防御性编程，避免传入无效 ID）
	if in.Id <= 0 {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DataNoExistError), "用户 ID 无效：%d", in.Id)
	}

	// 2. 查询用户（注意：FindOne 的参数类型需与 UserModel 定义一致）
	userDB, err := l.svcCtx.UserModel.FindOne(l.ctx, uint64(in.Id))
	if err != nil {
		if err == model.ErrNotFound {
			// 明确提示用户不存在，而非通用 DB 错误
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DataNoExistError), "用户不存在（ID：%d）", in.Id)
		}
		// 数据库查询失败，包装错误信息
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "查询用户信息失败（ID：%d），err：%v", in.Id, err)
	}

	// 3. 初始化 resp.User（关键修复：避免空指针）
	resp := &user.UserInfoResponse{
		User: &user.UserInfo{ // 假设 User 字段类型是 *user.UserDetail，需提前初始化
			IntroduceSign: userDB.IntroduceSign,
			Phone:         userDB.Phone,
			Username:      userDB.Username,
		},
	}

	resp.User.Id = userDB.Id

	return resp, nil
}
