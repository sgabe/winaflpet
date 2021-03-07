package main

import "strconv"

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
func hasStatus(b status, i int) bool            { return b&statusMap["fuzzer"+strconv.Itoa(i)] != 0 }

var statusMap = map[string]status{
	"fuzzer1":  F1,
	"fuzzer2":  F2,
	"fuzzer3":  F3,
	"fuzzer4":  F4,
	"fuzzer5":  F5,
	"fuzzer6":  F6,
	"fuzzer7":  F7,
	"fuzzer8":  F8,
	"fuzzer9":  F9,
	"fuzzer10": F10,
	"fuzzer11": F11,
	"fuzzer12": F12,
	"fuzzer13": F13,
	"fuzzer14": F14,
	"fuzzer15": F15,
	"fuzzer16": F16,
	"fuzzer17": F17,
	"fuzzer18": F18,
	"fuzzer19": F19,
	"fuzzer20": F20,
}
