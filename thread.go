// Package threadfold 提供评论线程折叠判定能力。
//
// 在一份固定线程快照上支持反复调整单条评论风险分，
// 并快速查询任意评论当前的展示状态。
package threadfold

// NoParent 表示根评论的父评论标识。
const NoParent int64 = -1

// CollapseThreshold 是分支平均分折叠阈值（百分制）。
const CollapseThreshold = 60

// Comment 是快照中的一条评论。
type Comment struct {
	ID       int64 // 评论标识，快照内唯一
	ParentID int64 // 父评论标识，根评论为 NoParent
	Score    int   // 风险分，0 到 100
}

// State 表示一条评论的展示状态。
type State int

const (
	// Visible 表示正常显示。
	Visible State = iota
	// SelfCollapsed 表示自身分支满足折叠条件，显示自身的折叠占位。
	SelfCollapsed
	// HiddenByAncestor 表示存在满足折叠条件的祖先，评论被隐藏。
	HiddenByAncestor
)

// Result 是单条评论的展示判定结果。
type Result struct {
	State State
	// PlaceholderID 是用户实际看到的折叠占位评论标识。
	// State 为 Visible 时为 NoParent；
	// State 为 SelfCollapsed 时为评论自身；
	// State 为 HiddenByAncestor 时为离根最近的满足折叠条件的祖先。
	PlaceholderID int64
}

// Thread 是一份装载后的固定线程快照。
type Thread struct {
	n       int
	ids     []int64
	indexOf map[int64]int
	parent  []int
	depth   []int
	size    []int
	heavy   []int
	head    []int
	pos     []int
	score   []int
	inv     []int
	// threshold 按剖分位置存放各分支初始判定值（分支总分 - 60*分支评论数）。
	threshold []int64
	seg       *segTree
}

// NewThread 装载线程快照。输入保证是一棵合法有根树。
func NewThread(comments []Comment) *Thread {
	t := &Thread{}
	t.buildIndex(comments)
	t.decompose()
	t.seg = newSegTree(t.threshold)
	return t
}

// SetScore 将评论 id 的风险分替换为 score（0 到 100）。
// 调分完成后，后续查询立即反映最新状态。
func (t *Thread) SetScore(id int64, score int) {
	v, ok := t.indexOf[id]
	if !ok {
		return
	}
	delta := int64(score - t.score[v])
	if delta == 0 {
		return
	}
	t.score[v] = score
	t.addPathToRoot(v, delta)
}

// Query 返回评论 id 当前的展示判定结果。
func (t *Thread) Query(id int64) Result {
	v, ok := t.indexOf[id]
	if !ok {
		return Result{State: Visible, PlaceholderID: NoParent}
	}
	if p := t.parent[v]; p >= 0 {
		if u := t.shallowestCollapsed(p); u >= 0 {
			return Result{State: HiddenByAncestor, PlaceholderID: t.ids[u]}
		}
	}
	if t.seg.valueAt(t.pos[v]) >= 0 {
		return Result{State: SelfCollapsed, PlaceholderID: id}
	}
	return Result{State: Visible, PlaceholderID: NoParent}
}

// addPathToRoot 对根到 v 路径上每个分支的判定值加上 delta。
func (t *Thread) addPathToRoot(v int, delta int64) {
	for {
		h := t.head[v]
		t.seg.add(t.pos[h], t.pos[v], delta)
		if h == t.head[0] {
			return
		}
		v = t.parent[h]
	}
}

// shallowestCollapsed 返回根到 v 路径上离根最近的满足折叠条件的节点，不存在时返回 -1。
func (t *Thread) shallowestCollapsed(v int) int {
	// 自根向下逐链检查，链内位置随深度递增，首个命中即离根最近。
	u := v
	var segs [][2]int
	for {
		h := t.head[u]
		segs = append(segs, [2]int{t.pos[h], t.pos[u]})
		if h == t.head[0] {
			break
		}
		u = t.parent[h]
	}
	for i := len(segs) - 1; i >= 0; i-- {
		if hit := t.seg.firstNonNegative(segs[i][0], segs[i][1]); hit >= 0 {
			return t.nodeAt(hit)
		}
	}
	return -1
}

// nodeAt 返回剖分位置 p 对应的节点下标。
func (t *Thread) nodeAt(p int) int {
	return t.inv[p]
}
