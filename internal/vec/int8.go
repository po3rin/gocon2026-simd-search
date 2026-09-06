package vec

import "math"

// QuantizeInt8 は v を対称 int8 量子化する(Stage 3)。
// q[i] = round(v[i] / scale)、scale = maxAbs/127。q[i]*scale でおおよそ v[i] に戻る。
// 1bit と違って大きさの情報が残るので、単体でも Recall が実用域に残る。
// 戻り値はこのベクトルの scale(内積を fp32 の尺度に戻すのに使う)。
func QuantizeInt8(v []float32, out []int8) (scale float32) {
	var maxAbs float32
	for _, x := range v {
		a := x
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}
	if maxAbs == 0 {
		for i := range out {
			out[i] = 0
		}
		return 1
	}
	scale = maxAbs / 127
	inv := 127 / maxAbs
	for i, x := range v {
		q := int32(math.Round(float64(x * inv)))
		if q > 127 {
			q = 127
		}
		if q < -127 {
			q = -127
		}
		out[i] = int8(q)
	}
	return scale
}

// DotInt8Naive は int8 ベクトルの内積(スカラ)。
// int32 に広げてから掛けるので dim が大きくても溢れない(127*127*dim は dim が 13 万まで int32 に収まる)。
func DotInt8Naive(a, b []int8) int32 {
	var s int32
	for i := range a {
		s += int32(a[i]) * int32(b[i])
	}
	return s
}
