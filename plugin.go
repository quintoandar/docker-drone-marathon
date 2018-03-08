package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strings"
	"time"

	"github.com/drone/envsubst"
	"github.com/ghodss/yaml"

	marathon "github.com/fbcbarbosa/go-marathon"

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
			"err": err,
		}).Errorf("failed to parse input data into JSON format")
		return err
	}

	log.Info("searching Marathon clusters")

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

	if err := app.UnmarshalJSON(b); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to unmarshal marathonfile: ", string(b))
		return err
	}

	ctx := log.WithField("app", app.ID)
	ctx.Info("applying configuration defaults")

	// Set every uri extract to true by default
	if app.Fetch != nil {
		var fetch []marathon.Fetch
		for _, v := range *app.Fetch {
			v.Extract = true
			fetch = append(fetch, v)
		}
		app.Fetch = &fetch
	}

	// Set faster default healthcheck timing configuration to avoid long rollbacks
	if app.HealthChecks != nil {
		for _, h := range *app.HealthChecks {
			if h.GracePeriodSeconds == 0 {
				h.GracePeriodSeconds = 60
			}
			if h.IntervalSeconds == 0 {
				h.IntervalSeconds = 15
			}
			if h.TimeoutSeconds == 0 {
				h.TimeoutSeconds = 10
			}
		}
	}

	app.Container.Docker.AddParameter("log-driver", "json-file")
	app.Container.Docker.AddParameter("log-opt", "max-size=512m")

	var prevVersion *marathon.ApplicationVersion

	// load application in case we need to roll back
	if p.Rollback {
		stableApp, err := client.Application(app.ID)

		if err != nil {
			ctx.WithError(err).Warning("could not get application information" +
				" from marathon (only required in case of rollback)")
		} else {
			prevVersion = &marathon.ApplicationVersion{
				Version: stableApp.Version,
			}
		}
	}

	ctx.Info("updating application")

	dep, err := client.UpdateApplication(&app, true)

	if err != nil {
		ctx.WithError(err).Error("failed to start application update")
		return err
	}

	ctx.WithFields(log.Fields{
		"deployment": dep.DeploymentID,
		"timeout":    p.Timeout,
		"version":    dep.Version,
	}).Info("deploying application")

	if err := client.WaitOnDeployment(dep.DeploymentID, p.Timeout); err != nil {

		ctx.WithFields(log.Fields{
			"err":        err,
			"deployment": dep.DeploymentID,
			"timeout":    p.Timeout,
			"version":    dep.Version,
		}).Error("failed to deploy application")

		if p.Rollback {

			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
				"version":    dep.Version,
			}).Info("cancelling deployment")

			if _, err := client.DeleteDeployment(dep.DeploymentID, true); err != nil {
				ctx.WithError(err).Error("failed to cancel deployment")
				return err
			}

			if prevVersion == nil {
				ctx.Error("no previous version available to roll back to")
			}

			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
				"version":    dep.Version,
			}).Info("waiting for all failed tasks to die")

			if err := waitOnTasksToDie(client, app.ID, dep.Version, p.Timeout); err != nil {
				ctx.WithError(err).Error("failed to rollback")
				return err
			}

			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
				"timeout":    p.Timeout,
				"version":    prevVersion.Version,
			}).Info("rolling back to previous application version")

			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
				"timeout":    p.Timeout,
				"version":    prevVersion.Version,
			}).Info("a new rolling deployment will start")

			rollback, err := client.SetApplicationVersion(app.ID, prevVersion)

			if err != nil {
				ctx.WithError(err).Error("failed to rollback")
				return err
			}

			if err := client.WaitOnDeployment(rollback.DeploymentID, p.Timeout); err != nil {

				ctx.WithFields(log.Fields{
					"err":      err,
					"rollback": rollback.DeploymentID,
					"timeout":  p.Timeout,
					"version":  prevVersion.Version,
				}).Error("failed to deploy rollback")

				ctx.WithFields(log.Fields{
					"rollback": rollback.DeploymentID,
					"timeout":  p.Timeout,
					"version":  prevVersion.Version,
				}).Info("cancelling rollback")

				if _, err := client.DeleteDeployment(rollback.DeploymentID, true); err != nil {
					ctx.WithFields(log.Fields{
						"err":      err,
						"rollback": rollback.DeploymentID,
						"version":  prevVersion.Version,
					}).Error("failed to cancel rollback")
					return err
				}

				// override Marathon timeout error with a more descriptive error
				if strings.Contains(err.Error(), "timed out") {
					err = errors.New(
						"your rollback has failed and the application is at an" +
							" unknown state, please check your application logs",
					)
				}

				return err
			}

			ctx.WithFields(log.Fields{
				"deployment": dep.DeploymentID,
				"rollback":   rollback.DeploymentID,
				"version":    prevVersion.Version,
			}).Info("rollback was successful")

		} else {
			ctx.WithField("deployment", dep.DeploymentID).Warning("rollback is not enabled")
		}

		// override Marathon timeout error with a more descriptive error
		if strings.Contains(err.Error(), "timed out") {
			err = errors.New(
				"could not deploy your application within the maximum timeout," +
					" please check your application logs",
			)
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

func waitOnTasksToDie(client marathon.Marathon, name, version string, timeout time.Duration) error {
	if val, err := areTasksDead(client, name, version); err != nil || val {
		return err
	}

	tick := time.Tick(time.Second * 5)
	tout := time.After(timeout)

	for {
		select {

		case <-tick:

			if val, err := areTasksDead(client, name, version); err != nil || val {
				return err
			}

		case <-tout:
			return errors.New("timed out")
		}
	}
}

func areTasksDead(client marathon.Marathon, name, version string) (bool, error) {

	tasks, err := client.Tasks(name)

	if err != nil {
		return false, err
	}

	return !containsVersion(tasks.Tasks, version), nil
}

func containsVersion(tasks []marathon.Task, version string) bool {
	for _, t := range tasks {
		if t.Version == version {
			log.WithFields(log.Fields{
				"task":    t.ID,
				"version": version,
			}).Info("waiting for failed task to die")
			return true
		}
	}
	return false
}
