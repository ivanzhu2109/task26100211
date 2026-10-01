package threadfold

// segTree 是支持区间加法与“区间内首个非负值位置”查询的懒标记线段树。
// 每个位置维护的值 = 分支风险分总和 - 60*分支评论数，非负即满足折叠条件。
type segTree struct {
	n    int
	max  []int64
	lazy []int64
}

// newSegTree 以 initial 为每个位置建立线段树。
func newSegTree(initial []int64) *segTree {
	n := len(initial)
	s := &segTree{
		n:    n,
		max:  make([]int64, 4*n+4),
		lazy: make([]int64, 4*n+4),
	}
	if n > 0 {
		s.build(1, 0, n-1, initial)
	}
	return s
}

func (s *segTree) build(node, lo, hi int, initial []int64) {
	if lo == hi {
		s.max[node] = initial[lo]
		return
	}
	mid := (lo + hi) / 2
	s.build(node*2, lo, mid, initial)
	s.build(node*2+1, mid+1, hi, initial)
	s.max[node] = maxInt64(s.max[node*2], s.max[node*2+1])
}

func (s *segTree) apply(node int, delta int64) {
	s.max[node] += delta
	s.lazy[node] += delta
}

func (s *segTree) pushDown(node int) {
	if s.lazy[node] != 0 {
		s.apply(node*2, s.lazy[node])
		s.apply(node*2+1, s.lazy[node])
		s.lazy[node] = 0
	}
}

// add 对闭区间 [l, r] 的每个位置加上 delta。
func (s *segTree) add(l, r int, delta int64) {
	if l > r || s.n == 0 {
		return
	}
	s.addRange(1, 0, s.n-1, l, r, delta)
}

func (s *segTree) addRange(node, lo, hi, l, r int, delta int64) {
	if l <= lo && hi <= r {
		s.apply(node, delta)
		return
	}
	s.pushDown(node)
	mid := (lo + hi) / 2
	if l <= mid {
		s.addRange(node*2, lo, mid, l, r, delta)
	}
	if r > mid {
		s.addRange(node*2+1, mid+1, hi, l, r, delta)
	}
	s.max[node] = maxInt64(s.max[node*2], s.max[node*2+1])
}

// firstNonNegative 返回闭区间 [l, r] 内首个值大于等于 0 的位置，不存在时返回 -1。
func (s *segTree) firstNonNegative(l, r int) int {
	if l > r || s.n == 0 {
		return -1
	}
	return s.findFirst(1, 0, s.n-1, l, r)
}

func (s *segTree) findFirst(node, lo, hi, l, r int) int {
	if hi < l || lo > r || s.max[node] < 0 {
		return -1
	}
	if lo == hi {
		return lo
	}
	s.pushDown(node)
	mid := (lo + hi) / 2
	if res := s.findFirst(node*2, lo, mid, l, r); res != -1 {
		return res
	}
	return s.findFirst(node*2+1, mid+1, hi, l, r)
}

// valueAt 返回单个位置当前的值。
func (s *segTree) valueAt(i int) int64 {
	return s.pointValue(1, 0, s.n-1, i)
}

func (s *segTree) pointValue(node, lo, hi, i int) int64 {
	if lo == hi {
		return s.max[node]
	}
	s.pushDown(node)
	mid := (lo + hi) / 2
	if i <= mid {
		return s.pointValue(node*2, lo, mid, i)
	}
	return s.pointValue(node*2+1, mid+1, hi, i)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
