package threadfold

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

const (
	scaleN    = 100000
	scaleOps  = 50000
	scaleSeed = 42
)

// scaleBudget 返回规模测试的耗时上限，默认 5 秒。
// 共享机器负载过高导致环境性超限时，可用 THREADFOLD_SCALE_BUDGET 临时放宽，
// 例如 THREADFOLD_SCALE_BUDGET=30s go test -run TestScale -v .
func scaleBudget() time.Duration {
	if v := os.Getenv("THREADFOLD_SCALE_BUDGET"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 5 * time.Second
}

// runMixedOps 在给定快照上交错执行 5 万次调分与 5 万次查询，返回总耗时。
func runMixedOps(t *testing.T, comments []Comment) time.Duration {
	t.Helper()
	th := NewThread(comments)
	rng := rand.New(rand.NewSource(scaleSeed))
	n := int64(len(comments))

	start := time.Now()
	for i := 0; i < scaleOps; i++ {
		// 调分集中在深层评论。
		deep := n/2 + rng.Int63n(n-n/2)
		th.SetScore(deep, rng.Intn(101))
		// 查询任意层级。
		th.Query(rng.Int63n(n))
	}
	return time.Since(start)
}

// TestScaleChain 验证 10 万条评论的长链在 5 万次调分 + 5 万次查询下满足耗时要求。
func TestScaleChain(t *testing.T) {
	comments := make([]Comment, scaleN)
	for i := 0; i < scaleN; i++ {
		parent := NoParent
		if i > 0 {
			parent = int64(i - 1)
		}
		comments[i] = Comment{ID: int64(i), ParentID: parent, Score: i % 101}
	}
	elapsed := runMixedOps(t, comments)
	t.Logf("chain n=%d: 50k SetScore + 50k Query took %v", scaleN, elapsed)
	if elapsed > scaleBudget() {
		t.Fatalf("chain scale test too slow: %v", elapsed)
	}
}

// TestScaleForked 验证 10 万条评论的分叉线程（二叉树）满足耗时要求。
func TestScaleForked(t *testing.T) {
	comments := make([]Comment, scaleN)
	for i := 0; i < scaleN; i++ {
		parent := NoParent
		if i > 0 {
			parent = int64((i - 1) / 2)
		}
		comments[i] = Comment{ID: int64(i), ParentID: parent, Score: i % 101}
	}
	elapsed := runMixedOps(t, comments)
	t.Logf("forked n=%d: 50k SetScore + 50k Query took %v", scaleN, elapsed)
	if elapsed > scaleBudget() {
		t.Fatalf("forked scale test too slow: %v", elapsed)
	}
}
