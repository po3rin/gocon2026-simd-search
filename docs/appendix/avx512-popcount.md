# 付録: AVX-512 の SIMD popcount は効かない

本編([workshop.md](../workshop/workshop.md))の Stage 4 で、1bit 量子化後のハミング距離は通常の POPCNT 命令で足り、SIMD 版の popcount(AVX-512 の VPOPCNT)を使っても速くならないと書きました。その実測です。

## 何を試したか

`vec.HammingSIMD` は、`Uint64x4.OnesCount`(VPOPCNTQ 命令)で 4 つの uint64 をまとめて popcount します。この命令は AVX-512 の拡張(AVX512VPOPCNTDQ)で、Codespaces に割り当てられる AMD EPYC 7763 にはありません。AVX-512 のある機械(AWS の c7i など)を自分で用意すれば、`make bench-bonus` で測れます。

## 結果

AVX-512 のある c7i(Sapphire Rapids)で、SIMD 版の `SearchBinarySIMD` は 0.75 ms、通常版の `SearchBinary` は 0.68 ms でした。SIMD 版の方がわずかに遅い結果です。理由は 2 つあります。

- 量子化後の DB(4.8 MB)はキャッシュに乗っていて、popcount の計算で時間を使っていない
- 1 ベクトルが 6 語(uint64 × 6)と短く、SIMD で 4 語ずつまとめる利点が出る前に終わる

AVX-512 の無い CPU では、機能チェックで通常版に切り替える分岐が入るぶん、SIMD 版の方が遅くなります(Codespaces の実測で 1.0 ms 対 0.77 ms)。

計算で詰まっていない所に SIMD を足しても速くならない、という本編の主張の実測例として残してあります。
