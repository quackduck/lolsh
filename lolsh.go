package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	//lol "github.com/kris-nova/lolgopher"
	//"github.com/arsham/rainbow/rainbow"
	//"github.com/peterh/liner"
	"github.com/candid82/liner"
)

var (
	version = "dev"
	helpMsg = `lolsh - A shell with all output lolcat-ed
Usage: lolsh [-v/--version | -v/--help]`
	homeDir    = os.Getenv("HOME")
	configPath = homeDir + "/.config/lolsh"
	exit       = false
	ctrlCChan  = make(chan os.Signal)
	line       *liner.State
	histFile   *os.File
)

func main() {
	if hasOption, _ := argsHaveOption("help", "h"); hasOption {
		fmt.Println(helpMsg)
		return
	}
	if hasOption, _ := argsHaveOption("version", "v"); hasOption {
		fmt.Println("lolsh " + version + " lol")
		return
	}

	if len(os.Args) > 1 {
		handleErrStr("too many arguments lol")
		fmt.Println(helpMsg)
		return
	}
	startShell()
}

func startShell() {
	signal.Notify(ctrlCChan, os.Interrupt) // because of this, lolsh itself won't get any signals but will just pass them on to executed commands.
	go func() {
		<-ctrlCChan
		fmt.Println("you hit ^C lol")
	}()
	err := os.Setenv("SHELL", "lolsh")
	if err != nil {
		handleErr(err)
		return
	}
	content, _ := ioutil.ReadFile(configPath + "/startup.lolsh")
	if os.Getenv("lolsh_disable_lol") == "" || os.Getenv("lolsh_disable_lol") == "false" {
		run(string(content), true)
	} else {
		run(string(content), false)
	}
	line = liner.NewLiner()
	defer line.Close()
	//line.SetTabCompletionStyle(liner.TabPrints)
	if err = os.MkdirAll(configPath, 0775); err != nil {
		handleErr(err)
		return
	}
	histFile, err = os.OpenFile(configPath+"/history.txt", os.O_RDWR|os.O_CREATE, 0664)
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
		if commandStr, err := line.Prompt(os.Getenv("USER") + "@" + name + " [" + cwd + "] lol $ "); err == nil {
			if strings.TrimSpace(commandStr) == "" {
				continue
			}
			line.AppendHistory(commandStr)
			if os.Getenv("lolsh_disable_lol") == "" || os.Getenv("lolsh_disable_lol") == "false" {
				run(commandStr, true)
			} else {
				run(commandStr, false)
			}
		} else if err == liner.ErrPromptAborted {
			continue
		} else {
			handleErr(err)
		}
	}
	exitJobs()
}

func run(commandStr string, withLol bool) {
	commandStr = strings.TrimSpace(commandStr)
	if commandStr == "" {
		return
	}
	for _, subCommand := range strings.Split(commandStr, "\n") {
		run(subCommand, withLol)
	}
	if strings.Contains(commandStr, ";") {
		for _, chunkCommand := range strings.Split(commandStr, ";") {
			run(chunkCommand, withLol)
		}
		return
	}
	if strings.Contains(commandStr, "#") {
		if stuffBefore := commandStr[:strings.Index(commandStr, "#")]; len(stuffBefore) > 1 {
			run(stuffBefore, withLol)
		}
		return
	}
	command := strings.Fields(commandStr)
	for i := range command {
		if strings.HasPrefix(command[i], "$") { // is env variable?
			command[i] = os.Getenv(strings.TrimPrefix(command[i], "$"))
		}
		command[i] = strings.Replace(command[i], "~", homeDir, 1) // 1 replacement per word
	}

	// Builtins
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
			fmt.Println("Command took", newt.Round(time.Millisecond/100), "lol")
		}()
		run(strings.TrimPrefix(commandStr, "time"), withLol)
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
		run(strings.TrimPrefix(commandStr, "time"), false)
		return
	case "history":
		if len(command) < 1 {
			handleErrStr("history: invalid number of arguments lol")
			return
		}
		buf := new(bytes.Buffer)
		_, err := line.WriteHistory(buf)
		if err != nil {
			handleErr(err)
			return
		}
		history := strings.Split(strings.TrimSpace(buf.String()), "\n")
		for i, entry := range history {
			fmt.Println(strconv.Itoa(i+1)+":", entry)
		}
		//run([]string{"cat", configPath + "/history.txt"}, withLol)
		return
	}
	cmd := exec.Command(command[0], command[1:]...)
	lolcatCmd := exec.Command("lolcat")
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	var err error
	if withLol {
		lolcatCmd.Stdin, err = cmd.StdoutPipe()
		lolcatCmd.Stdout = os.Stdout
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
		err = lolcatCmd.Start()
		if err != nil {
			handleErr(err)
			return
		}
		// comments are the ones which do not work with vim/nano and other editors

		//rand.Seed(time.Now().UTC().UnixNano())
		//seed := int(rand.Int31n(256))
		//runLol(seed, os.Stdout, r)

		//l := rainbow.Light{
		//	Writer: os.Stdout, // to write to
		//	Seed:   rand.Int63n(256),
		//}
		//if _, err = io.Copy(&l, r); err != nil {
		//	handleErr(err)
		//	return
		//}

		//if _, err = io.Copy(lol.NewLolWriter(), r); err != nil {
		//	handleErr(err)
		//	return
		//}

	}
	err = cmd.Wait()
	if err != nil {
		handleErr(err)
		return
	}
	if withLol {
		err = lolcatCmd.Wait()
		if err != nil {
			handleErr(err)
			return
		}
	}
}

func exitJobs() {
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
