package cart

import (
	"net/http"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/logic/cart"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 添加购物车
func CartAddHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CartAddRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := cart.NewCartAddLogic(r.Context(), svcCtx)
		resp, err := l.CartAdd(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
