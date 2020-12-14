package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Arafatk/glot"
)

const (
	GNUPLOT_CMDS = `
set terminal png truecolor enhanced size 1000,300 butt
set output 'public/plots/%[1]v/%[2]v/high_freq.png'

set xdata time
set timefmt '%[3]v'
set format x "%[4]v"
set tics font 'small'
unset mxtics
unset mytics

set grid xtics linetype 0 linecolor rgb '#e0e0e0'
set grid ytics linetype 0 linecolor rgb '#e0e0e0'
set border linecolor rgb '#50c0f0'
set tics textcolor rgb '#000000'
set key outside

set autoscale xfixmin
set autoscale xfixmax

plot 'public/plots/%[1]v/%[2]v/plot_data' using 1:4 with filledcurve x1 title 'total paths' linecolor rgb '#000000' fillstyle transparent solid 0.2 noborder, \
	 '' using 1:3 with filledcurve x1 title 'current path' linecolor rgb '#f0f0f0' fillstyle transparent solid 0.5 noborder, \
	 '' using 1:5 with lines title 'pending paths' linecolor rgb '#0090ff' linewidth 3, \
	 '' using 1:6 with lines title 'pending favs' linecolor rgb '#c00080' linewidth 3, \
	 '' using 1:2 with lines title 'cycles done' linecolor rgb '#c000f0' linewidth 3

set terminal png truecolor enhanced size 1000,200 butt
set output 'public/plots/%[1]v/%[2]v/low_freq.png'

plot 'public/plots/%[1]v/%[2]v/plot_data' using 1:8 with filledcurve x1 title '' linecolor rgb '#c00080' fillstyle transparent solid 0.2 noborder, \
	 '' using 1:8 with lines title ' uniq crashes' linecolor rgb '#c00080' linewidth 3, \
	 '' using 1:9 with lines title 'uniq hangs' linecolor rgb '#c000f0' linewidth 3, \
	 '' using 1:10 with lines title 'levels' linecolor rgb '#0090ff' linewidth 3

set terminal png truecolor enhanced size 1000,200 butt
set output 'public/plots/%[1]v/%[2]v/exec_speed.png'

plot 'public/plots/%[1]v/%[2]v/plot_data' using 1:11 with filledcurve x1 title '' linecolor rgb '#0090ff' fillstyle transparent solid 0.2 noborder, \
	 'public/plots/%[1]v/%[2]v/plot_data' using 1:11 with lines title '    execs/sec' linecolor rgb '#0090ff' linewidth 3 smooth bezier;`
)

type Plot struct {
	UnixTime      int     `json:"unix_time"`
	CyclesDone    int     `json:"cycles_done"`
	CurPath       int     `json:"cur_path"`
	PathsTotal    int     `json:"paths_total"`
	PendingTotal  int     `json:"pending_total"`
	PendingFavs   int     `json:"pending_favs"`
	MapSize       int     `json:"map_size"`
	UniqueCrashes int     `json:"unique_crashes"`
	UniqueHangs   int     `json:"unique_hangs"`
	MaxDepth      string  `json:"max_depth"`
	ExecsPerSec   float64 `json:"execs_per_sec"`
}

func createPlots(jGUID string, fId string) error {
	dimensions := 2
	persist := false
	debug := false

	plot, _ := glot.NewPlot(dimensions, persist, debug)

	plotCmd := fmt.Sprintf(GNUPLOT_CMDS, jGUID, fId, "%%s", "%%b %%d\\n%%H:%%M")
	if err := plot.Cmd(plotCmd); err != nil {
		return err
	}

	return nil
}

func collectPlots(jGUID string, fId string) ([]string, error) {
	var plots []string

	for _, img := range [3]string{"exec_speed.png", "high_freq.png", "low_freq.png"} {
		plot := fmt.Sprintf("/plots/%s/%s/%s", jGUID, fId, img)
		if fileEmpty(filepath.Join("public", plot)) {
			return plots, errors.New("plot data is not yet available")
		}
		plots = append(plots, plot)
	}

	return plots, nil
}

func savePlotData(jGUID string, fId string, data []byte) error {
	dirPath := filepath.Join("public", "plots", jGUID, fId)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	filePath := filepath.Join(dirPath, "plot_data")
	if err := ioutil.WriteFile(filePath, data, 0600); err != nil {
		return err
	}

	return nil
}
