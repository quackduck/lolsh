package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	//lol "github.com/kris-nova/lolgopher"
	"github.com/arsham/rainbow/rainbow"
	"github.com/peterh/liner"
)

var (
	version    = "dev"
	helpMsg    = ``
	homeDir    = os.Getenv("HOME")
	configPath = homeDir + "/.config/lolsh"
	exit       = false
	ctrlCChan  = make(chan os.Signal)
)

func main() {
	if len(os.Args) > 1 {
		handleErrStr("too many arguments")
		fmt.Println(helpMsg)
		return
	}
	if hasOption, _ := argsHaveOption("help", "h"); hasOption {
		fmt.Println(helpMsg)
		return
	}
	if hasOption, _ := argsHaveOption("version", "v"); hasOption {
		fmt.Println("lolsh " + version)
		return
	}
	startShell()
}

func startShell() {
	signal.Notify(ctrlCChan, os.Interrupt, syscall.SIGTERM) // because of this, lolsh itself won't get any signals but will just pass them on to executed commands.
	go func() {
		<-ctrlCChan
		fmt.Println("you hit ^C lol")
	}()
	var err error
	err = os.Setenv("SHELL", "lolsh")
	if err != nil {
		handleErr(err)
		return
	}
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetTabCompletionStyle(liner.TabPrints)

	if err = os.MkdirAll(configPath, 0775); err != nil {
		handleErr(err)
		return
	}
	histFile, err := os.OpenFile(configPath+"/history.txt", os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		handleErr(err)
		return
	}
	_, err = line.ReadHistory(histFile)
	if err != nil {
		handleErr(err)
		return
	}
	defer func() {
		err = histFile.Close()
		if err != nil {
			handleErr(err)
			return
		}
	}()

	for !exit {
		name, err := os.Hostname()
		if err != nil {
			handleErr(err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			handleErr(err)
		}

		if strings.HasPrefix(cwd, homeDir) { // It does have the home dir path at the front
			cwd = "~" + strings.TrimPrefix(cwd, homeDir)
		}
		if commandStr, err := line.Prompt(os.Getenv("USER") + "@" + name + " [" + cwd + "] LOL $ "); err == nil {
			if strings.TrimSpace(commandStr) == "" {
				continue
			}
			command := strings.Fields(commandStr)
			line.AppendHistory(commandStr)
			run(command, true)
		} else if err == liner.ErrPromptAborted {
			continue
		} else {
			handleErr(err)
		}
	}
	exitJobs(line, histFile)
}

func run(command []string, withLol bool) {
	for i := range command {
		if strings.HasPrefix(command[i], "$") { // is env variable?
			command[i] = os.Getenv(strings.TrimPrefix(command[i], "$"))
		}
	}
	switch command[0] {
	case "cd":
		if len(command) > 2 {
			handleErrStr("cd: invalid number of arguments")
			return
		}
		if len(command) == 1 {
			cd(homeDir)
			return
		}
		command[1] = strings.ReplaceAll(command[1], "~", homeDir)
		cd(command[1])
		return
	case "exit":
		if len(command) > 1 {
			handleErrStr("exit: invalid number of arguments lol")
			return
		}
		exit = true
		return
	case "time":
		if len(command) == 1 {
			handleErrStr("time: invalid number of arguments lol")
			return
		}
		t := time.Now()
		defer func() {
			newt := time.Since(t)
			fmt.Println(newt, "=", float64(newt.Nanoseconds())/1e6)
		}()
		run(command[1:], withLol)
		return
	case "set":
		if len(command) > 3 || len(command) < 3 {
			handleErrStr("set: invalid number of arguments lol")
			return
		}
		if err := os.Setenv(command[1], command[2]); err != nil {
			handleErr(err)
			return
		}
		return
	case "nolol":
		if len(command) == 1 {
			handleErrStr("nolol: invalid number of arguments lol")
			return
		}
		run(command[1:], false)
		return
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	var r io.ReadCloser
	var err error
	if withLol {
		r, err = cmd.StdoutPipe()
	} else {
		cmd.Stdout = os.Stdout
	}
	if err != nil {
		handleErr(err)
		return
	}
	err = cmd.Start()
	if err != nil {
		handleErr(err)
		return
	}
	if withLol {
		l := rainbow.Light{
			Writer: os.Stdout, // to write to
			Seed:   rand.Int63n(256),
		}
		if _, err = io.Copy(&l, r); err != nil {
			handleErr(err)
			return
		}
	}
	err = cmd.Wait()
	if err != nil {
		handleErr(err)
		return
	}
}

func exitJobs(line *liner.State, histFile *os.File) {
	_, err := line.WriteHistory(histFile)
	if err != nil {
		handleErr(err)
	}
	fmt.Println("okay bye lol")
}

func argsHaveOption(long string, short string) (hasOption bool, foundAt int) {
	for i, arg := range os.Args {
		if arg == "--"+long || arg == "-"+short {
			return true, i
		}
	}
	return false, 0
}

func handleErr(err error) {
	handleErrStr(err.Error())
}

func handleErrStr(str string) {
	_, _ = fmt.Fprintln(os.Stderr, color.RedString("error: ")+str)
}

func cd(path string) {
	if err := os.Chdir(path); err != nil {
		handleErr(err)
		return
	}
	if err := os.Setenv("PWD", path); err != nil {
		handleErr(err)
	}
}
