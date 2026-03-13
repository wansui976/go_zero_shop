package logic

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type DecrStockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDecrStockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DecrStockLogic {
	return &DecrStockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DecrStockLogic) DecrStock(in *product.DecrStockRequest) (*product.DecrStockResponse, error) {

	for _, it := range in.Items {
		if it.Num == 0 {
			return nil, status.Error(codes.InvalidArgument, "无效的购买数量")
		}
		if it.Id <= 0 {
			l.Errorf("无效的商品ID: %d", it.Id)
			return nil, status.Error(codes.InvalidArgument, "无效的商品ID")
		}
	}

	db := l.svcCtx.DB
	//获取子事务屏障
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	//开启子事务屏障
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {

		for _, it := range in.Items {

			result, err := l.svcCtx.ProductModel.TxUpdateStock(tx, it.Id, -it.Num)
			if err != nil {
				fmt.Errorf("库存扣减SQL执行失败, productId: %d, err: %v", it.Id, err)
				return err
			}
			//检查受影响的行数，判断库存是否充足
			affected, _ := result.RowsAffected()
			if affected == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("商品 %d 库存不足或已下架", it.Id))
			}
			l.Infof("库存扣减成功, productId: %d, 扣减数量: %d, 受影响行数: %d", it.Id, it.Num, affected)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &product.DecrStockResponse{}, nil
}

// if in.Id <= 0 {
// 	l.Errorf("无效的商品ID: %d", in.Id)
// 	return nil, status.Error(codes.InvalidArgument, "无效的商品ID")
// }

// decrNum := in.Num
// if decrNum <= 0 {
// 	decrNum = 1
// }

// l.Infof("开始扣减商品库存, productId: %d, 扣减数量: %d", in.Id, decrNum)

// db := l.svcCtx.DB
// //获取子事务屏障
// barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
// if err != nil {
// 	l.Errorf("创建子事务屏障失败:%v", err)
// }
// //开启子事务屏障
// err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
// 	result, err := l.svcCtx.ProductModel.TxUpdateStock(tx, in.Id, -decrNum)
// 	if err != nil {
// 		l.Errorf("库存扣减SQL执行失败, productId: %d, err: %v", in.Id, err)
// 		return err
// 	}

// 	//检查受影响的行数，判断库存是否充足
// 	affected, err := result.RowsAffected()
// 	if err != nil {
// 		l.Errorf("获取收影响行数失败,productId:%d,err:%v", in.Id, err)
// 		return err
// 	}

// 	if affected == 0 {
// 		l.Errorf("商品库存不足或不存在, productId: %d, 尝试扣减: %d", in.Id, decrNum)
// 		return dtmcli.ErrFailure // 向DTM报告失败
// 	}
// 	l.Infof("库存扣减成功, productId: %d, 扣减数量: %d, 受影响行数: %d", in.Id, decrNum, affected)
// 	return nil
// })

// // 处理子事务屏障返回的错误
// if err != nil {
// 	// 明确的库存不足失败，返回DTM失败标识
// 	if err == dtmcli.ErrFailure {
// 		l.Errorf("库存扣减失败, productId: %d, 原因: 库存不足", in.Id)
// 		return nil, status.Error(codes.Aborted, dtmcli.ResultFailure)
// 	}

// 	// 其他错误（如数据库异常）
// 	l.Errorf("库存扣减过程异常, productId: %d, err: %v", in.Id, err)
// 	return nil, status.Error(codes.Internal, "库存扣减失败")
// }

// l.Infof("商品库存扣减完成, productId: %d", in.Id)
