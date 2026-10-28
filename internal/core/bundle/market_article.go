package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// 用户文章与统一文章回路（社区文章发布与阅读）：文章发布到用户在社区的个人节点，
// 组织节点仅组织所有者可发。服务端路由（与账号/经验 API 同一 attachIdentity 通道；
// 官网文章读侧公开，匿名可访问）：
//
//	POST   /api/v1/articles               → 201 {slug,title,date}
//	DELETE /api/v1/articles/{slug}        → 204（仅作者）
//	GET    /api/v1/articles/{slug}        → {slug,title,summary,markdown,tags,date,org_id,scope}
//	                                        统一详情入口：官网文章匿名可读，个人文章仅作者、
//	                                        组织文章仅成员，无权/不存在 404（消息透传）
//	GET    /api/v1/articles?q=<kw>        → {"articles":[...]} 官网文章搜索（公开）
//	GET    /api/v1/articles?scope=mine&q=<kw>   → 我的文章搜索（含 org_id）
//	GET    /api/v1/orgs/{orgID}/articles?q=<kw> → 组织文章搜索（成员）
//	POST   /api/v1/orgs/{orgID}/articles  → 201（组织所有者）
//	DELETE /api/v1/orgs/{orgID}/articles/{slug} → 204（组织所有者）
//	GET    /api/v1/experiences            → {"experiences":[...]} 公共经验清单（会员）
//	GET    /api/v1/orgs/{slug}/experiences[/id] → 组织经验清单/全文（成员）
//
// 大统一（2026-10-27）：官网文章 / 经验文章 / 个人与组织节点文章是同一份文章库，
// 只是所有者与可读范围不同。读侧清单与全文以 Source 标来源，客户端按来源分组渲染。

// MarketArticle 统一文章清单条目（四来源归一形状；官网文章 slug 定位、经验文章
// id 定位、我的文章 slug+org_id 定位）。
type MarketArticle struct {
	// Source 来源回路：official 官网文章库 / experience 经验文章 / user 个人与组织节点文章。
	Source string `json:"source,omitempty"`
	// ID 经验文章主键（Source=experience）；其余来源为 0。
	ID int64 `json:"id,omitempty"`
	// Slug 定位标识（Source=official/user）。
	Slug    string `json:"slug,omitempty"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	// Date 展示日期 YYYY-MM-DD（官网 date / 经验与用户文章取发布日期）。
	Date string `json:"date,omitempty"`
	// Tags 逗号分隔标签（服务端形状 string）。
	Tags string `json:"tags,omitempty"`
	// OrgID 归属组织节点 ID（0 = 个人节点/官网文章）。
	OrgID int64 `json:"org_id,omitempty"`
	// SkillName 挂靠技能（Source=experience）。
	SkillName string `json:"skill_name,omitempty"`
	// AuthorName/AuthorAvatar 作者署名与头像（经验清单透出）。
	AuthorName   string `json:"author_name,omitempty"`
	AuthorAvatar string `json:"author_avatar,omitempty"`
	// ParentID/Position 树状挂载（Source=experience；ParentID 空 = 空间树根层级，
	// Position 为兄弟排序）。仅经验清单透出，前端按 parent_id 组树。
	ParentID *int64 `json:"parent_id,omitempty"`
	Position int64  `json:"position,omitempty"`
}

// ArticleDetail 文章详情（统一详情入口）。
type ArticleDetail struct {
	ID       int64  `json:"id,omitempty"` // 经验文章主键（经验详情）
	Slug     string `json:"slug,omitempty"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Markdown string `json:"markdown"`
	Tags     string `json:"tags,omitempty"`
	Date     string `json:"date,omitempty"`
	// OrgID 归属组织节点 ID（0 = 非组织文章）。
	OrgID int64 `json:"org_id,omitempty"`
	// Scope 归属范围：site（官网公开）/ personal（个人节点）/ org（组织节点）/
	// experience（经验文章）。
	Scope string `json:"scope,omitempty"`
	// SkillName/AuthorName/AuthorAvatar 经验详情透出（官网/用户文章为空）。
	SkillName    string `json:"skill_name,omitempty"`
	AuthorName   string `json:"author_name,omitempty"`
	AuthorAvatar string `json:"author_avatar,omitempty"`
	// CreatedAt 经验详情的创建时刻 RFC3339（官网/用户文章为空）。
	CreatedAt string `json:"created_at,omitempty"`
}

// ArticlePublishResult 文章发布响应（服务端 POST .../articles）。
type ArticlePublishResult struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// articleJSON 文章 API 专用 JSON 请求（POST/PUT/DELETE/GET 通用）：登录令牌鉴权
// （未登录直接报错，不发请求）；非 2xx 优先透传服务端的人话错误消息，404 按场景
// 归一（删除 = 文章不存在，其余 = 当前社区版本尚未提供文章功能）。
func (m *MarketClient) articleJSON(method, path string, body any, out any) error {
	if m.authToken == "" {
		return fmt.Errorf("尚未登录社区，请先绑定 MindX 账号")
	}
	return m.articleJSONOpen(method, path, body, out)
}

// articleJSONOpen 文章 API JSON 请求的匿名放行变体：官网文章读侧公开，未登录
// 也可访问（个人/组织文章的读写权限由服务端按令牌裁决，无权时原样返回错误）。
func (m *MarketClient) articleJSONOpen(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, m.apiBase+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusNotFound {
			if method == http.MethodDelete {
				return fmt.Errorf("这篇文章不存在，或它不属于你可管理的节点")
			}
			// 详情读取路径（GET /articles/{slug}）404 时透传服务端消息
			//（文章不存在或无权查看）；清单等其余接口 404 归一为社区版本未开放。
			if method == http.MethodGet && strings.HasPrefix(path, "/articles/") {
				var e struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
					return fmt.Errorf("%s", e.Error.Message)
				}
			}
			return fmt.Errorf("当前社区版本尚未开放文章功能，请稍后再试")
		}
		// 非 404 错误：透出服务端人话 message（回落见 apiErrorMessage）。
		return apiErrorMessage(data, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("解析响应失败：%w", err)
		}
	}
	return nil
}

// articleBody 组装发布请求体（个人节点与组织节点共用；空可选字段不下发）。
// tags 服务端形状为逗号分隔 string（数组入参在客户端 join）。
func articleBody(slug, title, summary, markdown string, tags []string) map[string]any {
	body := map[string]any{"slug": slug, "title": title, "markdown": markdown}
	if summary = strings.TrimSpace(summary); summary != "" {
		body["summary"] = summary
	}
	if len(tags) > 0 {
		body["tags"] = strings.Join(tags, ",")
	}
	return body
}

// PublishArticle 发布文章到个人节点（POST /api/v1/articles；slug 已存在时由
// 服务端报错，调用方原样转达）。
func (m *MarketClient) PublishArticle(slug, title, summary, markdown string, tags []string) (*ArticlePublishResult, error) {
	var out ArticlePublishResult
	if err := m.articleJSON(http.MethodPost, "/articles", articleBody(slug, title, summary, markdown, tags), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PublishArticleToOrg 发布文章到组织节点（POST /api/v1/orgs/{orgID}/articles；
// 仅组织所有者，非所有者由服务端拒绝）。
func (m *MarketClient) PublishArticleToOrg(orgID int64, slug, title, summary, markdown string, tags []string) (*ArticlePublishResult, error) {
	var out ArticlePublishResult
	path := "/orgs/" + strconv.FormatInt(orgID, 10) + "/articles"
	if err := m.articleJSON(http.MethodPost, path, articleBody(slug, title, summary, markdown, tags), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// articleListQuery 组装文章清单查询串：scope 段在前，q 关键词可选（空串省略）。
func articleListQuery(scope, q string) string {
	vals := url.Values{}
	if scope != "" {
		vals.Set("scope", scope)
	}
	if q = strings.TrimSpace(q); q != "" {
		vals.Set("q", q)
	}
	if enc := vals.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

// ReadArticle 读取文章详情（GET /api/v1/articles/{slug} 统一详情入口）：
// 官网文章匿名可读；个人文章仅作者、组织文章仅成员，无权或不存在时 404，
// 服务端的人话错误消息原样透传。
func (m *MarketClient) ReadArticle(slug string) (*ArticleDetail, error) {
	var out struct {
		Article *struct {
			Slug      string `json:"slug"`
			Title     string `json:"title"`
			Summary   string `json:"summary"`
			Markdown  string `json:"markdown"`
			Tags      string `json:"tags"`
			Date      string `json:"date"`
			OrgID     int64  `json:"org_id"`
			ScopeName string `json:"scope"`
		} `json:"article"`
	}
	path := "/articles/" + url.PathEscape(slug)
	if err := m.articleJSONOpen(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	if out.Article == nil {
		return nil, fmt.Errorf("这篇文章不存在或没有权限查看")
	}
	a := out.Article
	return &ArticleDetail{
		Slug: a.Slug, Title: a.Title, Summary: a.Summary, Markdown: a.Markdown,
		Tags: a.Tags, Date: a.Date, OrgID: a.OrgID, Scope: a.ScopeName,
	}, nil
}

// ReadExperience 读取经验文章全文（GET /experiences/{id} 或组织回路；会员登录态）。
// Scope 以 "experience" 标来源（与 slug 通道的 site/personal/org 并列）。
func (m *MarketClient) ReadExperience(id int64, orgSlug string) (*ArticleDetail, error) {
	var out struct {
		Experience *struct {
			ID           int64  `json:"id"`
			Title        string `json:"title"`
			Summary      string `json:"summary"`
			Markdown     string `json:"markdown"`
			SkillName    string `json:"skill_name"`
			AuthorName   string `json:"author_name"`
			AuthorAvatar string `json:"author_avatar"`
			CreatedAt    string `json:"created_at"`
		} `json:"experience"`
	}
	path := "/experiences/" + strconv.FormatInt(id, 10)
	if orgSlug = strings.TrimSpace(orgSlug); orgSlug != "" {
		path = "/orgs/" + url.PathEscape(orgSlug) + path
	}
	if err := m.articleJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	if out.Experience == nil {
		return nil, fmt.Errorf("这篇文章不存在或没有权限查看")
	}
	e := out.Experience
	return &ArticleDetail{
		ID: e.ID, Title: e.Title, Summary: e.Summary, Markdown: e.Markdown,
		Scope: "experience", SkillName: e.SkillName,
		AuthorName: e.AuthorName, AuthorAvatar: e.AuthorAvatar,
		Date: datePartOf(e.CreatedAt), CreatedAt: e.CreatedAt,
	}, nil
}

// ListSiteArticles 搜索官网公开文章（GET /api/v1/articles?q=<kw>；公开，匿名可访问）。
func (m *MarketClient) ListSiteArticles(q string) ([]MarketArticle, error) {
	var out struct {
		Articles []struct {
			Slug    string `json:"slug"`
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Date    string `json:"date"`
			Tags    string `json:"tags"`
		} `json:"articles"`
	}
	if err := m.articleJSONOpen(http.MethodGet, "/articles"+articleListQuery("", q), nil, &out); err != nil {
		return nil, err
	}
	items := make([]MarketArticle, 0, len(out.Articles))
	for _, a := range out.Articles {
		items = append(items, MarketArticle{
			Source: "official", Slug: a.Slug, Title: a.Title,
			Summary: a.Summary, Date: a.Date, Tags: a.Tags,
		})
	}
	return items, nil
}

// ListMyArticles 搜索我的文章清单（GET /api/v1/articles?scope=mine&q=<kw>；
// 个人节点 + 组织节点合并，作者视角全量）。
func (m *MarketClient) ListMyArticles(q string) ([]MarketArticle, error) {
	var out struct {
		Articles []struct {
			Slug    string `json:"slug"`
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Date    string `json:"date"`
			Tags    string `json:"tags"`
			OrgID   int64  `json:"org_id"`
		} `json:"articles"`
	}
	if err := m.articleJSON(http.MethodGet, "/articles"+articleListQuery("mine", q), nil, &out); err != nil {
		return nil, err
	}
	items := make([]MarketArticle, 0, len(out.Articles))
	for _, a := range out.Articles {
		items = append(items, MarketArticle{
			Source: "user", Slug: a.Slug, Title: a.Title, Summary: a.Summary,
			Date: a.Date, Tags: a.Tags, OrgID: a.OrgID,
		})
	}
	return items, nil
}

// ListOrgArticles 搜索组织节点文章（GET /api/v1/orgs/{orgID}/articles?q=<kw>；
// 组织成员可读，非成员由服务端拒绝）。
func (m *MarketClient) ListOrgArticles(orgID int64, q string) ([]MarketArticle, error) {
	var out struct {
		Articles []struct {
			Slug    string `json:"slug"`
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Date    string `json:"date"`
			Tags    string `json:"tags"`
			OrgID   int64  `json:"org_id"`
		} `json:"articles"`
	}
	path := "/orgs/" + strconv.FormatInt(orgID, 10) + "/articles" + articleListQuery("", q)
	if err := m.articleJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	items := make([]MarketArticle, 0, len(out.Articles))
	for _, a := range out.Articles {
		items = append(items, MarketArticle{
			Source: "user", Slug: a.Slug, Title: a.Title, Summary: a.Summary,
			Date: a.Date, Tags: a.Tags, OrgID: a.OrgID,
		})
	}
	return items, nil
}

// ListCommunityExperiences 公共经验清单（GET /api/v1/experiences；会员登录态，
// daemon 携带内置登录态代理）。
func (m *MarketClient) ListCommunityExperiences(q string) ([]MarketArticle, error) {
	return m.listExperienceEntries("/experiences", q)
}

// ListOrgExperiences 组织经验清单（GET /api/v1/orgs/{slug}/experiences；本人在册
// 组织，slug 寻址与成员资格由服务端裁决）。
func (m *MarketClient) ListOrgExperiences(orgSlug, q string) ([]MarketArticle, error) {
	return m.listExperienceEntries("/orgs/"+url.PathEscape(orgSlug)+"/experiences", q)
}

// listExperienceEntries 经验清单共用取数（prefix 区分公共/组织回路）→ 统一条目。
func (m *MarketClient) listExperienceEntries(prefix, q string) ([]MarketArticle, error) {
	var out struct {
		Experiences []struct {
			ID           int64  `json:"id"`
			Title        string `json:"title"`
			Summary      string `json:"summary"`
			SkillName    string `json:"skill_name"`
			AuthorName   string `json:"author_name"`
			AuthorAvatar string `json:"author_avatar"`
			CreatedAt    string `json:"created_at"`
			ParentID     *int64 `json:"parent_id"`
			Position     int64  `json:"position"`
		} `json:"experiences"`
	}
	if err := m.articleJSON(http.MethodGet, prefix+articleListQuery("", q), nil, &out); err != nil {
		return nil, err
	}
	items := make([]MarketArticle, 0, len(out.Experiences))
	for _, e := range out.Experiences {
		items = append(items, MarketArticle{
			Source: "experience", ID: e.ID, Title: e.Title, Summary: e.Summary,
			SkillName: e.SkillName, AuthorName: e.AuthorName, AuthorAvatar: e.AuthorAvatar,
			Date: datePartOf(e.CreatedAt), ParentID: e.ParentID, Position: e.Position,
		})
	}
	return items, nil
}

// datePartOf RFC3339 时间取日期部分（YYYY-MM-DD；不足 10 位原样返回）。
func datePartOf(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// DeleteArticle 删除个人节点文章（DELETE /api/v1/articles/{slug}；仅作者，204）。
func (m *MarketClient) DeleteArticle(slug string) error {
	return m.articleJSON(http.MethodDelete, "/articles/"+url.PathEscape(slug), nil, nil)
}

// DeleteArticleInOrg 删除组织节点文章（DELETE /api/v1/orgs/{orgID}/articles/{slug}；
// 仅组织所有者，204）。
func (m *MarketClient) DeleteArticleInOrg(orgID int64, slug string) error {
	path := "/orgs/" + strconv.FormatInt(orgID, 10) + "/articles/" + url.PathEscape(slug)
	return m.articleJSON(http.MethodDelete, path, nil, nil)
}
