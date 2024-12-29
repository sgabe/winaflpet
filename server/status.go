package main

const (
	F1 status = 1 << iota
	F2
	F3
	F4
	F5
	F6
	F7
	F8
	F9
	F10
	F11
	F12
	F13
	F14
	F15
	F16
	F17
	F18
	F19
	F20
	F21
	F22
	F23
	F24
	F25
	F26
	F27
	F28
	F29
	F30
	F31
	F32
	F33
	F34
	F35
	F36
	F37
	F38
	F39
	F40
)

type status uint64

func setStatus(b status, flag status) status    { return b | flag }
func clearStatus(b status, flag status) status  { return b &^ flag }
func toggleStatus(b status, flag status) status { return b ^ flag }
func hasStatus(b status, i int) bool            { return b&statusMap[i] != 0 }

var statusMap = map[int]status{
	1:  F1,
	2:  F2,
	3:  F3,
	4:  F4,
	5:  F5,
	6:  F6,
	7:  F7,
	8:  F8,
	9:  F9,
	10: F10,
	11: F11,
	12: F12,
	13: F13,
	14: F14,
	15: F15,
	16: F16,
	17: F17,
	18: F18,
	19: F19,
	20: F20,
	21: F21,
	22: F22,
	23: F23,
	24: F24,
	25: F25,
	26: F26,
	27: F27,
	28: F28,
	29: F29,
	30: F30,
	31: F31,
	32: F32,
	33: F33,
	34: F34,
	35: F35,
	36: F36,
	37: F37,
	38: F38,
	39: F39,
	40: F40,
}
