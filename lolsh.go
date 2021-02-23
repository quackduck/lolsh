package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	//lol "github.com/kris-nova/lolgopher"
	//"github.com/arsham/rainbow/rainbow"
	//"github.com/peterh/liner"
	"github.com/candid82/liner"
	"github.com/creack/pty"
	"golang.org/x/term"
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
	if hasOption, i := argsHaveOption("shell", "s"); hasOption {
		pluginShell(os.Args[i+1])
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
	err := os.Setenv("SHELL", os.Args[0])
	if err != nil {
		handleErr(err)
		return
	}
	content, _ := ioutil.ReadFile(configPath + "/startup.lolsh")
	if os.Getenv("lolsh_disable_lol") == "" || os.Getenv("lolsh_disable_lol") == "false" {
		parseAndRunCmdStr(string(content), true)
	} else {
		parseAndRunCmdStr(string(content), false)
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
	line.SetMultiLineMode(true)
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
				parseAndRunCmdStr(commandStr, true)
			} else {
				parseAndRunCmdStr(commandStr, false)
			}
		} else if err == liner.ErrPromptAborted {
			continue
		} else {
			handleErr(err)
		}
	}
	exitJobs()
}

func run(command []string, withLol bool) {
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
	if withLol {
		runCmdInPtyWithLol(cmd)
		return
	} else {
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		err := cmd.Start()
		if err != nil {
			handleErr(err)
			return
		}
		err = cmd.Wait()
		if err != nil {
			handleErr(err)
			return
		}
	}
	// Copy stdin to the pty and the pty to stdout.
	//go func() {
	//	_, err = io.Copy(ptmx, os.Stdin)
	//	if err != nil {
	//		handleErr(err)
	//	}
	//}()
	//_, _ = io.Copy(os.Stdout, ptmx)
}

func parseAndRunCmdStr(commandStr string, withLol bool) {
	commandStr = strings.TrimSpace(commandStr)
	commandStr = strings.ReplaceAll(commandStr, "\r\n", "\n")
	if commandStr == "" {
		return
	}
	if strings.Contains(commandStr, "\n") {
		for _, subCommand := range strings.Split(commandStr, "\n") {
			parseAndRunCmdStr(subCommand, withLol)
		}
		return
	}
	if strings.Contains(commandStr, ";") {
		for _, chunkCommand := range strings.Split(commandStr, ";") {
			parseAndRunCmdStr(chunkCommand, withLol)
		}
		return
	}
	if strings.Contains(commandStr, "#") {
		if stuffBefore := commandStr[:strings.Index(commandStr, "#")]; len(stuffBefore) > 1 {
			parseAndRunCmdStr(stuffBefore, withLol)
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
	run(command, withLol)
}

func runCmdInPtyWithLol(cmd *exec.Cmd) {
	lolcatCmd := exec.Command("lolcat")
	//	cmd.Stdin = os.Stdin
	// Start the command with a pty.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		handleErr(err)
		return
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.
	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err = pty.InheritSize(os.Stdin, ptmx); err != nil {
				handleErr(err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() {
		err = term.Restore(int(os.Stdin.Fd()), oldState)
		if err != nil {
			panic(err)
		}
	}()
	stop := make(chan bool)
	go func() { // copies stdin to the pty until a bool is sent through stop
		//_, _ = io.Copy(ptmx, os.Stdin)
		src := os.Stdin
		buf := make([]byte, 1)
		dst := ptmx

		for {
			select {
			case <-stop:
				return
			default:
				nr, er := src.Read(buf)
				if nr > 0 {
					nw, ew := dst.Write(buf[0:nr])
					if ew != nil {
						err = ew
						break
					}
					if nr != nw {
						err = io.ErrShortWrite
						break
					}
				}
				if er != nil {
					if er != io.EOF {
						err = er
					}
					break
				}
			}
		}
	}()
	lolcatCmd.Stdin = ptmx
	lolcatCmd.Stdout = os.Stdout
	err = lolcatCmd.Start()
	if err != nil {
		handleErr(err)
		return
	}
	err = lolcatCmd.Wait()
	if err != nil {
		handleErr(err)
		return
	}
	stop <- true
}

func pluginShell(shell string) {
	// Create arbitrary command.
	c := exec.Command(shell)
	lolcatCmd := exec.Command("lolcat")
	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		handleErr(err)
		return
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err = pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.
	lolcatCmd.Stdin = ptmx
	lolcatCmd.Stdout = os.Stdout
	err = lolcatCmd.Start()
	if err != nil {
		handleErr(err)
		return
	}
	// Copy stdin to the pty and the pty to stdout.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	//_, _ = io.Copy(os.Stdout, ptmx)
	err = lolcatCmd.Wait()
	if err != nil {
		handleErr(err)
		return
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
