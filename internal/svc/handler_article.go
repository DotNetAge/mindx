package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 用户文章 RPC（社区文章发布）：文章发布到用户在社区的个人节点，组织节点
// 仅组织所有者可发（非所有者由服务端拒绝，错误消息原样透传）。服务端路由
// 与鉴权方式见 bundle.MarketClient 文章方法注释。

// handleArticlePublish 发布文章（个人节点；org_id>0 = 组织节点）。
func (d *Daemon) handleArticlePublish(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ArticlePublishParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(p.Slug)
	title := strings.TrimSpace(p.Title)
	if slug == "" {
		return nil, fmt.Errorf("slug 为必填")
	}
	if title == "" {
		return nil, fmt.Errorf("title 为必填")
	}
	if strings.TrimSpace(p.Markdown) == "" {
		return nil, fmt.Errorf("文章正文不能为空")
	}
	var res *bundle.ArticlePublishResult
	var err error
	if p.OrgID > 0 {
		res, err = d.app.Market().PublishArticleToOrg(p.OrgID, slug, title, p.Summary, p.Markdown, p.Tags)
	} else {
		res, err = d.app.Market().PublishArticle(slug, title, p.Summary, p.Markdown, p.Tags)
	}
	if err != nil {
		return nil, err
	}
	d.logger.Info("文章发布完成", "slug", res.Slug, "org_id", p.OrgID)
	return map[string]any{
		"slug":   res.Slug,
		"title":  res.Title,
		"date":   res.Date,
		"org_id": p.OrgID,
	}, nil
}

// handleArticleRead 文章详情（统一详情入口）：slug 通道读官网文章（匿名可读）/
// 个人文章（仅作者）/组织文章（仅成员），无权/不存在由服务端 404，错误消息原样
// 透传；id 通道读经验文章（org_slug 非空走组织回路），会员登录态。
func (d *Daemon) handleArticleRead(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ArticleReadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if slug := strings.TrimSpace(p.Slug); slug != "" {
		return d.app.Market().ReadArticle(slug)
	}
	if p.ID > 0 {
		return d.app.Market().ReadExperience(p.ID, strings.TrimSpace(p.OrgSlug))
	}
	return nil, fmt.Errorf("缺少文章标识（slug 或 id）")
}

// handleArticleList 文章清单/搜索（大统一读回路，scope 四选一）：
// site 官网公开（匿名可搜）| mine 我的个人与组织节点文章（缺省）|
// org 组织节点（org_id 文章通道 / org_slug 经验回路，二选一）| community 公共经验（会员）。
func (d *Daemon) handleArticleList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ArticleListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	scope := strings.TrimSpace(p.Scope)
	if scope == "" {
		scope = "mine"
	}
	var articles []bundle.MarketArticle
	var err error
	switch scope {
	case "site":
		articles, err = d.app.Market().ListSiteArticles(p.Query)
	case "mine":
		articles, err = d.app.Market().ListMyArticles(p.Query)
	case "community":
		articles, err = d.app.Market().ListCommunityExperiences(p.Query)
	case "org":
		if slug := strings.TrimSpace(p.OrgSlug); slug != "" {
			articles, err = d.app.Market().ListOrgExperiences(slug, p.Query)
		} else if p.OrgID > 0 {
			articles, err = d.app.Market().ListOrgArticles(p.OrgID, p.Query)
		} else {
			return nil, fmt.Errorf("查看组织资料需要提供组织（org_id 或 org_slug）")
		}
	default:
		return nil, fmt.Errorf("资料范围仅支持 site（官网）、mine（我的）、community（社区经验）或 org（组织）")
	}
	if err != nil {
		return nil, err
	}
	if articles == nil {
		articles = []bundle.MarketArticle{}
	}
	return map[string]any{"articles": articles}, nil
}

// handleArticleDelete 删除文章（仅作者；org_id>0 = 组织节点文章）。
func (d *Daemon) handleArticleDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ArticleDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return nil, fmt.Errorf("slug 为必填")
	}
	if p.OrgID > 0 {
		if err := d.app.Market().DeleteArticleInOrg(p.OrgID, slug); err != nil {
			return nil, err
		}
	} else {
		if err := d.app.Market().DeleteArticle(slug); err != nil {
			return nil, err
		}
	}
	d.logger.Info("文章删除完成", "slug", slug, "org_id", p.OrgID)
	return map[string]any{"deleted": true, "slug": slug, "org_id": p.OrgID}, nil
}
