package sample

//go:generate const_gen -type=SampleInt -output=stringer_string.go -trimprefix=COUNTER_
type SampleInt int

// 123123sjdasio
const (
	COUNTER_A SampleInt = iota + 100 // 111111
	COUNTER_B                        // 222222
	COUNTER_C                        // 333333
	COUNTER_D                        // 444444
)

//go:generate const_gen -type=SampleInt2 -output=stringer_string.go -linecomment -trimprefix=COUNTER1_
type SampleInt2 int

const (
	COUNTER1_A SampleInt2 = iota
	COUNTER1_B
	COUNTER1_C
	COUNTER1_D
	COUNTER1_E
	COUNTER1_F
	COUNTER1_G
	COUNTER1_H
	COUNTER1_I
	COUNTER1_J
	COUNTER1_K
	COUNTER1_L
	COUNTER1_M
)

const (
	COUNTER2_A = 1<<iota + 2
	COUNTER2_B
	COUNTER2_C
	COUNTER2_D
)
