# threadfold

评论线程折叠判定库。在一份固定线程快照上支持反复调整单条评论风险分，
并快速查询任意评论当前的展示状态。数据全部保存在内存中，供上层社区治理工具直接调用。

## 业务规则

- 一条评论的**讨论分支**包括自身和全部后代；分支平均风险分 **>= 60** 时满足折叠条件（恰好 60 也折叠）。
- 平均分始终包含被隐藏的评论，统计范围不随展示结果改变。
- 查询某条评论时：
  - 若存在满足折叠条件的**祖先**（不含自身），结果为 `HiddenByAncestor`，
    占位为其中**离根最近**的祖先；
  - 否则若自身分支满足折叠条件，结果为 `SelfCollapsed`，占位为自身；
  - 否则结果为 `Visible`。
- 调分是**替换**旧分值；调分完成后后续查询立即反映最新状态，
  包括原来隐藏的评论重新出现、折叠占位上移或下移。

## 使用示例

```go
package main

import (
	"fmt"

	"threadfold"
)

func main() {
	th := threadfold.NewThread([]threadfold.Comment{
		{ID: 0, ParentID: threadfold.NoParent, Score: 0},
		{ID: 1, ParentID: 0, Score: 100},
		{ID: 2, ParentID: 1, Score: 100},
	})

	fmt.Println(th.Query(2)) // {HiddenByAncestor 1}：被祖先 1 折叠

	th.SetScore(1, 0)        // 替换分值，分支 (0+100)/2 < 60，立即展开
	fmt.Println(th.Query(2)) // {SelfCollapsed 2}：自身分支仍满足折叠条件
}
```

## API

- `NewThread(comments []Comment) *Thread`：装载线程快照。
  输入保证是一棵合法有根树，根评论的 `ParentID` 为 `NoParent`；装载后不增删评论、不改变父子关系。
- `(*Thread).SetScore(id int64, score int)`：将评论 `id` 的风险分替换为 `score`（0 到 100）。
- `(*Thread).Query(id int64) Result`：返回展示状态 `State`
  （`Visible` / `SelfCollapsed` / `HiddenByAncestor`）与实际折叠占位 `PlaceholderID`。

所有调用按顺序执行即可，库本身不做并发同步。

## 实现说明

对每个分支维护判定值 `分支风险分总和 - 60 * 分支评论数`，非负即满足折叠条件。
调分只会改变被调评论全部祖先的判定值（根到节点的路径加法）；
查询需要在根到节点的路径上找离根最近的非负值。
库内使用重链剖分 + 懒标记线段树，单次调分 / 查询均为 `O(log^2 n)`，
10 万条评论、10 万深度下 5 万次调分与 5 万次查询交错执行远快于 5 秒。

## 测试

```sh
go test ./...          # 全部测试（对拍 + 边界用例 + 规模耗时）
go test -run TestScale -v .   # 仅规模测试，查看实测耗时
```

- `fold_test.go`：随机树 / 随机长链上与直接按业务规则计算的参考实现逐项对拍。
- `edge_test.go`：平均分恰好跨越 60、隐藏评论调分导致多个祖先同时展开或折叠、
  多层分支同时折叠且外层解除后内层仍折叠、替换语义、隐藏评论仍计入平均分、分叉线程。
- `scale_test.go`：10 万条评论的长链与分叉线程，各 5 万次调分 + 5 万次查询交错，
  断言总耗时不超过 5 秒并输出实测耗时。
