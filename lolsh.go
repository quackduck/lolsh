package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	//"github.com/kris-nova/lolgopher"
	"github.com/peterh/liner"
)

var (
	version    = "dev"
	helpMsg    = ``
	homeDir    = os.Getenv("HOME")
	configPath = homeDir + "/.config/lolsh"
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

	for {
		name, err := os.Hostname()
		if err != nil {
			handleErr(err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			handleErr(err)
		}
		cwdWithoutHomeDirPath := strings.TrimPrefix(cwd, homeDir)
		if cwdWithoutHomeDirPath != cwd { // It does have the home dir path at the front
			cwd = "~" + cwdWithoutHomeDirPath
		}
		if commandStr, err := line.Prompt(os.Getenv("USER") + "@" + name + " [" + cwd + "] $ "); err == nil {
			command := strings.Split(commandStr, " ")
			line.AppendHistory(commandStr)
			run(command)
			_, err := line.WriteHistory(histFile)
			if err != nil {
				handleErr(err)
				return
			}
		} else if err == liner.ErrPromptAborted {
			continue
		} else {
			handleErr(err)
		}
	}
}

func run(command []string) {
	switch command[0] {
	case "cd":
		if len(command) == 1 {
			cd(homeDir)
			return
		}
		if len(command) > 2 {
			handleErrStr("too many arguments to cd")
			return
		}
		command[1] = strings.ReplaceAll(command[1], "~", homeDir)
		cd(command[1])
		return
	case "exit":
		os.Exit(0)
	case "time":
		t := time.Now()
		defer func() {
			newt := time.Since(t)
			fmt.Println(newt, "=", float64(newt.Nanoseconds())/1e6)
		}()
		run(command[1:])
		return
	}
	var err error
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	r, err := cmd.StdoutPipe()
	if err != nil {
		handleErr(err)
		return
	}
	err = cmd.Start()
	if err != nil {
		handleErr(err)
		return
	}
	rand.Seed(time.Now().UTC().UnixNano())
	seed := int(rand.Int31n(256))
	runLol(seed, os.Stdout, r)
	err = cmd.Wait()
	if err != nil {
		handleErr(err)
		return
	}
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
