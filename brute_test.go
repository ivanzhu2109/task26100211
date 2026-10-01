package threadfold

import "math/rand"

// bruteForce 是按业务规则直接计算的参考实现，用于对拍。
type bruteForce struct {
	ids      []int64
	indexOf  map[int64]int
	parent   []int
	children [][]int
	score    []int
}

func newBruteForce(comments []Comment) *bruteForce {
	b := &bruteForce{indexOf: make(map[int64]int, len(comments))}
	b.ids = make([]int64, len(comments))
	b.parent = make([]int, len(comments))
	b.children = make([][]int, len(comments))
	b.score = make([]int, len(comments))
	for i, c := range comments {
		b.ids[i] = c.ID
		b.indexOf[c.ID] = i
		b.score[i] = c.Score
	}
	for i, c := range comments {
		if c.ParentID == NoParent {
			b.parent[i] = -1
		} else {
			p := b.indexOf[c.ParentID]
			b.parent[i] = p
			b.children[p] = append(b.children[p], i)
		}
	}
	return b
}

func (b *bruteForce) setScore(id int64, score int) {
	b.score[b.indexOf[id]] = score
}

// collapsed 直接统计 v 的整个分支（含被隐藏评论），判断平均分是否 >= 60。
func (b *bruteForce) collapsed(v int) bool {
	var sum, count int64
	stack := []int{v}
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		sum += int64(b.score[u])
		count++
		stack = append(stack, b.children[u]...)
	}
	return sum >= CollapseThreshold*count
}

func (b *bruteForce) query(id int64) Result {
	v := b.indexOf[id]
	// 枚举全部祖先，取离根最近（深度最浅）的满足折叠条件者。
	best := -1
	bestDepth := -1
	depth := 0
	for u := b.parent[v]; u >= 0; u = b.parent[u] {
		depth++
		if b.collapsed(u) {
			d := 0
			for w := u; b.parent[w] >= 0; w = b.parent[w] {
				d++
			}
			if best == -1 || d < bestDepth {
				best = u
				bestDepth = d
			}
		}
	}
	_ = depth
	if best >= 0 {
		return Result{State: HiddenByAncestor, PlaceholderID: b.ids[best]}
	}
	if b.collapsed(v) {
		return Result{State: SelfCollapsed, PlaceholderID: id}
	}
	return Result{State: Visible, PlaceholderID: NoParent}
}

// randomTree 生成一棵 n 个节点的随机树，评论标识随机打乱。
func randomTree(rng *rand.Rand, n int) []Comment {
	comments := make([]Comment, n)
	perm := rng.Perm(n)
	for i := 0; i < n; i++ {
		parent := NoParent
		if i > 0 {
			parent = int64(perm[rng.Intn(i)])
		}
		comments[i] = Comment{ID: int64(perm[i]), ParentID: parent, Score: rng.Intn(101)}
	}
	return comments
}
