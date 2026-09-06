//go:build goexperiment.simd && amd64

package vec

import "simd/archsimd"

// clearAVXUpperBits は VZEROUPPER。ポータブル simd 版(dot_portable.go)がベクトルからスカラへ戻る
// 境界で呼ぶ。ポータブル API にはこの後始末が無く、Go 1.27 のコンパイラも自動挿入しないので、
// amd64 だけ archsimd を呼ぶ。
func clearAVXUpperBits() { archsimd.ClearAVXUpperBits() }
