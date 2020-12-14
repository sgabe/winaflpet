// +build windows

package main

type Stats struct {
	StartTime       int     `json:"start_time"`
	LastUpdate      int     `json:"last_update"`
	FuzzerProcessID int     `json:"fuzzer_pid"`
	CyclesDone      int     `json:"cycles_done"`
	ExecsDone       int     `json:"execs_done"`
	ExecsPerSec     float64 `json:"execs_per_sec"`
	PathsTotal      int     `json:"paths_total"`
	PathsFavored    int     `json:"paths_favored"`
	PathsFound      int     `json:"paths_found"`
	PathsImported   int     `json:"paths_imported"`
	MaxDepth        int     `json:"max_depth"`
	CurPath         int     `json:"cur_path"`
	PendingFavs     int     `json:"pending_favs"`
	PendingTotal    int     `json:"pending_total"`
	VariablePaths   int     `json:"variable_paths"`
	Stability       string  `json:"stability"`
	BitmapCvg       string  `json:"bitmap_cvg"`
	UniqueCrashes   int     `json:"unique_crashes"`
	UniqueHangs     int     `json:"unique_hangs"`
	LastPath        int     `json:"last_path"`
	LastCrash       int     `json:"last_crash"`
	LastHang        int     `json:"last_hang"`
	ExecsSinceCrash int     `json:"execs_since_crash"`
	ExecTimeout     int     `json:"exec_timeout"`
	AFLBanner       string  `json:"afl_banner"`
	AFLVersion      string  `json:"afl_version"`
}
