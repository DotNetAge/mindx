package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// 组织运行面·架构与人事：组织架构树（XML 蓝图）与成员管理客户端方法。
// 架构树是大秘经「构建组织」技能发布的唯一事实源（老板只见图，不见 XML）；
// 邀请/移除沿 market REST 原语义，服务端 RequireOrgRole 实时鉴权。

// putRaw 原始字节 PUT（架构树发布用；非 2xx 时优先解析服务端人话错误信息）。
func (m *MarketClient) putRaw(u string, contentType string, data []byte, out any) error {
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respData, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiErrorMessage(respData, resp.StatusCode)
	}
	if out != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, out); err != nil {
			return fmt.Errorf("解析响应失败：%w", err)
		}
	}
	return nil
}

// apiErrorMessage 服务端错误体优先取人话 message（writeError 的 {"error":{...}} 形态），
// 解析不出再回退原始文本。
func apiErrorMessage(data []byte, status int) error {
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err == nil && body.Error.Message != "" {
		return fmt.Errorf("%s", body.Error.Message)
	}
	return fmt.Errorf("请求失败（HTTP %d）：%s", status, strings.TrimSpace(string(data)))
}

// OrgArchitectureGet 取组织架构树 XML 原文（GET /orgs/{slug}/architecture；
// 未发布时服务端 404，返回带「尚未发布」字样的错误）。
func (m *MarketClient) OrgArchitectureGet(orgSlug string) (string, error) {
	resp, err := m.authedGet(m.apiBase + "/orgs/" + url.PathEscape(orgSlug) + "/architecture")
	if err != nil {
		return "", fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", apiErrorMessage(data, resp.StatusCode)
	}
	return string(data), nil
}

// PublishOrgArchitecture 发布/覆盖组织架构树 XML（PUT /orgs/{slug}/architecture；
// 整树替换，服务端校验结构合法性；仅 owner/admin 可发布）。
func (m *MarketClient) PublishOrgArchitecture(orgSlug, architectureXML string) error {
	trimmed := strings.TrimSpace(architectureXML)
	if trimmed == "" {
		return fmt.Errorf("架构树内容为空")
	}
	if m.authToken == "" && m.pat == "" {
		return fmt.Errorf("发布前请先绑定 MindX 账号或配置商户授权")
	}
	return m.putRaw(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/architecture",
		"application/xml; charset=utf-8", []byte(trimmed), nil)
}

// InviteToOrg 按可寻址账号发定向邀请（POST /orgs/{slug}/invites；仅 owner/admin）。
// 同组织同账号 pending 唯一；员工凭本人账号走 onboard 核销。
func (m *MarketClient) InviteToOrg(orgSlug, account, jobTitle string) error {
	if err := m.ensureIdentity(); err != nil {
		return err
	}
	return m.postJSON(m.apiBase+"/orgs/"+url.PathEscape(orgSlug)+"/invites",
		map[string]string{"account": account, "job_title": jobTitle}, nil)
}

// RemoveOrgMemberAt 移除单个成员（DELETE /orgs/{slug}/members/{member_id}；
// 软删保留快照历史，坐席解绑空出；owner 行不可移除，admin 只能移除 member）。
func (m *MarketClient) RemoveOrgMemberAt(orgSlug string, memberID int64) error {
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/orgs/%s/members/%d", m.apiBase, url.PathEscape(orgSlug), memberID), nil)
	if err != nil {
		return err
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
		return apiErrorMessage(data, resp.StatusCode)
	}
	return nil
}
