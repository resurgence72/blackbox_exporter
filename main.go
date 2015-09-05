package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
)

var addr = flag.String("web.listen-address", ":8080", "The address to listen on for HTTP requests.")
var configFile = flag.String("config.file", "blackbox.yml", "Blackbox exporter configuration file.")

type Config struct {
	Modules map[string]Module `yaml:"modules"`
}

type Module struct {
	Prober  string        `yaml:"prober"`
	Timeout time.Duration `yaml:"timeout"`
	HTTP    HTTPProbe     `yaml:"http"`
	TCP     TCPProbe      `yaml:"tcp"`
}

type HTTPProbe struct {
	// Defaults to 2xx.
	ValidStatusCodes  []int `yaml:"valid_status_codes"`
	NoFollowRedirects bool  `yaml:"no_follow_redirects"`
	FailIfSSL         bool  `yaml:"fail_if_ssl"`
	FailIfNotSSL      bool  `yaml:"fail_if_not_ssl"`
}

type TCPProbe struct {
}

var Probers = map[string]func(string, http.ResponseWriter, Module) bool{
	"http": probeHTTP,
	"tcp":  probeTCP,
}

func probeHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	params := r.URL.Query()
	target := params.Get("target")
	moduleName := params.Get("module")
	if target == "" {
		http.Error(w, "Target parameter is missing", 400)
		return
	}
	if moduleName == "" {
		moduleName = "http2xx"
	}
	module, ok := config.Modules[moduleName]
	if !ok {
		http.Error(w, fmt.Sprintf("Unkown module %s", moduleName), 400)
		return
	}
	start := time.Now()
	success := Probers[module.Prober](target, w, module)
	fmt.Fprintf(w, "probe_duration_seconds %f\n", float64(time.Now().Sub(start))/1e9)
	if success {
		fmt.Fprintf(w, "probe_success %d\n", 1)
	} else {
		fmt.Fprintf(w, "probe_success %d\n", 0)
	}
}

func main() {
	flag.Parse()

	yamlFile, err := ioutil.ReadFile(*configFile)

	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	config := Config{}

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Fatalf("Error parsing config file: %s", err)
	}

	http.Handle("/metrics", prometheus.Handler())
	http.HandleFunc("/probe",
		func(w http.ResponseWriter, r *http.Request) {
			probeHandler(w, r, &config)
		})
	http.ListenAndServe(*addr, nil)
}
