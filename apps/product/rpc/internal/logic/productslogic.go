package logic

import (
	"context"
	"strconv"
	"strings"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/model"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
)

type ProductsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductsLogic {
	return &ProductsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProductsLogic) Products(in *product.ProductRequest) (*product.ProductResponse, error) {
	products := make(map[int64]*product.ProductItem)
	pdis := strings.Split(in.ProductIds, ",")

	// 修复1：显式指定泛型参数，确保“输入类型、Map输出类型、Reduce输出类型”一致
	// 泛型参数说明：
	// - 第一个 string：Map 数据源类型（商品ID字符串）
	// - 第二个 *model.Product：Map 输出类型（单个商品详情）
	// - 第三个 []*model.Product：Reduce 输出类型（商品列表）
	ps, err := mr.MapReduce(
		// 阶段1：Map 数据源（输出 string 类型的商品ID）
		func(source chan<- string) {
			for _, pid := range pdis {
				if pid != "" { // 额外优化：过滤空字符串（如客户端传入 ",1001"）
					source <- pid
				}
			}
		},

		// 阶段2：Map 任务处理（输入 string，输出 *model.Product）
		func(item string, writer mr.Writer[*model.Product], cancel func(error)) {
			// 无需类型断言：item 直接是 string 类型（泛型参数已指定）
			pid, err := strconv.ParseInt(item, 10, 64)
			if err != nil {
				l.Logger.Errorf("invalid product id: %s, err: %v", item, err)
				return
			}

			// 查询单个商品（返回 *model.Product，与 writer 类型匹配）
			p, err := l.svcCtx.ProductModel.FindOne(l.ctx, pid)
			if err != nil {
				l.Logger.Errorf("find product %d failed, err: %v", pid, err)
				return
			}

			writer.Write(p)
		},

		// 阶段3：Reduce 结果合并（输入 *model.Product，输出 []*model.Product）
		func(pipe <-chan *model.Product, writer mr.Writer[[]*model.Product], cancel func(error)) {
			var r []*model.Product

			for p := range pipe {
				r = append(r, p)
			}
			// 写入合并后的列表（类型匹配）
			writer.Write(r)
		},
	)

	if err != nil {
		l.Logger.Errorf("batch find products failed, err: %v", err)
		return nil, err
	}

	for _, p := range ps {
		products[p.Id] = &product.ProductItem{
			Id:   p.Id,
			Name: p.Name,
		}
	}

	l.Logger.Infof("batch find products success, request ids: %s, success count: %d", in.ProductIds, len(products))
	return &product.ProductResponse{Products: products}, nil
}
