package productStock

import (
	"net/http"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/logic/productStock"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 库存检查与扣减（下单专用）
func CheckAndUpdateStockHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CheckAndUpdateStockRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := productStock.NewCheckAndUpdateStockLogic(r.Context(), svcCtx)
		resp, err := l.CheckAndUpdateStock(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
