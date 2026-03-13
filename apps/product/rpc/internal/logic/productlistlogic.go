package logic

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/model"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
)

const (
	defaultPageSize = 10
	defaultLimit    = 300
	expireTime      = 3600 * 24 * 3
)

type ProductListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProductListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductListLogic {
	return &ProductListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func categoryKey(cid int32) string {
	return fmt.Sprintf("category:%d", cid)
}

// cacheProductList：从 Redis ZSET 按 score 倒序读取 id 列表（返回 int64 id 列表）
// 参数 cursor：表示最大 score（inclusive）。如果 cursor == 0 表示从 +inf 开始（即最新）。
func (l *ProductListLogic) cacheProductList(ctx context.Context, cid int32, cursor int64, ps int64) ([]int64, error) {
	key := categoryKey(cid)

	var max int64
	if cursor <= 0 {
		max = int64(math.MaxInt64)
	} else {
		max = cursor
	}
	var min int64 = 0

	// BizRedis: 请确保你项目中 BizRedis 提供的方法签名如下：
	// ZrevrangebyscoreWithScoresAndLimitCtx(ctx, key string, max, min int64, offset, count int) ([]redis.Pair, error)
	// 若签名不同，请按你项目实际方法适配这里的调用与返回解析。
	pairs, err := l.svcCtx.BizRedis.ZrevrangebyscoreWithScoresAndLimitCtx(ctx, key, max, min, 0, int(ps))
	if err != nil {
		logx.Errorf("cacheProductList: zrevrangebyscore failed key=%s err=%v", key, err)
		return nil, err
	}

	ids := make([]int64, 0, len(pairs))
	for _, pair := range pairs {
		id, perr := strconv.ParseInt(pair.Key, 10, 64)
		if perr != nil {
			l.Errorf("cacheProductList: parse id failed key=%s err=%v", pair.Key, perr)
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// productsByIds：并发拉取 product 详情，保持输入 id 顺序（最终按输入顺序返回）
func (l *ProductListLogic) productsByIds(ctx context.Context, pids []int64) ([]*model.Product, error) {
	if len(pids) == 0 {
		return []*model.Product{}, nil
	}

	// 使用 go-zero 的 mr.MapReduce 并发获取
	// 注意：mr.MapReduce 的具体泛型签名依赖 go-zero 版本，这里使用非泛型风格
	products, err := mr.MapReduce(
		// ------- 1. Source -------
		func(source chan<- int64) {
			for _, id := range pids {
				source <- id
			}
		},

		// ------- 2. Mapper -------
		func(pid int64, writer mr.Writer[*model.Product], cancel func(error)) {
			p, err := l.svcCtx.ProductModel.FindOne(ctx, pid)
			if err != nil {
				cancel(err)
				return
			}
			writer.Write(p)
		},

		// ------- 3. Reducer -------
		func(pipe <-chan *model.Product, writer mr.Writer[[]*model.Product], cancel func(error)) {
			// 用 map 保证最终按 pids 顺序返回
			m := make(map[int64]*model.Product, len(pids))
			for p := range pipe {
				m[p.Id] = p
			}

			ordered := make([]*model.Product, 0, len(pids))
			for _, id := range pids {
				if v, ok := m[id]; ok {
					ordered = append(ordered, v)
				}
			}

			writer.Write(ordered)
		},
	)
	if err != nil {
		l.Errorf("productsByIds: MapReduce failed: %v", err)
		return nil, err
	}

	return products, nil
}

// addCacheProductList：异步把 DB 拉到的 products 写入 ZSET
func (l *ProductListLogic) addCacheProductList(ctx context.Context, categoryId int32, products []*model.Product, isEnd bool) error {
	if len(products) == 0 {
		_ = l.svcCtx.BizRedis.ExpireCtx(ctx, categoryKey(categoryId), expireTime)
		return nil
	}

	key := categoryKey(categoryId)
	for _, p := range products {
		score := p.CreateTime.Unix()
		if score < 0 {
			score = 0
		}
		// ZADD 是幂等的
		if _, err := l.svcCtx.BizRedis.ZaddCtx(ctx, key, score, strconv.FormatInt(p.Id, 10)); err != nil {
			l.Errorf("addCacheProductList: zadd failed category=%d id=%d err=%v", categoryId, p.Id, err)
		}
	}

	if isEnd {
		_, _ = l.svcCtx.BizRedis.ZaddCtx(ctx, key, 0, "-1")
	}

	if err := l.svcCtx.BizRedis.ExpireCtx(ctx, key, expireTime); err != nil {
		l.Errorf("addCacheProductList: expire failed key=%s err=%v", key, err)
	}
	return nil
}

// ProductList：入口逻辑（兼容缓存与 DB 回源）
// in.Cursor 表示上一页最后一条的 create_time（秒），若为 0 表示从最新开始
func (l *ProductListLogic) ProductList(in *product.ProductListRequest) (*product.ProductListResponse, error) {
	l.Infof("ProductList called: CategoryId=%d, Cursor=%d, Ps=%d, ProductId=%d",
		in.CategoryId, in.Cursor, in.Ps, in.ProductId)
	if in.Ps <= 0 {
		in.Ps = defaultPageSize
	}

	if in.Cursor == 0 {
		in.Cursor = time.Now().Unix()
	}
	cursor := in.Cursor

	cid := in.CategoryId
	if cid <= 0 {
		return nil, fmt.Errorf("invalid categoryId: %d", cid)
	}

	// 尝试缓存
	pids, err := l.cacheProductList(l.ctx, cid, cursor, int64(in.Ps))
	if err == nil && len(pids) > 0 {
		isEnd := false
		filtered := make([]int64, 0, len(pids))
		for _, id := range pids {
			if id == -1 {
				isEnd = true
				continue
			}
			filtered = append(filtered, id)
		}

		if len(filtered) >= int(in.Ps) {
			products, err := l.productsByIds(l.ctx, filtered)
			if err == nil {
				firstPage := make([]*product.ProductItem, 0, len(products))
				for _, p := range products {
					desc := ""
					if p.Brief.Valid {
						desc = p.Brief.String
					}
					img := ""
					if p.ImageUrl.Valid {
						img = p.ImageUrl.String
					}
					firstPage = append(firstPage, &product.ProductItem{
						Id:         p.Id,
						Name:       p.Name,
						Brief:      desc,
						ImageUrl:   img,
						Price:      p.Price,
						Stock:      p.Stock,
						CategoryId: p.CategoryId.Int64,
						Status:     product.ProductStatus(p.Status),
						CreateTime: p.CreateTime.Unix(),
					})
				}
				var lastID int64
				var lastTime int64
				if len(firstPage) > 0 {
					last := firstPage[len(firstPage)-1]
					lastID = last.Id
					lastTime = last.CreateTime
				}
				return &product.ProductListResponse{
					IsEnd:     isEnd,
					Timestamp: lastTime,
					ProductId: lastID,
					Products:  firstPage,
				}, nil
			}
			l.Errorf("ProductList: productsByIds failed (cached ids): %v", err)
			// 如果缓存解析/读取失败，放弃缓存并回源 DB
		}
	}

	// DB 回源：把 cursor 秒时间转成 MySQL datetime 字符串
	ctime := time.Unix(cursor, 0).Format("2006-01-02 15:04:05")
	limit := int(in.Ps) + 1
	productsFromDB, err := l.svcCtx.ProductModel.FindByCategory(l.ctx, ctime, int64(cid), int64(limit))
	if err != nil {
		l.Errorf("ProductList: FindByCategory failed: %v", err)
		return nil, err
	}

	isEnd := false
	var sliceForResp []*model.Product
	if len(productsFromDB) <= int(in.Ps) {
		isEnd = true
		sliceForResp = productsFromDB
	} else {
		sliceForResp = productsFromDB[:int(in.Ps)]
	}

	firstPage := make([]*product.ProductItem, 0, len(sliceForResp))
	for _, p := range sliceForResp {
		desc := ""
		if p.Brief.Valid {
			desc = p.Brief.String
		}
		img := ""
		if p.ImageUrl.Valid {
			img = p.ImageUrl.String
		}
		firstPage = append(firstPage, &product.ProductItem{
			Id:         p.Id,
			Name:       p.Name,
			Brief:      desc,
			ImageUrl:   img,
			Price:      p.Price,
			Stock:      p.Stock,
			CategoryId: p.CategoryId.Int64,
			Status:     product.ProductStatus(p.Status),
			CreateTime: p.CreateTime.Unix(),
		})
	}

	var lastID int64
	var lastTime int64
	if len(firstPage) > 0 {
		last := firstPage[len(firstPage)-1]
		lastID = last.Id
		lastTime = last.CreateTime
	}

	// 后台异步写缓存（写入整个 DB 拉到的列表）
	go func(products []*model.Product, cid int32, isEnd bool) {
		defer func() {
			if r := recover(); r != nil {
				l.Errorf("addCacheProductList panic: %v", r)
			}
		}()
		_ = l.addCacheProductList(context.Background(), cid, products, isEnd)
	}(productsFromDB, cid, isEnd)

	return &product.ProductListResponse{
		IsEnd:     isEnd,
		Timestamp: lastTime,
		ProductId: lastID,
		Products:  firstPage,
	}, nil
}
