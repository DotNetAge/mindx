package conv

import "time"

// FoldState 折叠 UI 态（对齐 Desktop TreeView.vue 的折叠策略）。
type FoldState struct {
	Overrides map[string]bool
	AutoOpen  map[string]bool
}

func (f *FoldState) Expanded(node TreeNode) bool {
	if f == nil {
		return !node.Base().FoldDefault
	}
	if f.Overrides == nil {
		f.Overrides = map[string]bool{}
	}
	if v, ok := f.Overrides[node.Base().ID]; ok {
		return v
	}
	if _, active := f.AutoOpen[node.Base().ID]; active {
		return true
	}
	return !node.Base().FoldDefault
}

func (f *FoldState) Toggle(id string, current bool) {
	if f == nil {
		return
	}
	if f.Overrides == nil {
		f.Overrides = map[string]bool{}
	}
	f.Overrides[id] = !current
}

// advanceShimmers 推进所有 executing 节点的流光位置。
func (s *Stream) advanceShimmers() {
	tickMs := int(tickInterval / time.Millisecond)
	if s.Shimmers == nil {
		s.Shimmers = map[string]*Shimmer{}
	}
	if s.Nodes == nil {
		return
	}

	for _, node := range s.Nodes {
		b := node.Base()
		if b.Status == NodeExecuting {
			if sh, ok := s.Shimmers[b.ID]; ok {
				s.Shimmers[b.ID] = sh.Advance(tickMs)
			} else {
				if pres, ok := Presentations[b.Kind]; ok && pres.Executing != nil {
					s.Shimmers[b.ID] = NewShimmer(pres.Executing(node))
				} else if pres, ok := Presentations[b.Kind]; ok && pres.Verb != nil {
					verb := pres.Verb(node)
					if verb != "" {
						s.Shimmers[b.ID] = NewShimmer(verb)
					}
				}
			}
		} else {
			delete(s.Shimmers, b.ID)
		}
	}
}

// EnsureNodes 懒构建 Nodes（Phase 4 ViewTree 调用）。
func (s *Stream) EnsureNodes() {
	if s.buildState == nil {
		s.buildState = NewBuildState(s.CreatedAt)
	}
	if s.FoldState == nil {
		s.FoldState = &FoldState{Overrides: map[string]bool{}}
	}
	if len(s.Items) > 0 {
		isFinal := s.Status == StatusDone
		s.Nodes = BuildNodes(s.Items, s.buildState, isFinal)
	}
}
