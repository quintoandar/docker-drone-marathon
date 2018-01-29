package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var app = `
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

func TestPlugin(t *testing.T) {

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {

			want := "/v2/deployments"
			if r.URL.Path != want {
				t.Fatalf("got: %v want: %v", r.URL.Path, want)
			}

			w.WriteHeader(http.StatusOK)

			// return empty deployment list (deployment was successful)
			w.Write([]byte(`[]`))

			return
		}

		if r.Method == http.MethodPut {

			want := "/v2/apps/quintoandar/app"
			if r.URL.Path != want {
				t.Fatalf("got: %v want: %v", r.URL.Path, want)
			}

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"deploymentId": "5ed4c0c5-9ff8-4a6f-a0cd-f57f59a34b43", "version": "2015-09-29T15:59:51.164Z"}`))

			return
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	plugin := Plugin{
		Server:       srv.URL,
		Marathonfile: "",
		AppConfig:    app,
		Timeout:      time.Duration(5) * time.Minute,
	}

	if err := plugin.Exec(); err != nil {
		t.Fatalf("plugin.Exec failed: \n%v", err)
	}
}
