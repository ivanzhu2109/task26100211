package threadfold

// buildIndex 建立评论标识到内部下标的映射，并记录每条评论的父节点下标。
func (t *Thread) buildIndex(comments []Comment) {
	n := len(comments)
	t.n = n
	t.ids = make([]int64, n)
	t.indexOf = make(map[int64]int, n)
	t.parent = make([]int, n)
	t.score = make([]int, n)
	for i, c := range comments {
		t.ids[i] = c.ID
		t.indexOf[c.ID] = i
		t.score[i] = c.Score
	}
	for i, c := range comments {
		if c.ParentID == NoParent {
			t.parent[i] = -1
		} else {
			t.parent[i] = t.indexOf[c.ParentID]
		}
	}
}

// decompose 计算深度、子树大小、重儿子与重链剖分，
// 并按剖分位置初始化每个分支的判定值（分支总分 - 60*分支评论数）。
// 线程深度可达 10 万，这里全部使用迭代避免递归过深。
func (t *Thread) decompose() {
	n := t.n
	t.depth = make([]int, n)
	t.size = make([]int, n)
	t.heavy = make([]int, n)
	t.head = make([]int, n)
	t.pos = make([]int, n)
	t.inv = make([]int, n)
	t.threshold = make([]int64, n)
	if n == 0 {
		return
	}

	// 孩子表（CSR）。
	childStart := make([]int, n+1)
	for _, p := range t.parent {
		if p >= 0 {
			childStart[p+1]++
		}
	}
	for i := 0; i < n; i++ {
		childStart[i+1] += childStart[i]
	}
	children := make([]int, n-1)
	cursor := make([]int, n+1)
	copy(cursor, childStart)
	root := -1
	for v, p := range t.parent {
		if p < 0 {
			root = v
		} else {
			children[cursor[p]] = v
			cursor[p]++
		}
	}

	// 先序遍历，同时记录深度。
	order := make([]int, 0, n)
	stack := []int{root}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		order = append(order, v)
		for i := childStart[v]; i < childStart[v+1]; i++ {
			c := children[i]
			t.depth[c] = t.depth[v] + 1
			stack = append(stack, c)
		}
	}

	// 逆序累加子树大小、子树总分与重儿子。
	subSum := make([]int64, n)
	for i := n - 1; i >= 0; i-- {
		v := order[i]
		t.size[v] = 1
		subSum[v] = int64(t.score[v])
		t.heavy[v] = -1
		maxSize := 0
		for j := childStart[v]; j < childStart[v+1]; j++ {
			c := children[j]
			t.size[v] += t.size[c]
			subSum[v] += subSum[c]
			if t.size[c] > maxSize {
				maxSize = t.size[c]
				t.heavy[v] = c
			}
		}
	}

	// 重链剖分：沿重链连续编号，轻儿子各自起新链。
	cur := 0
	type item struct{ v, h int }
	stack2 := []item{{root, root}}
	for len(stack2) > 0 {
		it := stack2[len(stack2)-1]
		stack2 = stack2[:len(stack2)-1]
		for v, h := it.v, it.h; v != -1; v = t.heavy[v] {
			t.head[v] = h
			t.pos[v] = cur
			t.inv[cur] = v
			t.threshold[cur] = subSum[v] - CollapseThreshold*int64(t.size[v])
			cur++
			for j := childStart[v]; j < childStart[v+1]; j++ {
				c := children[j]
				if c != t.heavy[v] {
					stack2 = append(stack2, item{c, c})
				}
			}
		}
	}
}
