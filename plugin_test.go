package main

import (
	"testing"
	"time"

	gock "gopkg.in/h2non/gock.v1"
)

var app = `
id: quintoandar/app
cpus: 0.1
mem: 128
container:
  type: DOCKER
  docker: 
    image: quintoandar/app
    network: BRIDGE
    portMappings:
      - containerPort: 8080
healthChecks:
  - protocol: MESOS_HTTP
    path: /health
`

var appWithURI = `
id: quintoandar/app
cpus: 0.1
mem: 128
fetch:
  - uri: "http://internal.lb.maintenance.marathon.mesos:10002/docker.tar.gz"
container:
  type: DOCKER
  docker: 
    image: quintoandar/app
    network: BRIDGE
    portMappings:
      - containerPort: 8080
healthChecks:
  - protocol: MESOS_HTTP
    path: /health
`

const server = "http://marathon.mesos:8080"

func TestAppDeploy(t *testing.T) {
	deploy(t, app)
}

func TestAppWithURIDeploy(t *testing.T) {
	deploy(t, appWithURI)
}

func deploy(t *testing.T, app string) {
	defer gock.Off()
	gock.New(server).Get("/v2/deployments").Reply(200).JSON([]map[string]string{})
	gock.New(server).Put("/v2/apps/quintoandar/app").Reply(201).
		JSON(map[string]string{
			"deploymentId": "5ed4c0c5-9ff8-4a6f-a0cd-f57f59a34b43",
			"version":      "2015-09-29T15:59:51.164Z",
		})

	plugin := Plugin{
		Server:       server,
		Marathonfile: "",
		AppConfig:    app,
		Timeout:      time.Duration(5) * time.Minute,
	}

	if err := plugin.Exec(); err != nil {
		t.Fatalf("plugin.Exec failed: \n%v", err)
	}

	if !gock.IsDone() {
		t.Fatalf("gock.IsDone() false")
	}
}

func TestAppFailedDeploy(t *testing.T) {
	defer gock.Off()

	// accept application
	gock.New(server).Put("/v2/apps/quintoandar/app").Reply(201).
		JSON(map[string]string{
			"deploymentId": "5ed4c0c5-9ff8-4a6f-a0cd-f57f59a34b43",
			"version":      "2015-09-29T15:59:51.164Z",
		})

	// accept delete
	gock.New(server).
		Delete("/v2/deployments/5ed4c0c5-9ff8-4a6f-a0cd-f57f59a34b43").
		Reply(202).
		JSON(map[string]string{
			"deploymentId": "97c136bf-5a28-4821-9d94-480d9fbb01c8",
			"version":      "2015-09-29T15:59:51.164Z",
		})

	// return an error twice (deployment and rollback will fail)
	gock.New(server).Times(1).Get("/v2/deployments").Reply(400).JSON([]map[string]string{})

	plugin := Plugin{
		Server:       server,
		Marathonfile: "",
		AppConfig:    app,
		Rollback:     true,
		Timeout:      time.Duration(5) * time.Minute,
	}

	if err := plugin.Exec(); err == nil {
		t.Fatalf("plugin.Exec did not fail: \n%v", err)
	}

	// guarantee that delete/wait on rollback were called
	if !gock.IsDone() {
		t.Fatalf("gock.IsDone() false")
	}
}
