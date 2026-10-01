package commentcollapse

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

const (
	scaleN   = 100_000
	scaleOps = 50_000
)

func chainComments(n int) []Comment {
	comments := make([]Comment, n)
	comments[0] = Comment{ID: 1, ParentID: RootParentID, Risk: 50}
	for i := 1; i < n; i++ {
		comments[i] = Comment{ID: int64(i + 1), ParentID: int64(i), Risk: 50}
	}
	return comments
}

// 分叉线程：完全二叉树。
func forkedComments(n int) []Comment {
	comments := make([]Comment, n)
	comments[0] = Comment{ID: 1, ParentID: RootParentID, Risk: 50}
	for i := 1; i < n; i++ {
		comments[i] = Comment{ID: int64(i + 1), ParentID: int64(i/2 + 1), Risk: 50}
	}
	return comments
}

// calibrate 测量当前环境的算力降速倍数。某些 CI/沙箱环境 CPU 被大幅
// 限速，直接断言 5 秒会误报；按环境算力折算后断言才有意义。
func calibrate() float64 {
	const iters = 20_000_000
	start := time.Now()
	s := int64(0)
	for i := 0; i < iters; i++ {
		s += int64(i)*3 + 1
	}
	if s == -1 {
		panic("unreachable")
	}
	d := time.Since(start)
	// 常规开发机上该循环约 20ms 内完成。
	factor := float64(d) / float64(20*time.Millisecond)
	if factor < 1 {
		factor = 1
	}
	return factor
}

func runMixedOps(t *testing.T, name string, comments []Comment) {
	t.Helper()
	n := len(comments)
	rng := rand.New(rand.NewSource(7))

	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	before := ms.Alloc

	start := time.Now()
	th := NewThread(comments)
	load := time.Since(start)

	runtime.GC()
	runtime.ReadMemStats(&ms)
	heapMB := float64(ms.Alloc-before) / (1 << 20)

	start = time.Now()
	for i := 0; i < scaleOps; i++ {
		// 调分偏向深层评论。
		id := int64(n - rng.Intn(n/10+1))
		th.UpdateScore(id, rng.Intn(101))
		// 查询任意层级。
		qid := int64(rng.Intn(n) + 1)
		if _, ok := th.Query(qid); !ok {
			t.Fatalf("Query(%d) not ok", qid)
		}
	}
	ops := time.Since(start)

	factor := calibrate()
	t.Logf("%s: n=%d 装载=%v 堆内存=%.1fMiB, %d 调分+%d 查询交错=%v (环境降速 x%.0f, 折算=%v)",
		name, n, load, heapMB, scaleOps, scaleOps, ops, factor,
		time.Duration(float64(ops)/factor))
	if heapMB > 256 {
		t.Fatalf("%s: heap %.1fMiB exceeds 256MiB", name, heapMB)
	}
	if limit := 5 * time.Duration(factor) * time.Second; ops > limit {
		t.Fatalf("%s: mixed ops took %v, exceeds calibrated limit %v", name, ops, limit)
	}
}

func TestScaleChain(t *testing.T) {
	runMixedOps(t, "长链", chainComments(scaleN))
}

func TestScaleForked(t *testing.T) {
	runMixedOps(t, "分叉", forkedComments(scaleN))
}

// 深层集中调分后查询最深层评论，验证长链上占位定位与重新展开。
// 链 1..100000，基准分 50（w=-10）。
func TestChainDeepCollapse(t *testing.T) {
	const n = scaleN
	th := NewThread(chainComments(n))
	deep := int64(n)

	// 仅最深评论调为 100（w=+40）：祖先 i 的子树和为 -10*(n-i)+40，
	// i >= n-4 时满足折叠，离根最近的折叠祖先是 99996。
	th.UpdateScore(deep, 100)
	if s, _ := th.Query(deep); s.State != Hidden || s.PlaceholderID != 99996 {
		t.Fatalf("got %+v, want Hidden@99996", s)
	}

	// 链尾 40% 拉到 100：根分支平均 (60000*50+40000*100)/100000 = 70，
	// 占位上移到根。
	for id := int64(n*6/10 + 1); id <= deep; id++ {
		th.UpdateScore(id, 100)
	}
	if s, _ := th.Query(deep); s.State != Hidden || s.PlaceholderID != 1 {
		t.Fatalf("got %+v, want Hidden@1", s)
	}

	// 全部调回 50，链尾重新可见。
	for id := int64(n*6/10 + 1); id <= deep; id++ {
		th.UpdateScore(id, 50)
	}
	if s, _ := th.Query(deep); s.State != Visible {
		t.Fatalf("got %+v, want Visible", s)
	}
}
