package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 组织运行面 RPC：架构树（大秘「构建组织」技能的生产通路）与成员管理。
// 组织别名缺省解析同信箱通道（resolveOrgSlug：显式指定优先，否则取第一个在册正常组织）；
// owner/admin 权限由 market 服务端 RequireOrgRole 实时把关，daemon 只做人话代理。

// handleOrgArchitectureGet 处理 market.org.architecture.get：取架构树 XML 原文。
func (d *Daemon) handleOrgArchitectureGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.OrgArchGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	xml, err := d.app.Market().OrgArchitectureGet(orgSlug)
	if err != nil {
		return nil, err
	}
	return map[string]any{"org_slug": orgSlug, "xml": xml}, nil
}

// handleOrgArchitecturePublish 处理 market.org.architecture.publish：发布/覆盖架构树
// （整树替换；服务端校验 org/member 结构与 500 人上限，拒绝时给服务端人话原因）。
func (d *Daemon) handleOrgArchitecturePublish(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.OrgArchPublishParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.XML) == "" {
		return nil, fmt.Errorf("架构树内容为空：请先整理出完整 XML 再发布")
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	if err := d.app.Market().PublishOrgArchitecture(orgSlug, p.XML); err != nil {
		return nil, err
	}
	d.logger.Info("组织架构树已发布", "org_slug", orgSlug)
	return map[string]any{"saved": true, "org_slug": orgSlug}, nil
}

// handleOrgInvite 处理 market.org.invite：按可寻址账号发定向邀请（员工 onboard 核销）。
func (d *Daemon) handleOrgInvite(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.OrgInviteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	account := strings.TrimSpace(p.Account)
	if account == "" {
		return nil, fmt.Errorf("被邀请人的可寻址账号为必填（用户名 / 手机号 / GitHub 名任一）")
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	if err := d.app.Market().InviteToOrg(orgSlug, account, strings.TrimSpace(p.JobTitle)); err != nil {
		return nil, err
	}
	d.logger.Info("组织邀请已发出", "org_slug", orgSlug, "account", account)
	return map[string]any{"org_slug": orgSlug, "account": account, "job_title": strings.TrimSpace(p.JobTitle), "status": "pending"}, nil
}

// handleOrgMemberRemove 处理 market.org.member.remove：移除单个在册成员
// （软删保留快照历史，坐席解绑空出；owner 行不可移除，admin 只能移除 member）。
func (d *Daemon) handleOrgMemberRemove(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.OrgMemberRemoveParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.MemberID <= 0 {
		return nil, fmt.Errorf("成员编号不合法：请用 mindx org members 查看待移除成员的编号")
	}
	orgSlug, err := d.resolveOrgSlug(p.OrgSlug)
	if err != nil {
		return nil, err
	}
	if err := d.app.Market().RemoveOrgMemberAt(orgSlug, p.MemberID); err != nil {
		return nil, err
	}
	d.logger.Info("组织成员已移除", "org_slug", orgSlug, "member_id", p.MemberID)
	return map[string]any{"removed": true, "org_slug": orgSlug, "member_id": p.MemberID}, nil
}
