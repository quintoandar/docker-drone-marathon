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
			"err":  err,
			"data": string(b),
		}).Errorf("failed to parse input data into JSON format")
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

	log.Infof("searching Marathon clusters")

	if err := app.UnmarshalJSON(b); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to unmarshal marathonfile: ", string(b))
		return err
	}

	// Set every uri extract to true by default
	var fetch []marathon.Fetch
	for _, v := range *app.Fetch {
		v.Extract = true
		fetch = append(fetch, v)
	}
	app.Fetch = &fetch

	app.Container.Docker.AddParameter("log-driver", "json-file")
	app.Container.Docker.AddParameter("log-opt", "max-size=512m")

	log.WithFields(log.Fields{
		"app": app.ID,
	}).Info("updating application")

	dep, err := client.UpdateApplication(&app, true)

	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
			"app": app.ID,
		}).Error("failed to update application")
		return err
	}

	log.WithFields(log.Fields{
		"app":        app.ID,
		"deployment": dep.DeploymentID,
		"timeout":    p.Timeout,
	}).Info("deploying application")

	if err := client.WaitOnDeployment(dep.DeploymentID, p.Timeout); err != nil {
		log.WithFields(log.Fields{
			"err":        err,
			"app":        app.ID,
			"deployment": dep.DeploymentID,
		}).Error("failed to deploy application")
		return err
	}

	log.WithFields(log.Fields{
		"app": app.ID,
	}).Info("application deployed successfully")

	return nil
}

// ReadInput reads Marathonfile/Appconfig data
func (p Plugin) ReadInput() (data string, err error) {
	if p.Marathonfile != "" {
		log.WithFields(log.Fields{
			"file": p.Marathonfile,
		}).Info("parsing marathonfile")

		// When 0.9 comes out, limit to secrets and other Drone variables
		b, err := ioutil.ReadFile(p.Marathonfile)

		if err != nil {
			return "", err
		}

		log.Infof("App data: \n%s", string(b))
		return envsubst.EvalEnv(string(b))
	}

	if p.AppConfig != "" {
		log.Warn("app_config is deprecated, please use a marathonfile instead")

		log.Infof("App data: \n%s", string(p.AppConfig))
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

	err = errors.New("invalid data format")
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
