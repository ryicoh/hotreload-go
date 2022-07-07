package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func main() {
	if err := parseFlag(); err != nil {
		fmt.Fprintf(os.Stderr, "parse flag: %v\n", err)
		os.Exit(1)
	}

	if err := start(); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}

var (
	flagCommand      string
	flagIncludeSlice []string
	flagVerbose      bool

	verbosePrintln func(arg ...interface{})
	verbosePrintf  func(format string, arg ...interface{})
)

func parseFlag() error {
	flag.StringVar(&flagCommand, "cmd", "", "Run command")
	include := ""
	flag.StringVar(&include, "include", "", "File include pattern (optional)")
	flag.BoolVar(&flagVerbose, "verbose", false, "Verbose logging (default:false)")
	flag.Parse()

	if flagCommand == "" {
		return errors.New("`-cmd` is a required flag")
	}
	flagIncludeSlice = strings.Split(include, ",")

	verbosePrintln = func(arg ...interface{}) {
		if flagVerbose {
			fmt.Println(arg...)
		}
	}
	verbosePrintf = func(format string, arg ...interface{}) {
		if flagVerbose {
			fmt.Printf(format, arg...)
		}
	}

	verbosePrintf(`# flags
cmd    : %#v
include: %#v
verbose: %#v

`, flagCommand, flagIncludeSlice, flagVerbose)

	return nil
}

func start() error {
	for {
		cmd := exec.Command("sh", "-c", flagCommand)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}
		go printReader(stdout)
		go printReader(stderr)
		cmd.Start()

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}

		for _, glob := range flagIncludeSlice {
			verbosePrintf("glob pattern: %#v\n", glob)
			matches, err := filepath.Glob(glob)
			if err != nil {
				return err
			}
			for _, path := range matches {
				verbosePrintf("- %#v\n", path)
				if err = watcher.Add(path); err != nil {
					return err
				}
			}
		}

	loop:
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return nil
				}
				if event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					verbosePrintf("event: %s\n", event.String())
					break loop
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
				fmt.Fprintf(os.Stderr, "watcher: %v\n", err)
			}
		}
		stdout.Close()
		stderr.Close()
		cmd.Process.Kill()
		watcher.Close()
	}
}

func printReader(r io.Reader) {
	reader := bufio.NewReader(r)
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if flagVerbose {
			fmt.Print("| ", str)
		}
		fmt.Print(str)
	}
}
