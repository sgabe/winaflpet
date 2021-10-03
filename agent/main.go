//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"syscall"

	flag "github.com/spf13/pflag"

	"github.com/kardianos/service"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	BuildVer string
	BuildRev string
	logger   service.Logger
)

func main() {
	var (
		action   string
		version  bool
		username string
	)

	flag.StringVarP(&action, "service", "s", "", "Control the system service.")
	flag.BoolVarP(&version, "version", "v", false, "Output the current version of the agent.")
	flag.Parse()

	if version {
		fmt.Printf("WinAFL Pet Agent v%s (rev %s)\n", BuildVer, BuildRev)
		os.Exit(0)
	}

	options := make(service.KeyValue)

	if action == "install" {
		fmt.Print("Username of service account: ")
		fmt.Scanln(&username)
		fmt.Print("Password of service account: ")
		password, _ := terminal.ReadPassword(int(syscall.Stdin))
		options["Password"] = string(password)
	}

	svcName := "WinAFLPetAgent"
	svcConfig := &service.Config{
		Name:        svcName,
		DisplayName: "WinAFL Pet Agent",
		Description: "This is a service agent exposing an API to manage WinAFL.",
		UserName:    username,
		Option:      options,
	}

	a := &Agent{}

	s, err := service.New(a, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	errs := make(chan error, 5)
	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			err := <-errs
			if err != nil {
				log.Print(err)
			}
		}
	}()

	if len(action) != 0 {
		err := service.Control(s, action)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}

		switch action {
		case service.ControlAction[3]:
			if err := initKey(); err != nil {
				log.Fatal(err)
			}
		case service.ControlAction[4]:
			if err := delKey(); err != nil {
				log.Fatal(err)
			}
		default:
			// not required
		}

		return
	}

	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
