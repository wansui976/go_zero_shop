package product

import (
	"context"
	"strings"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	replyclient "github.com/wansui976/go_zero_shop/apps/reply/rpc/replyclient"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"
	"github.com/zeromicro/go-zero/core/logx"
)

func buildProductComments(ctx context.Context, userRPC user.UserClient, items []*replyclient.CommentItem) []*types.Comment {
	userCache := make(map[int64]*types.UserMini)
	comments := make([]*types.Comment, 0, len(items))

	for _, item := range items {
		userInfo := getCommentUser(ctx, userRPC, userCache, item.ReplyUserId)
		comments = append(comments, &types.Comment{
			ID:         item.Id,
			ProductID:  item.TargetId,
			Content:    item.Content,
			Images:     buildCommentImages(item.Image),
			User:       userInfo,
			CreateTime: item.CreateTime,
			UpdateTime: item.UpdateTime,
		})
	}

	return comments
}

func getCommentUser(ctx context.Context, userRPC user.UserClient, cache map[int64]*types.UserMini, userID int64) *types.UserMini {
	if userID <= 0 {
		return &types.UserMini{}
	}
	if cached, ok := cache[userID]; ok {
		return &types.UserMini{ID: cached.ID, Name: cached.Name, Avatar: cached.Avatar}
	}

	mini := &types.UserMini{ID: userID, Avatar: ""}
	resp, err := userRPC.UserInfo(ctx, &userclient.UserInfoRequest{Id: userID})
	if err != nil {
		logx.WithContext(ctx).Errorf("query comment user failed: user_id=%d err=%v", userID, err)
		cache[userID] = mini
		return &types.UserMini{ID: mini.ID, Name: mini.Name, Avatar: mini.Avatar}
	}
	if resp != nil && resp.User != nil {
		mini.Name = resp.User.Username
	}

	cache[userID] = mini
	return &types.UserMini{ID: mini.ID, Name: mini.Name, Avatar: mini.Avatar}
}

func buildCommentImages(image string) []*types.Image {
	image = strings.TrimSpace(image)
	if image == "" {
		return []*types.Image{}
	}

	parts := strings.Split(image, ",")
	resp := make([]*types.Image, 0, len(parts))
	for idx, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		resp = append(resp, &types.Image{
			ID:  int64(idx + 1),
			URL: part,
		})
	}
	return resp
}
