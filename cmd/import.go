package main

import (
	"fmt"
	"math/rand"
	"os"
	_ "os/exec"
	"time"

	cli "gopkg.in/urfave/cli.v1"

	_ "github.com/go-sql-driver/mysql"

	importer "github.com/dutchcoders/slackarchive-import"
	config "github.com/dutchcoders/slackarchive-import/config"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

var version string = "0.1"

func main() {
	app := cli.NewApp()
	app.Name = "SlackArchive"
	app.Version = version
	app.Flags = append(app.Flags, []cli.Flag{
		cli.StringFlag{
			Name:   "config, c",
			Value:  "config.yaml",
			Usage:  "Custom configuration file path",
			EnvVar: "",
		},
	}...)

	app.Before = config.Load
	app.Action = run

	app.Run(os.Args)
}

func run(c *cli.Context) {
	if len(c.Args()) != 2 {
		fmt.Println("Usage: import {token} {path}")
		return
	}

	token := c.Args().Get(0)
	p := c.Args().Get(1)

	conf := config.Get()

	app := importer.New(conf)
	app.Import(token, p)
}
