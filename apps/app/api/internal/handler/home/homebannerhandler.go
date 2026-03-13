package home

import (
	"net/http"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/logic/home"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 首页Banner列表
func HomeBannerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := home.NewHomeBannerLogic(r.Context(), svcCtx)
		resp, err := l.HomeBanner()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
