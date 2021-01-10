package main

import (
	"bufio"
	"strings"
	"time"
	//"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	//"github.com/peterh/liner"
)

var (
	version = "dev"
	helpMsg = ``
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
	err := os.Setenv("SHELL", "lolsh")
	if err != nil {
		handleErr(err)
		return
	}
	//reader := bufio.NewReader(os.Stdin)
	//for {
	//	input, _, err := reader.ReadRune()
	//	if err != nil && err == io.EOF {
	//		break
	//	}
	//	output = append(output, input)
	//}
	reader := bufio.NewReader(os.Stdin)
	for {
		name, err := os.Hostname()
		if err != nil {
			handleErr(err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			handleErr(err)
		}
		fmt.Print(os.Getenv("USER") + "@" + name + " [" + cwd + "] $ ")

		input, err := reader.ReadString('\n')
		if err != nil {
			handleErr(err)
		}
		input = strings.TrimSuffix(input, "\n")
		command := strings.Split(input, " ")
		run(command)
	}

}

func run(command []string) {
	// run it with the users shell
	switch command[0] {
	case "cd":
		if len(command) == 1 {
			cd(os.Getenv("HOME"))
			return
		}
		cd(command[1])
	case "exit":
		os.Exit(0)
	case "time":
		t := time.Now()
		defer func() {
			fmt.Println(time.Since(t))
		}()
		run(command[1:])
		return
	}
	var err error
	cmd := exec.Command(command[0], command[1:]...)
	lol := exec.Command("lolcat")
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	lol.Stdin, err = cmd.StdoutPipe()
	if err != nil {
		handleErr(err)
		return
	}
	lol.Stdout = os.Stdout
	err = lol.Start()
	if err != nil {
		handleErr(err)
		return
	}
	err = cmd.Run()
	if err != nil {
		handleErr(err)
		return
	}
	err = lol.Wait()
	if err != nil {
		handleErr(err)
		return
	}
	//cmd.Stderr = nil
	//cmd.Stdout = nil
	//cmd.Stdin = nil
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

//cmd := exec.Command(os.Getenv("SHELL"), "-c", command+" | lolcat") //nolint //"Subprocess launched with function call as argument or cmd arguments"
//l := Light{
//	Writer: os.Stdout, // to write to
//	Seed:   rand.Int63n(256),
//}
//io.Pipe()
//io.Copy(&l, )
//stdout, err := cmd.StdoutPipe()
//if err != nil {
//	handleErr(err)
//}
//stderr, err := cmd.StderrPipe()
//if err != nil {
//	handleErr(err)
//}
//err = cmd.Start()
//if err != nil {
//	handleErr(err)
//}
//
//stdo, g := ioutil.ReadAll(stdout)
//stde, f := ioutil.ReadAll(stderr)
//
//d := cmd.Wait()

//fmt.Println(out.String())
//err := l.Paint()
//if err != nil {
//	handleErr(err)
//}
