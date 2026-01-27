## [2026-01-27 02:19] From: yuki To: priya

### Subject
PR Review Request - Round 1

### Body
できた。

```
=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
PASS
```

作成したファイル:
- `test/math.go` - Add関数
- `test/math_test.go` - テーブル駆動テスト


### Action Required
Please review

---
## [2026-01-27 02:21] From: yuki To: priya

### Subject
PR Review Request - Round 1

### Body
できた。

テスト通った。

**実装済みの内容:**

1. `test/math.go` - Add関数
```go
func Add(a, b int) int {
    return a + b
}
```

2. `test/math_test.go` - テーブル駆動テスト
   - 4パターン: `(1,2)`, `(0,0)`, `(-1,1)`, `(100,-50)`

3. `go test` → PASS


### Action Required
Please review

---
