package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bmatcuk/doublestar/v4"
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
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		wg := &sync.WaitGroup{}
		wg.Add(2)
		go printReader(wg, stdout)
		go printReader(wg, stderr)
		cmd.Start()

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}

		for _, glob := range flagIncludeSlice {
			verbosePrintf("glob pattern: %#v\n", glob)
			matches, err := doublestar.Glob(os.DirFS("."), glob)
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

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		closeFn := func() {
			term := make(chan struct{})
			defer close(term)
			go func() {
				verbosePrintf("send SIGTERM to pid(%v)\n", -cmd.Process.Pid)
				if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
					verbosePrintf("terminate failed: %v\n", err)
				}
				term <- struct{}{}
			}()

			select {
			case <-term:
			case <-time.After(5 * time.Second):
				verbosePrintf("send SIGKILL to pid(%v)\n", -cmd.Process.Pid)
				if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
					fmt.Fprintf(os.Stderr, "kill failed: %v\n", err)
				}
			}
			wg.Wait()

			stdout.Close()
			stderr.Close()
			watcher.Close()
		}

	loop:
		for {
			select {
			case <-stop:
				closeFn()
				return nil

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
				fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
			}
		}
		closeFn()
	}
}

func printReader(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()

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
