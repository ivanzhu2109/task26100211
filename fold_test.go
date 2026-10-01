package threadfold

import (
	"math/rand"
	"testing"
)

// TestDifferentialRandom 在随机树上交错调分与查询，与参考实现逐项对拍。
func TestDifferentialRandom(t *testing.T) {
	for seed := int64(0); seed < 30; seed++ {
		rng := rand.New(rand.NewSource(seed))
		n := 1 + rng.Intn(200)
		comments := randomTree(rng, n)
		fast := NewThread(comments)
		slow := newBruteForce(comments)

		for op := 0; op < 2000; op++ {
			id := int64(rng.Intn(n))
			if rng.Intn(2) == 0 {
				score := rng.Intn(101)
				fast.SetScore(id, score)
				slow.setScore(id, score)
			} else {
				got := fast.Query(id)
				want := slow.query(id)
				if got != want {
					t.Fatalf("seed=%d op=%d id=%d: got %+v, want %+v", seed, op, id, got, want)
				}
			}
		}
	}
}

// TestDifferentialChain 在随机长链上交错调分与查询，与参考实现逐项对拍。
func TestDifferentialChain(t *testing.T) {
	for seed := int64(100); seed < 110; seed++ {
		rng := rand.New(rand.NewSource(seed))
		n := 1 + rng.Intn(120)
		comments := make([]Comment, n)
		for i := 0; i < n; i++ {
			parent := NoParent
			if i > 0 {
				parent = int64(i - 1)
			}
			comments[i] = Comment{ID: int64(i), ParentID: parent, Score: rng.Intn(101)}
		}
		fast := NewThread(comments)
		slow := newBruteForce(comments)

		for op := 0; op < 2000; op++ {
			id := int64(rng.Intn(n))
			if rng.Intn(2) == 0 {
				score := rng.Intn(101)
				fast.SetScore(id, score)
				slow.setScore(id, score)
			} else {
				got := fast.Query(id)
				want := slow.query(id)
				if got != want {
					t.Fatalf("seed=%d op=%d id=%d: got %+v, want %+v", seed, op, id, got, want)
				}
			}
		}
	}
}
