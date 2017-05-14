package main

import (
	"fmt"
	"github.com/coldog/tool-ecs/cmd/ecs/actions"
	"io"
	"os"
)

type Cmd interface {
	ShortDescription() string
	ParseArgs(args []string)
	PrintUsage()
	Run(w io.Writer) error
}

var commands = map[string]Cmd{
	"apply":  &actions.Apply{},
	"remove": &actions.Remove{},
	"scale":  &actions.Scale{},
	"deploy": &actions.Deploy{},
}

func printUsages() {
	fmt.Println("ECS cli")
	for name, cmd := range commands {
		fmt.Println("  -", name, cmd.ShortDescription())
	}
}

func main() {
	if len(os.Args) <= 1 {
		printUsages()
		return
	}

	arg := os.Args[1]
	cmd, ok := commands[arg]
	if !ok {
		fmt.Println("Command not recognized", arg)
		printUsages()
		os.Exit(1)
		return
	}

	cmd.ParseArgs(os.Args[2:])

	if len(os.Args) >= 3 && os.Args[2] == "help" {
		cmd.PrintUsage()
		return
	}

	err := cmd.Run(os.Stdout)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
		return
	}
}
