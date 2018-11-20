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
	"net/url"
	"os"

	"github.com/prometheus/common/log"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/prom2json"
)

var USAGE = fmt.Sprintf(`Usage: %s [METRICS_PATH | METRICS_URL [--cert CERT_PATH --key KEY_PATH | --accept-invalid-cert]]`, os.Args[0])

func main() {
	cert := flag.String("cert", "", "client certificate file")
	key := flag.String("key", "", "client certificate's key file")
	skipServerCertCheck := flag.Bool("accept-invalid-cert", false, "Accept any certificate during TLS handshake. Insecure, use only for testing.")
	flag.Parse()

	var input io.Reader
	var err error
	arg := flag.Arg(0)

	if arg == "" {
		input = os.Stdin
	} else if URL, urlErr := url.Parse(arg); urlErr != nil || URL.Scheme == "file" || URL.Scheme == "" {
		// Try to parse the arg as an url
		if input, err = os.Open(arg); err != nil {
			log.Fatal("error opening file:", err)
		}
	} else if (*cert != "" && *key == "") || (*cert == "" && *key != "") {
		log.Fatalf("%s\n with TLS client authentication: %s --cert /path/to/certificate --key /path/to/key METRICS_URL", USAGE, os.Args[0])
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
			err := prom2json.FetchMetricFamilies(arg, mfChan, *cert, *key, *skipServerCertCheck)
			if err != nil {
				log.Fatalln(err)
			}
		}()
	}

	result := []*prom2json.Family{}
	for mf := range mfChan {
		result = append(result, prom2json.NewFamily(mf))
	}
	jsonText, err := json.Marshal(result)
	if err != nil {
		log.Fatalln("error marshaling JSON:", err)
	}
	if _, err := os.Stdout.Write(jsonText); err != nil {
		log.Fatalln("error writing to stdout:", err)
	}
	fmt.Println()
}
