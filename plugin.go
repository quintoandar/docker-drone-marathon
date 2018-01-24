package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"

	"github.com/drone/envsubst"
	"github.com/ghodss/yaml"

	marathon "github.com/gambol99/go-marathon"

	log "github.com/Sirupsen/logrus"
)

// Plugin defines the parameters
type Plugin struct {
	Server       string
	Marathonfile string
	AppConfig    string
	Timeout      time.Duration
}

// Exec runs the plugin
func (p *Plugin) Exec() error {

	log.WithFields(log.Fields{
		"server":       p.Server,
		"marathonfile": p.Marathonfile,
		"timeout":      p.Timeout,
	}).Info("attempting to start job")

	data, err := p.ReadInput()

	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to read marathonfile/app_config input data")
		return err
	}

	b, err := parseData(data)

	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to parse input data into JSON format: ", string(b))
		return err
	}

	config := marathon.NewDefaultConfig()
	config.URL = p.Server

	client, err := marathon.NewClient(config)

	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to create a client for marathon")
		return err
	}

	var app marathon.Application

	log.Infof("searching cluster for application")

	if err := app.UnmarshalJSON(b); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to unmarshal marathonfile: ", string(b))
		return err
	}

	app.Container.Docker.AddParameter("log-driver", "json-file")
	app.Container.Docker.AddParameter("log-opt", "max-size=512m")

	if _, err = client.Application(app.ID); err != nil {
		log.Infof("failed to get application %s (%s)", app.ID, err)
		log.Infof("creating application %s", app.ID)

		if _, err = client.CreateApplication(&app); err != nil {
			log.WithFields(log.Fields{
				"err": err,
				"app": app.ID,
			})
			log.Errorf("failed to create application")
			return err
		}
	} else {
		log.Infof("updating application %s", app.ID)
		dep, err := client.UpdateApplication(&app, true)

		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
				"app": app.ID,
			})
			log.Errorf("failed to update application")
			return err
		}

		if err := client.WaitOnDeployment(dep.DeploymentID, p.Timeout); err != nil {
			log.WithFields(log.Fields{
				"err":        err,
				"app":        app.ID,
				"deployment": dep.DeploymentID,
			})
			log.Errorf("failed to deploy application")
			return err
		}
	}

	return nil
}

// ReadInput reads Marathonfile/Appconfig data
func (p Plugin) ReadInput() (data string, err error) {
	if p.Marathonfile != "" {
		log.Info("parsing marathonfile ", p.Marathonfile)

		// When 0.9 comes out, limit to secrets and other Drone variables
		b, err := ioutil.ReadFile(p.Marathonfile)

		if err != nil {
			return "", err
		}

		return envsubst.EvalEnv(string(b))
	}

	if p.AppConfig != "" {
		log.Warn("app_config is deprecated and will be removed, please use a marathonfile instead")

		return envsubst.EvalEnv(p.AppConfig)
	}

	err = errors.New("missing parameters")
	return
}

func parseData(data string) (b []byte, err error) {
	if isYAML(data) {
		log.Info("data is in YAML format, parsing into JSON")
		return yaml.YAMLToJSON([]byte(data))
	}

	if isJSON(data) {
		log.Info("data is in JSON format, no need to parse")
		return
	}

	err = errors.New("invalid data")
	return
}

func isJSON(s string) bool {
	var j map[string]interface{}
	return json.Unmarshal([]byte(s), &j) == nil
}

func isYAML(s string) bool {
	var y map[string]interface{}
	return yaml.Unmarshal([]byte(s), &y) == nil
}
