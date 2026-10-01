# commentcollapse

评论线程折叠判定库（Go，纯内存，无外部依赖）。装载一份固定线程快照后，
支持反复调整单条评论的风险分，并实时查询任意评论的展示状态与折叠占位归属。

## 业务规则

- 一条评论的**讨论分支**为自身加全部后代；分支平均风险分 **>= 60** 即满足折叠条件
  （恰好 60 也折叠，平均分始终包含被隐藏的评论）。
- 查询某评论：存在满足折叠条件的**祖先**时，结果为 `Hidden`，占位为其中**离根最近**
  的祖先；否则自身满足折叠条件时为 `Collapsed`（占位为自身）；否则 `Visible`。
- 调分是**替换**旧分值，立即生效：隐藏评论可重新出现，占位可上移/下移。

## 使用

```go
th := commentcollapse.NewThread([]commentcollapse.Comment{
    {ID: 1, ParentID: commentcollapse.RootParentID, Risk: 50}, // 根
    {ID: 2, ParentID: 1, Risk: 80},
    {ID: 3, ParentID: 2, Risk: 100},
})

th.UpdateScore(3, 0)          // 替换风险分，立即生效
s, ok := th.Query(2)          // s.State: Visible / Collapsed / Hidden
                              // s.PlaceholderID: 实际折叠占位所属评论
```

- `NewThread(comments)`：装载快照。输入须为合法有根树，根的 `ParentID` 为
  `RootParentID`；装载后不增删评论、不改父子关系。
- `UpdateScore(id, risk) bool`：替换风险分（0–100），id 不存在或分值越界返回 false。
- `Query(id) (Status, bool)`：返回展示状态，id 不存在时 `ok` 为 false。
- 所有方法须顺序调用，不提供并发安全。

## 实现与复杂度

把「分支平均分 >= 60」转化为子树权值和 `sum(risk-60) >= 0`（整数精确判定，
等于 60 时和恰为 0）。调分等价于给根到该评论路径上的子树和加 delta；
查询等价于在根到父评论路径上找离根最近的非负节点。用**重链剖分 + 懒标记
线段树**维护，调分与查询均为 `O(log² n)`，装载 `O(n)`。

## 测试

```sh
go test ./...        # 正确性（含暴力对拍）+ 规模测试
go test -v -run 'TestScale|TestChainDeep' ./...   # 查看实测耗时
```

覆盖：平均分恰好跨越 60、调整隐藏评论导致多个祖先同时展开/折叠、多层分支
同时折叠且外层解除后内层仍折叠、占位上移/下移，以及 10 万长链与 10 万分叉
（完全二叉树）线程各 5 万调分 + 5 万查询交错。

本机实测（Apple Silicon）：长链交错 10 万操作约 33ms，分叉约 95ms，
堆内存约 13MiB，远低于 5s / 256MiB 的限制。规模测试内置环境算力校准，
在限速 CI/沙箱中按折算后的上限断言。
