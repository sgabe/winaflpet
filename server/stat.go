package main

import (
	"github.com/rs/xid"
	"github.com/sgabe/structable"
)

const (
	TB_NAME_STATS   = "stats"
	TB_SCHEMA_STATS = `CREATE TABLE stats (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"jid" INTEGER,
		"guid" TEXT UNIQUE,
		"fuzzer_pid" INTEGER,
		"start_time" INTEGER,
		"last_update" INTEGER,
		"cycles_done" INTEGER,
		"execs_done" INTEGER,
		"execs_per_sec" REAL,
		"paths_total" INTEGER,
		"paths_favored" INTEGER,
		"paths_found" INTEGER,
		"paths_imported" INTEGER,
		"max_depth" INTEGER,
		"cur_path" INTEGER,
		"pending_favs" INTEGER,
		"pending_total" INTEGER,
		"variable_paths" INTEGER,
		"stability" TEXT,
		"bitmap_cvg" TEXT,
		"unique_crashes" INTEGER,
		"unique_hangs" INTEGER,
		"last_path" INTEGER,
		"last_crash" INTEGER,
		"last_hang" INTEGER,
		"execs_since_crash" INTEGER,
		"exec_timeout" INTEGER,
		"afl_banner" TEXT,
		"afl_version" TEXT,
		FOREIGN KEY (jid) REFERENCES jobs(id)
	  );`
)

type Stat struct {
	structable.Recorder
	ID              int     `stbl:"id, PRIMARY_KEY, AUTO_INCREMENT"`
	JobID           int     `json:"jid" stbl:"jid"`
	GUID            xid.ID  `json:"guid" stbl:"guid"`
	FuzzerProcessID int     `json:"fuzzer_pid" stbl:"fuzzer_pid"`
	StartTime       int     `json:"start_time" stbl:"start_time"`
	LastUpdate      int     `json:"last_update" stbl:"last_update"`
	CyclesDone      int     `json:"cycles_done" stbl:"cycles_done"`
	ExecsDone       int     `json:"execs_done" stbl:"execs_done"`
	ExecsPerSec     float64 `json:"execs_per_sec" stbl:"execs_per_sec"`
	PathsTotal      int     `json:"paths_total" stbl:"paths_total"`
	PathsFavored    int     `json:"paths_favored" stbl:"paths_favored"`
	PathsFound      int     `json:"paths_found" stbl:"paths_found"`
	PathsImported   int     `json:"paths_imported" stbl:"paths_imported"`
	MaxDepth        int     `json:"max_depth" stbl:"max_depth"`
	CurPath         int     `json:"cur_path" stbl:"cur_path"`
	PendingFavs     int     `json:"pending_favs" stbl:"pending_favs"`
	PendingTotal    int     `json:"pending_total" stbl:"pending_total"`
	VariablePaths   int     `json:"variable_paths" stbl:"variable_paths"`
	Stability       string  `json:"stability" stbl:"stability"`
	BitmapCvg       string  `json:"bitmap_cvg" stbl:"bitmap_cvg"`
	UniqueCrashes   int     `json:"unique_crashes" stbl:"unique_crashes"`
	UniqueHangs     int     `json:"unique_hangs" stbl:"unique_hangs"`
	LastPath        int     `json:"last_path" stbl:"last_path"`
	LastCrash       int     `json:"last_crash" stbl:"last_crash"`
	LastHang        int     `json:"last_hang" stbl:"last_hang"`
	ExecsSinceCrash int     `json:"execs_since_crash" stbl:"execs_since_crash"`
	ExecTimeout     int     `json:"exec_timeout" stbl:"exec_timeout"`
	AFLBanner       string  `json:"afl_banner" stbl:"afl_banner"`
	AFLVersion      string  `json:"afl_version" stbl:"afl_version"`
}

func newStat() *Stat {
	s := new(Stat)
	s.GUID = xid.New()
	s.Recorder = structable.New(db, DB_FLAVOR).Bind(TB_NAME_STATS, s)
	return s
}

// TODO: Find a better way to do this.
func (newStat *Stat) CopyStat(oldStat Stat) {
	newStat.FuzzerProcessID = oldStat.FuzzerProcessID
	newStat.StartTime = oldStat.StartTime
	newStat.LastUpdate = oldStat.LastUpdate
	newStat.CyclesDone = oldStat.CyclesDone
	newStat.ExecsDone = oldStat.ExecsDone
	newStat.ExecsPerSec = oldStat.ExecsPerSec
	newStat.PathsTotal = oldStat.PathsTotal
	newStat.PathsFavored = oldStat.PathsFavored
	newStat.PathsFound = oldStat.PathsFound
	newStat.PathsImported = oldStat.PathsImported
	newStat.MaxDepth = oldStat.MaxDepth
	newStat.CurPath = oldStat.CurPath
	newStat.PendingFavs = oldStat.PendingFavs
	newStat.PendingTotal = oldStat.PendingTotal
	newStat.VariablePaths = oldStat.VariablePaths
	newStat.Stability = oldStat.Stability
	newStat.BitmapCvg = oldStat.BitmapCvg
	newStat.UniqueCrashes = oldStat.UniqueCrashes
	newStat.UniqueHangs = oldStat.UniqueHangs
	newStat.LastPath = oldStat.LastPath
	newStat.LastCrash = oldStat.LastCrash
	newStat.LastHang = oldStat.LastHang
	newStat.ExecsSinceCrash = oldStat.ExecsSinceCrash
	newStat.ExecTimeout = oldStat.ExecTimeout
	newStat.AFLBanner = oldStat.AFLBanner
	newStat.AFLVersion = oldStat.AFLVersion
}

func (s *Stat) GetJob() (*Job, error) {
	j := newJob()
	j.ID = s.JobID
	if err := j.Load(); err != nil {
		return j, err
	}
	return j, nil
}

func (s *Stat) LoadJobIDFuzzerID() error {
	return s.Recorder.LoadWhere("jid = ? and afl_banner = ?", s.JobID, s.AFLBanner)
}

func (s *Stat) LoadJobIDProcessID() error {
	return s.Recorder.LoadWhere("jid = ? and fuzzer_pid = ?", s.JobID, s.FuzzerProcessID)
}
