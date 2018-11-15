// Copyright 2014 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	netUrl "net/url"
	"os"

	"github.com/prometheus/common/log"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/prom2json"
)

var USAGE = fmt.Sprintf("Usage: %s [[METRICS_URL | METRICS_PATH] | --url METRICS_URL [--cert CERT_PATH --key KEY_PATH | --accept-invalid-cert] | --file METRICS_PATH | --stdin]", os.Args[0])

func main() {
	cert := flag.String("cert", "", "certificate file")
	key := flag.String("key", "", "key file")
	skipServerCertCheck := flag.Bool("accept-invalid-cert", false, "Accept any certificate during TLS handshake. Insecure, use only for testing.")
	flag.String("url", "", "Read data from an URL (exclusive with positional argument, --stdin and --file)")
	flag.String("file", "", "Read data from a file (exclusive with positional argument, --url and --stdin)")
	flag.Bool("stdin", false, "Read data from stdin (exclusive with positional argument, --file and --url)")
	flag.Parse()

	arg := flag.Arg(0)
	url := arg
	file := ""
	modeCount := 0
	flag.Visit(func(entry *flag.Flag) {
		switch entry.Name {
		case "url":
			url = entry.Value.String()
			file = ""
			modeCount += 1
		case "file":
			url = ""
			file = entry.Value.String()
			modeCount += 1
		case "stdin":
			modeCount += 1
		}
	})
	// default to stdin if nothing is passed
	if modeCount == 0 || arg != "" {
		modeCount += 1
	}
	if modeCount == 0 {
		log.Fatalf("Usage: %s [[METRICS_URL | METRICS_PATH] | --url METRICS_URL [--cert CERT_PATH --key KEY_PATH] [--accept-invalid-cert] | --file METRICS_PATH | --stdin]", os.Args[0])
	} else if modeCount > 1 {
		log.Fatal("At most one of the following arguments must be set: positional argument, --url, --file or --stdin")
	}

	var input io.Reader
	var err error

	// Try to parse the url
	if url != "" {
		if obj, err := netUrl.Parse(url); obj.Scheme == "file" {
			file = obj.Path
			url = ""
		} else if err != nil || obj.Scheme == "" {
			file = url
			url = ""
		} else if (*cert != "" && *key == "") || (*cert == "" && *key != "") {
			log.Fatalf("%s\n with TLS client authentication: %s --cert /path/to/certificate --key /path/to/key METRICS_URL", USAGE, os.Args[0])
		}
	}

	if file != "" {
		if input, err = os.Open(file); err != nil {
			log.Fatal("error opening file:", err)
		}
	} else if url == "" {
		input = os.Stdin
	}

	mfChan := make(chan *dto.MetricFamily, 1024)

	if input != nil {
		go func() {
			if err := prom2json.ParseReader(input, mfChan); err != nil {
				log.Fatal("error reading metrics:", err)
			}
		}()
	} else {

		go func() {
			err := prom2json.FetchMetricFamilies(url, mfChan, *cert, *key, *skipServerCertCheck)
			if err != nil {
				log.Fatalln(err)
			}
		}()
	}

	result := []*prom2json.Family{}
	for mf := range mfChan {
		result = append(result, prom2json.NewFamily(mf))
	}
	json, err := json.Marshal(result)
	if err != nil {
		log.Fatalln("error marshaling JSON:", err)
	}
	if _, err := os.Stdout.Write(json); err != nil {
		log.Fatalln("error writing to stdout:", err)
	}
	fmt.Println()
}
