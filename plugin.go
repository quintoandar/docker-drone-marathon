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
	Rollback     bool
}

// Exec runs the plugin
func (p *Plugin) Exec() error {

	log.WithFields(log.Fields{
		"server":       p.Server,
		"marathonfile": p.Marathonfile,
		"timeout":      p.Timeout,
		"rollback":     p.Rollback,
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
	if app.Fetch != nil {
		var fetch []marathon.Fetch
		for _, v := range *app.Fetch {
			v.Extract = true
			fetch = append(fetch, v)
		}
		app.Fetch = &fetch
	}

	app.Container.Docker.AddParameter("log-driver", "json-file")
	app.Container.Docker.AddParameter("log-opt", "max-size=512m")

	ctx := log.WithFields(log.Fields{
		"app": app.ID,
	})

	ctx.Info("updating application")

	dep, err := client.UpdateApplication(&app, true)

	if err != nil {
		ctx.WithFields(log.Fields{
			"err": err,
		}).Error("failed to update application")
		return err
	}

	ctx.WithFields(log.Fields{
		"deployment": dep.DeploymentID,
		"timeout":    p.Timeout,
	}).Info("deploying application")

	if err := client.WaitOnDeployment(dep.DeploymentID, p.Timeout); err != nil {
		ctx.WithFields(log.Fields{
			"err":        err,
			"deployment": dep.DeploymentID,
			"timeout":    p.Timeout,
		}).Error("failed to deploy application")

		if p.Rollback {

			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
				"timeout":    p.Timeout,
			}).Info("rolling back")

			rollback, err := client.DeleteDeployment(dep.DeploymentID, false)

			if err != nil {
				ctx.WithFields(log.Fields{
					"err":        err,
					"deployment": dep.DeploymentID,
				}).Error("failed to start rollback")
				return err
			}

			if err := client.WaitOnDeployment(rollback.DeploymentID, p.Timeout); err != nil {
				ctx.WithFields(log.Fields{
					"err":      err,
					"rollback": rollback.DeploymentID,
					"timeout":  p.Timeout,
				}).Error("failed to rollback")

				ctx.WithFields(log.Fields{
					"rollback": rollback.DeploymentID,
				}).Info("force deleting rollback")

				if _, err := client.DeleteDeployment(rollback.DeploymentID, true); err != nil {
					ctx.WithFields(log.Fields{
						"err":      err,
						"rollback": rollback.DeploymentID,
					}).Error("failed to force delete rollback")
				}

				return err
			}

			ctx.WithFields(log.Fields{
				"rollback": rollback.DeploymentID,
			}).Info("deployment rollback was successful")
		} else {
			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
			}).Warning("rollback is not enabled")
		}

		return err
	}

	ctx.Info("application deployed successfully")

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
