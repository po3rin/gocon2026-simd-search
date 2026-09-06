# 付録: MaxSim(late interaction)。最初から演算律速な検索方式

本編([workshop.md](../workshop/workshop.md))の Stage 2 の考え方を、別の検索方式に当てはめた実測です。`make bench-maxsim` で再現できます。

本編の Stage 2 は「クエリが 32 本まとめて来る」状況を利用して算術強度を上げました。MaxSim(ColBERT 系)は、クエリを複数のトークンベクトルで表し、score = Σ(クエリトークンごとに文書トークンとの最大内積)を取る検索方式です。文書ベクトルを 1 回運ぶたびに複数の内積を取る、という Stage 2 と同じ構造が、検索方式そのものに含まれています。

```go
// internal/index/maxsim.go — 文書トークン dt はキャッシュ常駐でクエリ Tq 本と内積
score(q, d) = Σ_{qt∈q} max_{dt∈d} dot(qt, dt)
```

## 結果

1 万文書 × 4 トークン、クエリ 16 トークンで測ります。

```bash
$ make bench-maxsim
BenchmarkSearchMaxSimNaive   239 ms/op    2.06 GFLOP/s   8.0 AI(flop/byte)
BenchmarkSearchMaxSimSIMD     42 ms/op   11.7  GFLOP/s   8.0 AI(flop/byte)  ← 5.7x
```

クエリのトークン数を Tq とすると算術強度は Tq/2 = 8 flop/byte で、何もしなくても最初からリッジの右にあります。そのため SIMD が最初から 5.7x 効きます。Stage 2 で行った「算術強度を上げる工夫」が、この検索方式では最初から組み込まれています。
