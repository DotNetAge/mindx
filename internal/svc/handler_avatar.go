package svc

// 内置头像库（avatar.list / avatar.get）。
//
// 数据源是随安装分发的 ~/.mindx/assets/{avatars,teams}（256×256 JPEG，由
// scripts/sync-avatar-library.py 从市场仓头像库存派生）。这里只做两件事：
// 清单（带 64px 缩略，避免一次传上百张 256 图）+ 单张取原图。
//
// 选图本身不在此处：agent.update / team.update 收到 icon 字符串（data URI）后
// 原样写进 frontmatter。头像库只是推荐池，不强制、不做引用式存储——包要能单独
// 分发，icon 必须自带字节。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/draw"

	"github.com/DotNetAge/mindx/internal/core"
)

const (
	avatarGroupAgent  = "agent"
	avatarGroupTeam   = "team"
	avatarThumbSize   = 64
	avatarThumbQ      = 70
	avatarDataPrefix  = "data:image/jpeg;base64,"
	avatarGroupAgentD = "avatars"
	avatarGroupTeamD  = "teams"
)

// 只允许由文件名派生的 id：拒绝路径分隔与点，杜绝 ../ 穿越。
var avatarIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type avatarListParams struct {
	Group string `json:"group,omitempty"`
}

type avatarItem struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	// Thumb 64×64 JPEG data URI：浏览网格用，避免一次传整张 256 图。
	Thumb string `json:"thumb"`
}

type avatarListResult struct {
	Group string       `json:"group"`
	Items []avatarItem `json:"items"`
}

type avatarGetParams struct {
	Group string `json:"group"`
	ID    string `json:"id"`
}

type avatarGetResult struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	// Data 256×256 JPEG data URI（原图字节，不再重编码）。
	Data string `json:"data"`
}

// 缩略图进程内缓存：静态资产，一次生成后复用，重启即失效。
var avatarThumbCache sync.Map // key "group/id" → data URI

func avatarDir(group string) (string, error) {
	var sub string
	switch group {
	case avatarGroupAgent:
		sub = avatarGroupAgentD
	case avatarGroupTeam:
		sub = avatarGroupTeamD
	case "":
		return "", fmt.Errorf("group 不能为空（agent | team）")
	default:
		return "", fmt.Errorf("未知头像分组 %q（只支持 agent | team）", group)
	}
	return filepath.Join(core.DefaultUserPrefsDir(), "assets", sub), nil
}

func (d *Daemon) handleAvatarList(_ context.Context, params json.RawMessage) (any, error) {
	var p avatarListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	groups := []string{avatarGroupAgent, avatarGroupTeam}
	if p.Group != "" {
		if _, err := avatarDir(p.Group); err != nil {
			return nil, err
		}
		groups = []string{p.Group}
	}

	result := avatarListResult{Group: p.Group, Items: []avatarItem{}}
	for _, g := range groups {
		dir, err := avatarDir(g)
		if err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			// 目录缺失（旧安装未带资产）不报错：返回空清单，界面自行提示。
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".jpg") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if !avatarIDPattern.MatchString(id) {
				continue
			}
			thumb, err := avatarThumb(g, id, filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			result.Items = append(result.Items, avatarItem{ID: id, Group: g, Thumb: thumb})
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].Group != result.Items[j].Group {
			return result.Items[i].Group < result.Items[j].Group
		}
		return naturalLess(result.Items[i].ID, result.Items[j].ID)
	})
	return result, nil
}

func (d *Daemon) handleAvatarGet(_ context.Context, params json.RawMessage) (any, error) {
	var p avatarGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if !avatarIDPattern.MatchString(p.ID) {
		return nil, fmt.Errorf("头像 id 非法：%q", p.ID)
	}
	dir, err := avatarDir(p.Group)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, p.ID+".jpg")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("头像 %s/%s 不存在", p.Group, p.ID)
	}
	return avatarGetResult{ID: p.ID, Group: p.Group, Data: avatarDataPrefix + base64.StdEncoding.EncodeToString(raw)}, nil
}

// avatarThumb 生成 64×64 缩略并缓存；失败返回空串由调用方跳过该张。
func avatarThumb(group, id, path string) (string, error) {
	key := group + "/" + id
	if v, ok := avatarThumbCache.Load(key); ok {
		return v.(string), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	src, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	dst := image.NewRGBA(image.Rect(0, 0, avatarThumbSize, avatarThumbSize))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: avatarThumbQ}); err != nil {
		return "", err
	}
	data := avatarDataPrefix + base64.StdEncoding.EncodeToString(buf.Bytes())
	avatarThumbCache.Store(key, data)
	return data, nil
}

// naturalLess 按「数字段按数值、其余按字典」排序：1,2,10 而不是 1,10,2。
func naturalLess(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		ca, cb := a[ia], b[ib]
		da, db := ca >= '0' && ca <= '9', cb >= '0' && cb <= '9'
		if da && db {
			ja, jb := ia, ib
			for ja < len(a) && a[ja] >= '0' && a[ja] <= '9' {
				ja++
			}
			for jb < len(b) && b[jb] >= '0' && b[jb] <= '9' {
				jb++
			}
			na, nb := strings.TrimLeft(a[ia:ja], "0"), strings.TrimLeft(b[ib:jb], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			ia, ib = ja, jb
			continue
		}
		if ca != cb {
			return ca < cb
		}
		ia++
		ib++
	}
	return len(a) < len(b)
}
