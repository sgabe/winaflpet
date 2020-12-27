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
)

type status uint8

func setStatus(b status, flag status) status    { return b | flag }
func clearStatus(b status, flag status) status  { return b &^ flag }
func toggleStatus(b status, flag status) status { return b ^ flag }
func hasStatus(b status, i int) bool            { return b&statusMap["fuzzer"+strconv.Itoa(i)] != 0 }

var statusMap = map[string]status{
	"fuzzer1": F1,
	"fuzzer2": F2,
	"fuzzer3": F3,
	"fuzzer4": F4,
	"fuzzer5": F5,
	"fuzzer6": F6,
	"fuzzer7": F7,
	"fuzzer8": F8,
}
