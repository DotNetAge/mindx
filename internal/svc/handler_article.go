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

// handleArticleRead 文章详情（GET 统一详情入口：官网文章匿名可读；个人文章
// 仅作者、组织文章仅成员，无权/不存在由服务端 404，错误消息原样透传）。
func (d *Daemon) handleArticleRead(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ArticleReadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		return nil, fmt.Errorf("slug 为必填")
	}
	detail, err := d.app.Market().ReadArticle(slug)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// handleArticleList 文章清单/搜索（scope：site 官网公开 | mine 我的（缺省）|
// org 组织成员；q 为服务端关键词搜索）。
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
	case "org":
		if p.OrgID <= 0 {
			return nil, fmt.Errorf("查看组织文章需要提供组织节点 ID（org_id）")
		}
		articles, err = d.app.Market().ListOrgArticles(p.OrgID, p.Query)
	default:
		return nil, fmt.Errorf("文章清单范围仅支持 site（官网）、mine（我的）或 org（组织）")
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
