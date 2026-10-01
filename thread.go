// Package commentcollapse 提供评论线程折叠判定的内存库。
package commentcollapse

// Comment 是线程快照中的一条评论。
type Comment struct {
	ID       int64 // 评论标识，快照内唯一
	ParentID int64 // 父评论标识；根评论固定为 RootParentID
	Risk     int   // 风险分，0 到 100
}

// RootParentID 是根评论的父标识占位值。
const RootParentID int64 = -1

// collapseThreshold 是折叠条件的平均分阈值。
const collapseThreshold = 60

// Display 表示评论的展示状态。
type Display int

const (
	// Visible 表示正常显示。
	Visible Display = iota
	// Collapsed 表示显示自身的折叠占位。
	Collapsed
	// Hidden 表示被某个祖先的折叠占位隐藏。
	Hidden
)

// Status 是单条评论的展示判定结果。
type Status struct {
	State         Display
	PlaceholderID int64 // 实际折叠占位所属评论；Visible 时为 RootParentID
}

// Thread 是装载后的固定线程快照，所有方法须顺序调用。
type Thread struct {
	n     int
	risk  []int
	idOf  []int64
	index map[int64]int
	hld   *hldIndex
}

// NewThread 装载线程快照。输入必须是一棵合法有根树，
// 根评论的 ParentID 为 RootParentID，其余评论的 ParentID 必须存在。
func NewThread(comments []Comment) *Thread {
	n := len(comments)
	t := &Thread{
		n:     n,
		risk:  make([]int, n),
		idOf:  make([]int64, n),
		index: make(map[int64]int, n),
	}
	if n == 0 {
		return t
	}

	pos := make(map[int64]int, n)
	rootPos := -1
	for i, c := range comments {
		pos[c.ID] = i
		if c.ParentID == RootParentID {
			rootPos = i
		}
	}
	rawChildren := make([][]int, n)
	for i, c := range comments {
		if i == rootPos {
			continue
		}
		p := pos[c.ParentID]
		rawChildren[p] = append(rawChildren[p], i)
	}

	// 从根出发重排内部下标，保证根为 0。
	order := make([]int, 0, n)
	queue := []int{rootPos}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		queue = append(queue, rawChildren[u]...)
	}
	remap := make([]int, n)
	for idx, u := range order {
		remap[u] = idx
	}
	children := make([][]int, n)
	for idx, u := range order {
		c := comments[u]
		t.idOf[idx] = c.ID
		t.risk[idx] = c.Risk
		t.index[c.ID] = idx
		for _, v := range rawChildren[u] {
			children[idx] = append(children[idx], remap[v])
		}
	}

	// 初始子树权值和：w = risk - 60，自底向上累加。
	sum := make([]int64, n)
	for i := n - 1; i >= 0; i-- {
		u := order[i]
		idx := remap[u]
		sum[idx] += int64(comments[u].Risk - collapseThreshold)
		if u != rootPos {
			sum[remap[pos[comments[u].ParentID]]] += sum[idx]
		}
	}
	t.hld = newHLDIndex(n, children, sum)
	return t
}

// UpdateScore 替换单条评论的风险分。id 不存在或分值越界时返回 false。
func (t *Thread) UpdateScore(id int64, risk int) bool {
	idx, ok := t.index[id]
	if !ok || risk < 0 || risk > 100 {
		return false
	}
	delta := risk - t.risk[idx]
	if delta == 0 {
		return true
	}
	t.risk[idx] = risk
	t.hld.addPathRoot(idx, int64(delta))
	return true
}

// Query 返回单条评论当前的展示状态。id 不存在时 ok 为 false。
func (t *Thread) Query(id int64) (status Status, ok bool) {
	idx, ok := t.index[id]
	if !ok {
		return Status{}, false
	}
	if p := t.hld.parent[idx]; p >= 0 {
		if a := t.hld.topmostNonNegative(p); a >= 0 {
			return Status{State: Hidden, PlaceholderID: t.idOf[a]}, true
		}
	}
	if t.hld.value(idx) >= 0 {
		return Status{State: Collapsed, PlaceholderID: id}, true
	}
	return Status{State: Visible, PlaceholderID: RootParentID}, true
}
