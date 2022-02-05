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
}
