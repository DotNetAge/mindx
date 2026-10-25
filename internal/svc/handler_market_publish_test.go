package svc

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 发布与投递 handler 的参数校验路径（网络行为由 bundle 层客户端测试覆盖：
// 无 token 拒发、sha 头与请求体形态、路由拼接均已实证）。

// TestMarketPackSkillSummary 验收：market.pack 对技能打包返回清单摘要（不入库）。
func TestMarketPackSkillSummary(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	mustWriteGlobalSkill(t, d, "publish-skill", "发布验收技能")
	res, err := d.handleMarketPack(context.Background(), mustJSON(t, rpc.MarketPackParams{Kind: "skill", Name: "publish-skill"}))
	if err != nil {
		t.Fatalf("market.pack 失败: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("响应形态不符: %T", res)
	}
	if m["kind"] != "skill" || m["name"] != "publish-skill" {
		t.Errorf("摘要身份 = %v/%v", m["kind"], m["name"])
	}
	if m["description"] != "发布验收技能" {
		t.Errorf("摘要描述 = %v", m["description"])
	}
	size, _ := m["size"].(int)
	sha, _ := m["sha256"].(string)
	if size <= 0 || len(sha) != 64 {
		t.Errorf("包大小/摘要 = %v/%q", size, sha)
	}

	// 目标不存在显式报错
	if _, err := d.handleMarketPack(context.Background(), mustJSON(t, rpc.MarketPackParams{Kind: "skill", Name: "ghost"})); err == nil {
		t.Fatal("不存在的技能打包应报错")
	}
	// 非法 kind 显式报错
	if _, err := d.handleMarketPack(context.Background(), mustJSON(t, rpc.MarketPackParams{Kind: "ghost", Name: "x"})); err == nil {
		t.Fatal("非法 kind 打包应报错")
	}
}

// TestMarketPublishValidation 验收：market.publish 的参数校验在网络调用前完成。
func TestMarketPublishValidation(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	mustWriteGlobalSkill(t, d, "publish-v-skill", "发布校验技能")

	// 版本号必填
	if _, err := d.handleMarketPublish(context.Background(), mustJSON(t, rpc.MarketPublishParams{Kind: "skill", Name: "publish-v-skill"})); err == nil {
		t.Fatal("缺 version 应报错")
	}
	// 谱系声明必须 kind+name 成对
	if _, err := d.handleMarketPublish(context.Background(), mustJSON(t, rpc.MarketPublishParams{
		Kind: "skill", Name: "publish-v-skill", Version: "0.1.0", DerivedKind: "skill",
	})); err == nil {
		t.Fatal("谱系声明缺 derived_name 应报错")
	}
	// 谱系指向包自身（自指）在打包校验中拒绝
	if _, err := d.handleMarketPublish(context.Background(), mustJSON(t, rpc.MarketPublishParams{
		Kind: "skill", Name: "publish-v-skill", Version: "0.1.0",
		DerivedKind: "skill", DerivedName: "publish-v-skill",
	})); err == nil {
		t.Fatal("自指谱系声明应报错")
	}
}

// TestMarketUnpublishValidation 验收：market.unpublish 的参数校验在网络调用前完成。
func TestMarketUnpublishValidation(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// kind/name 必填
	if _, err := d.handleMarketUnpublish(context.Background(), mustJSON(t, rpc.MarketUnpublishParams{
		Name: "some-pkg", Version: "1.0.0",
	})); err == nil {
		t.Fatal("缺 kind 应报错")
	}
	if _, err := d.handleMarketUnpublish(context.Background(), mustJSON(t, rpc.MarketUnpublishParams{
		Kind: "skill", Version: "1.0.0",
	})); err == nil {
		t.Fatal("缺 name 应报错")
	}
	// version 必填
	if _, err := d.handleMarketUnpublish(context.Background(), mustJSON(t, rpc.MarketUnpublishParams{
		Kind: "skill", Name: "some-pkg",
	})); err == nil {
		t.Fatal("缺 version 应报错")
	}
}

// TestMarketSubmitValidation 验收：market.submit 的版本必填与自指拦截。
func TestMarketSubmitValidation(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	mustWriteGlobalSkill(t, d, "submit-skill", "投递校验技能")

	// 版本号必填（服务端投递校验链要求）
	if _, err := d.handleMarketSubmit(context.Background(), mustJSON(t, rpc.MarketSubmitParams{Kind: "skill", Name: "submit-skill"})); err == nil {
		t.Fatal("缺 version 应报错")
	}
	// 谱系指向投递目标自身：同名包的进化版不重声明谱系
	if _, err := d.handleMarketSubmit(context.Background(), mustJSON(t, rpc.MarketSubmitParams{
		Kind: "skill", Name: "submit-skill", Version: "0.2.0",
		DerivedKind: "skill", DerivedName: "submit-skill",
	})); err == nil {
		t.Fatal("投递清单自指谱系应报错")
	}
	// 目标本地不存在显式报错（不打到网络）
	if _, err := d.handleMarketSubmit(context.Background(), mustJSON(t, rpc.MarketSubmitParams{Kind: "skill", Name: "ghost", Version: "0.1.0"})); err == nil {
		t.Fatal("不存在的技能投递应报错")
	}
}

// TestMarketSubmissionsValidation 验收：收件箱与决策 RPC 的参数校验。
func TestMarketSubmissionsValidation(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleMarketSubmissionsList(context.Background(), mustJSON(t, rpc.MarketSubmissionsListParams{Kind: "skill"})); err == nil {
		t.Fatal("缺 name 应报错")
	}
	if _, err := d.handleMarketSubmissionDecide(context.Background(), mustJSON(t, rpc.MarketSubmissionDecideParams{Kind: "skill", Name: "x", ID: 0})); err == nil {
		t.Fatal("非法 id 应报错")
	}
}
