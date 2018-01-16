package main

import (
	"os"

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
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {

	plugin := Plugin{
		Server:       c.String("server"),
		Marathonfile: c.String("marathonfile"),
		AppConfig:    c.String("app_config"),
	}

	return plugin.Exec()
}
