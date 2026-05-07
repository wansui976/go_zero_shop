package logic

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateProductStockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProductStockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProductStockLogic {
	return &UpdateProductStockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProductStockLogic) UpdateProductStock(in *product.UpdateProductStockRequest) (*product.UpdateProductStockResponse, error) {
	if in.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "无效的商品ID")
	}
	if in.Num <= 0 {
		return nil, status.Error(codes.InvalidArgument, "请求数量必须大于0")
	}

	db := l.svcCtx.DB
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		result, err := l.svcCtx.ProductModel.TxUpdateStock(tx, in.ProductId, -in.Num)
		if err != nil {
			l.Errorf("库存扣减SQL执行失败, productId: %d, err: %v", in.ProductId, err)
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("商品 %d 库存不足或已下架", in.ProductId))
		}
		l.Infof("库存扣减成功, productId: %d, 扣减数量: %d", in.ProductId, in.Num)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &product.UpdateProductStockResponse{}, nil
}
