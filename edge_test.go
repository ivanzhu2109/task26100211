package threadfold

import "testing"

func queryState(t *testing.T, th *Thread, id int64, wantState State, wantPlaceholder int64) {
	t.Helper()
	got := th.Query(id)
	if got.State != wantState || got.PlaceholderID != wantPlaceholder {
		t.Fatalf("Query(%d) = %+v, want state=%v placeholder=%d", id, got, wantState, wantPlaceholder)
	}
}

// TestExactThreshold 验证平均分恰好等于 60 时折叠，差 1 分时不折叠。
func TestExactThreshold(t *testing.T) {
	// 链 0 -> 1 -> 2，总分 179 < 180，不折叠。
	th := NewThread([]Comment{
		{ID: 0, ParentID: NoParent, Score: 60},
		{ID: 1, ParentID: 0, Score: 60},
		{ID: 2, ParentID: 1, Score: 59},
	})
	queryState(t, th, 0, Visible, NoParent)
	queryState(t, th, 2, Visible, NoParent)

	// 总分 180 = 60*3，恰好达到阈值，必须折叠。
	th.SetScore(2, 60)
	queryState(t, th, 0, SelfCollapsed, 0)
	queryState(t, th, 2, HiddenByAncestor, 0)

	// 降回 59，重新展开。
	th.SetScore(2, 59)
	queryState(t, th, 0, Visible, NoParent)
	queryState(t, th, 2, Visible, NoParent)
}

// TestNestedCollapse 验证多层分支同时折叠，以及外层解除后内层仍折叠。
func TestNestedCollapse(t *testing.T) {
	// 链 0 -> 1 -> 2，全部 100 分，三层分支都满足折叠条件。
	th := NewThread([]Comment{
		{ID: 0, ParentID: NoParent, Score: 100},
		{ID: 1, ParentID: 0, Score: 100},
		{ID: 2, ParentID: 1, Score: 100},
	})
	// 折叠占位归属离根最近的祖先。
	queryState(t, th, 0, SelfCollapsed, 0)
	queryState(t, th, 1, HiddenByAncestor, 0)
	queryState(t, th, 2, HiddenByAncestor, 0)

	// 外层解除：0 的子树平均分降为 (0+0+100)/3 < 60，1 的子树 (0+100)/2 < 60。
	th.SetScore(0, 0)
	th.SetScore(1, 0)
	queryState(t, th, 0, Visible, NoParent)
	queryState(t, th, 1, Visible, NoParent)
	// 内层 2 自身仍满足折叠条件，显示自身占位。
	queryState(t, th, 2, SelfCollapsed, 2)
}

// TestHiddenCommentRescoring 验证调整隐藏评论会立即影响祖先的折叠状态。
func TestHiddenCommentRescoring(t *testing.T) {
	// 链 0 -> 1 -> 2 -> 3，初始 1 的分支折叠，2 和 3 被隐藏。
	th := NewThread([]Comment{
		{ID: 0, ParentID: NoParent, Score: 0},
		{ID: 1, ParentID: 0, Score: 100},
		{ID: 2, ParentID: 1, Score: 100},
		{ID: 3, ParentID: 2, Score: 0},
	})
	queryState(t, th, 0, Visible, NoParent)
	queryState(t, th, 1, SelfCollapsed, 1)
	queryState(t, th, 2, HiddenByAncestor, 1)
	queryState(t, th, 3, HiddenByAncestor, 1)

	// 调高隐藏评论 3 的分值：0 的分支平均分变为 300/4 >= 60，
	// 多个祖先同时满足折叠条件，占位上移到离根最近的 0。
	th.SetScore(3, 100)
	queryState(t, th, 0, SelfCollapsed, 0)
	queryState(t, th, 1, HiddenByAncestor, 0)
	queryState(t, th, 2, HiddenByAncestor, 0)
	queryState(t, th, 3, HiddenByAncestor, 0)

	// 调回后 0 重新展开，占位下移到 1。
	th.SetScore(3, 0)
	queryState(t, th, 0, Visible, NoParent)
	queryState(t, th, 3, HiddenByAncestor, 1)

	// 继续调低 2，1 的分支也展开，被隐藏的评论重新出现。
	th.SetScore(2, 0)
	queryState(t, th, 1, Visible, NoParent)
	queryState(t, th, 2, Visible, NoParent)
	queryState(t, th, 3, Visible, NoParent)
}

// TestReplaceSemantics 验证调分是替换而非追加，重复设置相同分值不改变结果。
func TestReplaceSemantics(t *testing.T) {
	th := NewThread([]Comment{
		{ID: 0, ParentID: NoParent, Score: 0},
		{ID: 1, ParentID: 0, Score: 0},
	})
	th.SetScore(1, 100)
	th.SetScore(1, 100)
	// 若为追加，0 的子树总分会被累加成 200/2 >= 60 而折叠；替换语义下不折叠。
	queryState(t, th, 0, Visible, NoParent)
	queryState(t, th, 1, SelfCollapsed, 1)

	// 替换为低分后立即展开。
	th.SetScore(1, 30)
	queryState(t, th, 1, Visible, NoParent)
}

// TestHiddenStillCounted 验证被隐藏的评论仍计入祖先分支的平均分。
func TestHiddenStillCounted(t *testing.T) {
	// 0 -> 1 -> 2：1 的分支折叠使 2 被隐藏，但 2 的分值仍计入 0 和 1 的统计。
	th := NewThread([]Comment{
		{ID: 0, ParentID: NoParent, Score: 0},
		{ID: 1, ParentID: 0, Score: 100},
		{ID: 2, ParentID: 1, Score: 100},
	})
	// 0 的子树总分 200/3 >= 60，因为隐藏的 2 仍被计入。
	queryState(t, th, 0, SelfCollapsed, 0)
	queryState(t, th, 1, HiddenByAncestor, 0)
	queryState(t, th, 2, HiddenByAncestor, 0)
}

// TestForkedTree 验证分叉线程中各分支互不影响。
func TestForkedTree(t *testing.T) {
	//     0
	//    / \
	//   1   2
	//   |   |
	//   3   4
	th := NewThread([]Comment{
		{ID: 0, ParentID: NoParent, Score: 0},
		{ID: 1, ParentID: 0, Score: 100},
		{ID: 2, ParentID: 0, Score: 0},
		{ID: 3, ParentID: 1, Score: 100},
		{ID: 4, ParentID: 2, Score: 0},
	})
	queryState(t, th, 1, SelfCollapsed, 1)
	queryState(t, th, 3, HiddenByAncestor, 1)
	queryState(t, th, 2, Visible, NoParent)
	queryState(t, th, 4, Visible, NoParent)

	// 抬高另一侧分支，0 的子树总分 400/5 >= 60，整棵树折叠到根。
	th.SetScore(4, 100)
	th.SetScore(2, 100)
	queryState(t, th, 0, SelfCollapsed, 0)
	queryState(t, th, 4, HiddenByAncestor, 0)
	queryState(t, th, 3, HiddenByAncestor, 0)
}
