package main

import (
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "Marathon deploy Drone plugin"
	app.Usage = "marathon deploy Drone plugin"
	app.Action = run
	app.Flags = []cli.Flag{

		cli.StringFlag{
			Name:   "server",
			Usage:  "dcos server",
			Value:  "http://master.mesos:8080",
			EnvVar: "PLUGIN_SERVER",
		},
		cli.StringFlag{
			Name:   "marathonfile",
			Usage:  "application marathon file",
			EnvVar: "PLUGIN_MARATHONFILE",
		},
		cli.StringFlag{
			Name:   "app_config",
			Usage:  "application in-line config",
			EnvVar: "PLUGIN_APP_CONFIG",
		},
		cli.StringFlag{
			Name:   "timeout",
			Usage:  "deployment timeout in minutes (applies to rollbacks too)",
			Value:  "5",
			EnvVar: "PLUGIN_TIMEOUT",
		},
		cli.BoolTFlag{
			Name:   "rollback",
			Usage:  "if true will attempt to rollback failed deployments",
			EnvVar: "PLUGIN_ROLLBACK",
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	timeout, err := strconv.Atoi(c.String("timeout"))

	if err != nil {
		log.WithFields(log.Fields{
			"timeout": c.String("timeout"),
			"error":   err,
		}).Error("invalid timeout configuration")
		return err
	}

	plugin := Plugin{
		Server:       c.String("server"),
		Marathonfile: c.String("marathonfile"),
		AppConfig:    c.String("app_config"),
		Timeout:      time.Duration(timeout) * time.Minute,
		Rollback:     c.BoolT("rollback"),
	}

	return plugin.Exec()
}
