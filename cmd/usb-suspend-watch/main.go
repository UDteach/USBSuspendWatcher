package main

import (
	"flag"
	"fmt"
	"os"

	"usb-suspend-watch/internal/etwhelper"
	"usb-suspend-watch/internal/ui"
)

var version = "dev"

func main() {
	var helper bool
	var logDir string
	var session string
	var stopFile string
	var parentPID int
	var etlPath string

	flag.BoolVar(&helper, "etw-helper", false, "run as elevated ETW helper")
	flag.StringVar(&logDir, "log-dir", "", "log directory")
	flag.StringVar(&session, "session", "", "ETW session name")
	flag.StringVar(&stopFile, "stop-file", "", "path watched by ETW helper for shutdown")
	flag.IntVar(&parentPID, "parent-pid", 0, "parent GUI process id watched by the ETW helper")
	flag.StringVar(&etlPath, "etl-path", "", "ETW .etl output path for logman capture")
	flag.Parse()

	if helper {
		os.Exit(etwhelper.Run(etwhelper.Config{
			LogDir:    logDir,
			Session:   session,
			StopFile:  stopFile,
			ParentPID: parentPID,
			ETLPath:   etlPath,
		}))
	}

	if err := ui.Run(ui.Config{Version: version}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
