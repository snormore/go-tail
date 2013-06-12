// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"github.com/fw42/go-tail"
	"os"
)

func args2config() tail.Config {
	config := tail.Config{Follow: true}
	flag.IntVar(&config.Location, "n", 0, "tail from the last (N-1)th byte offset location (use negative value to tail from start of file)")
	flag.BoolVar(&config.Follow, "f", false, "wait for additional data to be appended to the file")
	flag.BoolVar(&config.ReOpen, "F", false, "follow, and track file rename/rotation")
	flag.BoolVar(&config.Poll, "p", false, "use polling, instead of inotify")
	flag.Parse()
	if config.ReOpen {
		config.Follow = true
	}
	return config
}

func main() {
	config := args2config()
	if flag.NFlag() < 1 {
		fmt.Println("need one or more files as arguments")
		os.Exit(1)
	}

	done := make(chan bool)
	for _, filename := range flag.Args() {
		go tailFile(filename, config, done)
	}

	for _, _ = range flag.Args() {
		<-done
	}
}

func tailFile(filename string, config tail.Config, done chan bool) {
	defer func() { done <- true }()
	t, err := tail.TailFile(filename, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	for line := range t.Lines {
		fmt.Println(line.Text)
	}
	err = t.Wait()
	if err != nil {
		fmt.Println(err)
	}
}
