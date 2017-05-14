package main

type Cmd interface {
	ShortDescription() string
	ParseArgs(args []string)
	Run() error
}
