package home

import (
	"net/http"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/logic/home"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取首页信息
func HomeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := home.NewHomeLogic(r.Context(), svcCtx)
		resp, err := l.Home()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
