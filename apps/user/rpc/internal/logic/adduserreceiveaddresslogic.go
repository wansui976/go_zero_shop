package logic

import (
	"context"
	"fmt"
	"regexp"

	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type AddUserReceiveAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddUserReceiveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddUserReceiveAddressLogic {
	return &AddUserReceiveAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 添加收货地址
// 手机号正则（简化版，适配国内11位手机号）
var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

func (l *AddUserReceiveAddressLogic) AddUserReceiveAddress(in *user.UserReceiveAddressAddRequest) (*user.UserReceiveAddressAddResponse, error) {

	addrId, err := snowflake.GenIDInt()
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.SnowflakeError), "生成地址ID失败: %v", err)
	}

	dbAddr := &model.UserReceiveAddress{
		Id:            addrId,
		Uid:           in.Uid,
		Name:          in.Name,
		Phone:         in.Phone,
		Province:      in.Province,
		City:          in.City,
		Region:        in.Region,
		DetailAddress: in.DetailAddress,
		IsDefault:     uint64(in.IsDefault),
		IsDelete:      0,
	}

	// 类型断言：必须是 CustomUserReceiveAddressModel
	customModel, ok := l.svcCtx.UserReceiveAddressModel.(*model.CustomUserReceiveAddressModel)
	if !ok {
		return nil, errors.Wrap(xerr.NewErrCode(xerr.ReuqestParamError), "模型类型转换失败")
	}

	if in.IsDefault == 1 {

		// ------- go-zero 官方事务 -------
		err := customModel.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {

			// 取消旧默认地址
			if err := customModel.CancelOldDefaultAddress(ctx, session, in.Uid); err != nil {
				return err
			}

			// 插入新默认地址
			_, err := customModel.InsertWithTx(ctx, session, dbAddr)
			return err
		})

		if err != nil {
			return nil, errors.Wrap(xerr.NewErrCode(xerr.DbError), "事务失败: "+err.Error())
		}

	} else {
		// 非默认地址直接插入
		if _, err := customModel.Insert(l.ctx, dbAddr); err != nil {
			return nil, errors.Wrap(xerr.NewErrCode(xerr.DbError), "插入地址失败: "+err.Error())
		}
	}
	ver, _ := customModel.GetVersion(l.ctx, dbAddr.Uid)
	// 清主键缓存（
	if err := customModel.DelCacheCtx(l.ctx, fmt.Sprintf("user:address:list:%d:%d", dbAddr.Uid, ver)); err != nil {
		logx.Errorf("删除地址主键缓存失败: id=%d, err=%v", dbAddr.Id, err)
		return nil, err
	}
	// 缓存版本号递增
	if _, err := customModel.IncVersion(l.ctx, in.Uid); err != nil {
		logx.Errorf("递增版本号失败 uid=%d err=%v", in.Uid, err)
		return nil, err
	}

	return &user.UserReceiveAddressAddResponse{}, nil
}
