package commentcollapse

// hldIndex 维护重链剖分下的子树权值和，支持：
// 根到某点路径整体加 delta、路径上离根最近的非负位置查询、单点取值。
type hldIndex struct {
	n      int
	parent []int
	head   []int
	dfn    []int
	rev    []int
	max    []int64
	lazy   []int64
	segs   []seg // topmostNonNegative 复用的临时缓冲
}

type seg struct{ l, r int }

// newHLDIndex 在给定树结构上建索引，根节点为 0，w 为每个节点的初始权值。
func newHLDIndex(n int, children [][]int, w []int64) *hldIndex {
	h := &hldIndex{
		n:      n,
		parent: make([]int, n),
		head:   make([]int, n),
		dfn:    make([]int, n),
		rev:    make([]int, n),
	}
	size := make([]int, n)
	heavy := make([]int, n)
	depth := make([]int, n)

	// 迭代先序遍历，避免深链递归。
	order := make([]int, 0, n)
	stack := []int{0}
	h.parent[0] = -1
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		order = append(order, u)
		for _, c := range children[u] {
			h.parent[c] = u
			depth[c] = depth[u] + 1
			stack = append(stack, c)
		}
	}
	for i := n - 1; i >= 0; i-- {
		u := order[i]
		size[u] = 1
		best := -1
		for _, c := range children[u] {
			size[u] += size[c]
			if best == -1 || size[c] > size[best] {
				best = c
			}
		}
		heavy[u] = best
	}
	timer := 0
	type frame struct{ u, head int }
	stack2 := []frame{{0, 0}}
	for len(stack2) > 0 {
		f := stack2[len(stack2)-1]
		stack2 = stack2[:len(stack2)-1]
		// 沿重链一次性展开。
		for u, hd := f.u, f.head; u != -1; u = heavy[u] {
			h.head[u] = hd
			h.dfn[u] = timer
			h.rev[timer] = u
			timer++
			for _, c := range children[u] {
				if c != heavy[u] {
					stack2 = append(stack2, frame{c, c})
				}
			}
		}
	}

	h.max = make([]int64, 4*n)
	h.lazy = make([]int64, 4*n)
	base := make([]int64, n)
	for i := 0; i < n; i++ {
		base[h.dfn[i]] = w[i]
	}
	h.build(1, 0, n-1, base)
	return h
}

func (h *hldIndex) build(p, l, r int, base []int64) {
	if l == r {
		h.max[p] = base[l]
		return
	}
	m := (l + r) >> 1
	h.build(p<<1, l, m, base)
	h.build(p<<1|1, m+1, r, base)
	h.max[p] = max(h.max[p<<1], h.max[p<<1|1])
}

func (h *hldIndex) apply(p int, delta int64) {
	h.max[p] += delta
	h.lazy[p] += delta
}

func (h *hldIndex) push(p int) {
	if h.lazy[p] != 0 {
		h.apply(p<<1, h.lazy[p])
		h.apply(p<<1|1, h.lazy[p])
		h.lazy[p] = 0
	}
}

func (h *hldIndex) rangeAdd(p, l, r, ql, qr int, delta int64) {
	if ql <= l && r <= qr {
		h.apply(p, delta)
		return
	}
	h.push(p)
	m := (l + r) >> 1
	if ql <= m {
		h.rangeAdd(p<<1, l, m, ql, qr, delta)
	}
	if qr > m {
		h.rangeAdd(p<<1|1, m+1, r, ql, qr, delta)
	}
	h.max[p] = max(h.max[p<<1], h.max[p<<1|1])
}

// firstNonNegative 返回 [ql,qr] 内最左且值 >= 0 的位置，不存在返回 -1。
func (h *hldIndex) firstNonNegative(p, l, r, ql, qr int) int {
	if h.max[p] < 0 {
		return -1
	}
	if l == r {
		return l
	}
	h.push(p)
	m := (l + r) >> 1
	res := -1
	if ql <= m {
		res = h.firstNonNegative(p<<1, l, m, ql, qr)
	}
	if res == -1 && qr > m {
		res = h.firstNonNegative(p<<1|1, m+1, r, ql, qr)
	}
	return res
}

// addPathRoot 给根到 u 的路径上所有节点加 delta。
func (h *hldIndex) addPathRoot(u int, delta int64) {
	for u != -1 {
		h.rangeAdd(1, 0, h.n-1, h.dfn[h.head[u]], h.dfn[u], delta)
		u = h.parent[h.head[u]]
	}
}

// topmostNonNegative 在根到 u 的路径上找离根最近且值 >= 0 的节点，
// 不存在时返回 -1。
func (h *hldIndex) topmostNonNegative(u int) int {
	segs := h.segs[:0]
	for v := u; v != -1; v = h.parent[h.head[v]] {
		segs = append(segs, seg{h.dfn[h.head[v]], h.dfn[v]})
	}
	h.segs = segs
	for i := len(segs) - 1; i >= 0; i-- {
		if pos := h.firstNonNegative(1, 0, h.n-1, segs[i].l, segs[i].r); pos != -1 {
			return h.rev[pos]
		}
	}
	return -1
}

// value 返回单点当前值。
func (h *hldIndex) value(u int) int64 {
	pos := h.dfn[u]
	p, l, r := 1, 0, h.n-1
	for l != r {
		h.push(p)
		m := (l + r) >> 1
		if pos <= m {
			p, r = p<<1, m
		} else {
			p, l = p<<1|1, m+1
		}
	}
	return h.max[p]
}
