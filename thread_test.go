package commentcollapse

import (
	"math/rand"
	"testing"
)

// refThread 是按业务规则直接计算的参考实现，用于对拍。
type refThread struct {
	risk     map[int64]int
	parent   map[int64]int64
	children map[int64][]int64
	root     int64
}

func newRefThread(comments []Comment) *refThread {
	r := &refThread{
		risk:     make(map[int64]int),
		parent:   make(map[int64]int64),
		children: make(map[int64][]int64),
	}
	for _, c := range comments {
		r.risk[c.ID] = c.Risk
		r.parent[c.ID] = c.ParentID
		if c.ParentID == RootParentID {
			r.root = c.ID
		} else {
			r.children[c.ParentID] = append(r.children[c.ParentID], c.ID)
		}
	}
	return r
}

func (r *refThread) subtree(id int64) (sum, count int64) {
	sum = int64(r.risk[id])
	count = 1
	for _, c := range r.children[id] {
		s, n := r.subtree(c)
		sum += s
		count += n
	}
	return sum, count
}

func (r *refThread) collapsed(id int64) bool {
	sum, count := r.subtree(id)
	return sum*1 >= collapseThreshold*count
}

func (r *refThread) query(id int64) Status {
	// 收集从根到 id 的严格祖先，按离根从近到远排列。
	var ancestors []int64
	for p := r.parent[id]; p != RootParentID; p = r.parent[p] {
		ancestors = append(ancestors, p)
	}
	for i, j := 0, len(ancestors)-1; i < j; i, j = i+1, j-1 {
		ancestors[i], ancestors[j] = ancestors[j], ancestors[i]
	}
	for _, a := range ancestors {
		if r.collapsed(a) {
			return Status{State: Hidden, PlaceholderID: a}
		}
	}
	if r.collapsed(id) {
		return Status{State: Collapsed, PlaceholderID: id}
	}
	return Status{State: Visible, PlaceholderID: RootParentID}
}

func randomTree(rng *rand.Rand, n int) []Comment {
	comments := make([]Comment, n)
	comments[0] = Comment{ID: 1, ParentID: RootParentID, Risk: rng.Intn(101)}
	for i := 1; i < n; i++ {
		comments[i] = Comment{
			ID:       int64(i + 1),
			ParentID: int64(rng.Intn(i) + 1),
			Risk:     rng.Intn(101),
		}
	}
	return comments
}

func TestDifferentialRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 200; trial++ {
		n := 1 + rng.Intn(60)
		comments := randomTree(rng, n)
		th := NewThread(comments)
		ref := newRefThread(comments)
		for op := 0; op < 300; op++ {
			if rng.Intn(2) == 0 {
				id := int64(rng.Intn(n) + 1)
				risk := rng.Intn(101)
				if !th.UpdateScore(id, risk) {
					t.Fatalf("trial %d: UpdateScore(%d,%d) failed", trial, id, risk)
				}
				ref.risk[id] = risk
			} else {
				id := int64(rng.Intn(n) + 1)
				got, ok := th.Query(id)
				if !ok {
					t.Fatalf("trial %d: Query(%d) not ok", trial, id)
				}
				want := ref.query(id)
				if got != want {
					t.Fatalf("trial %d op %d: Query(%d) = %+v, want %+v", trial, op, id, got, want)
				}
			}
		}
	}
}

func TestBoundaryExactly60(t *testing.T) {
	// 根(60) — 子(60)：两条分支平均分恰好 60，都满足折叠条件。
	th := NewThread([]Comment{
		{ID: 1, ParentID: RootParentID, Risk: 60},
		{ID: 2, ParentID: 1, Risk: 60},
	})
	if s, _ := th.Query(2); s.State != Hidden || s.PlaceholderID != 1 {
		t.Fatalf("got %+v, want Hidden@1", s)
	}
	if s, _ := th.Query(1); s.State != Collapsed || s.PlaceholderID != 1 {
		t.Fatalf("got %+v, want Collapsed@1", s)
	}
	// 调到 59 解除，调回 60 再次折叠。
	th.UpdateScore(2, 59)
	if s, _ := th.Query(1); s.State != Visible {
		t.Fatalf("got %+v, want Visible", s)
	}
	th.UpdateScore(2, 60)
	if s, _ := th.Query(1); s.State != Collapsed {
		t.Fatalf("got %+v, want Collapsed", s)
	}
}

func TestHiddenCommentUpdateTogglesAncestors(t *testing.T) {
	// 链 1-2-3-4，评论 4 被隐藏时调分，多个祖先同时折叠/展开。
	th := NewThread([]Comment{
		{ID: 1, ParentID: RootParentID, Risk: 0},
		{ID: 2, ParentID: 1, Risk: 0},
		{ID: 3, ParentID: 2, Risk: 0},
		{ID: 4, ParentID: 3, Risk: 0},
	})
	th.UpdateScore(4, 100)
	// 各分支平均：4=100, 3=50, 2=33.3, 1=25 —— 只有 4 自身折叠。
	if s, _ := th.Query(4); s.State != Collapsed || s.PlaceholderID != 4 {
		t.Fatalf("got %+v, want Collapsed@4", s)
	}
	if s, _ := th.Query(3); s.State != Visible {
		t.Fatalf("got %+v, want Visible", s)
	}
	// 隐藏的 4 继续调分，使 2、3 同时越过阈值。
	th.UpdateScore(3, 100)
	// 3=100, 2=66.7, 1=50 —— 2 折叠，3、4 被隐藏且占位为 2。
	if s, _ := th.Query(3); s.State != Hidden || s.PlaceholderID != 2 {
		t.Fatalf("got %+v, want Hidden@2", s)
	}
	if s, _ := th.Query(4); s.State != Hidden || s.PlaceholderID != 2 {
		t.Fatalf("got %+v, want Hidden@2", s)
	}
	// 再把 1 拉过阈值，占位上移到 1。
	th.UpdateScore(2, 100)
	// 1=75 —— 全部隐藏到 1。
	if s, _ := th.Query(4); s.State != Hidden || s.PlaceholderID != 1 {
		t.Fatalf("got %+v, want Hidden@1", s)
	}
	// 隐藏评论 4 调低，1、2 同时展开，3 仍折叠。
	th.UpdateScore(4, 0)
	// 1=50, 2=66.7? 2 的子树为 2,3,4 = 100+100+0=200/3=66.7 仍折叠。
	if s, _ := th.Query(4); s.State != Hidden || s.PlaceholderID != 2 {
		t.Fatalf("got %+v, want Hidden@2", s)
	}
	th.UpdateScore(3, 0)
	// 2=33.3, 3=0, 4=0 —— 全部展开。
	if s, _ := th.Query(4); s.State != Visible {
		t.Fatalf("got %+v, want Visible", s)
	}
	if s, _ := th.Query(2); s.State != Visible {
		t.Fatalf("got %+v, want Visible", s)
	}
}

func TestNestedCollapseInnerSurvives(t *testing.T) {
	// 外层解除后内层仍折叠，且占位归属内层。
	th := NewThread([]Comment{
		{ID: 1, ParentID: RootParentID, Risk: 100},
		{ID: 2, ParentID: 1, Risk: 100},
		{ID: 3, ParentID: 2, Risk: 0},
	})
	// 1=66.7 折叠，2=50 不折叠。
	if s, _ := th.Query(3); s.State != Hidden || s.PlaceholderID != 1 {
		t.Fatalf("got %+v, want Hidden@1", s)
	}
	// 拉低 1 使外层展开，但 2 的分支仍满足折叠。
	th.UpdateScore(1, 0)
	// 1 的子树为 0+100+0，平均 33.3 展开，2 的子树平均 50 不折叠。
	if s, _ := th.Query(3); s.State != Visible {
		t.Fatalf("got %+v, want Visible", s)
	}
	// 构造内层折叠：2 的子树平均过阈值。
	th.UpdateScore(2, 100)
	th.UpdateScore(3, 20)
	// 2 的子树平均 60 恰好折叠，1 的子树平均 40 展开 —— 占位在 2。
	if s, _ := th.Query(3); s.State != Hidden || s.PlaceholderID != 2 {
		t.Fatalf("got %+v, want Hidden@2", s)
	}
	// 把 1 也拉过阈值，占位上移到离根最近的 1。
	th.UpdateScore(1, 100)
	// 1 的子树平均 73.3 折叠。
	if s, _ := th.Query(3); s.State != Hidden || s.PlaceholderID != 1 {
		t.Fatalf("got %+v, want Hidden@1", s)
	}
	th.UpdateScore(1, 0)
	// 1 的子树平均 40 展开，2 仍折叠 —— 占位下移到 2。
	if s, _ := th.Query(3); s.State != Hidden || s.PlaceholderID != 2 {
		t.Fatalf("got %+v, want Hidden@2", s)
	}
	if s, _ := th.Query(2); s.State != Collapsed || s.PlaceholderID != 2 {
		t.Fatalf("got %+v, want Collapsed@2", s)
	}
}

func TestInvalidInput(t *testing.T) {
	th := NewThread([]Comment{{ID: 1, ParentID: RootParentID, Risk: 10}})
	if th.UpdateScore(99, 50) {
		t.Fatal("expected false for unknown id")
	}
	if th.UpdateScore(1, 101) || th.UpdateScore(1, -1) {
		t.Fatal("expected false for out-of-range risk")
	}
	if _, ok := th.Query(99); ok {
		t.Fatal("expected not ok for unknown id")
	}
	// 重复设置相同分值不改变结果。
	th.UpdateScore(1, 80)
	before, _ := th.Query(1)
	th.UpdateScore(1, 80)
	after, _ := th.Query(1)
	if before != after {
		t.Fatalf("idempotent update changed result: %+v -> %+v", before, after)
	}
}
