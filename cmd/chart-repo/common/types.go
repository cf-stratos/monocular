/*
Copyright (c) 2018 The Helm Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"archive/tar"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"local/monocular/cmd/chart-repo/utils"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	helmrepo "k8s.io/helm/pkg/repo"
)

type Repo struct {
	Name                string
	URL                 string
	AuthorizationHeader string `bson:"-"`
}

type Maintainer struct {
	Name  string
	Email string
}

type Chart struct {
	ID            string `bson:"_id"`
	Name          string
	Repo          Repo
	Description   string
	Home          string
	Keywords      []string
	Maintainers   []Maintainer
	Sources       []string
	Icon          string
	ChartVersions []ChartVersion
}

type ChartVersion struct {
	Version    string
	AppVersion string
	Created    time.Time
	Digest     string
	URLs       []string
}

type ChartFiles struct {
	ID     string `bson:"_id"`
	Readme string
	Values string
	Repo   Repo
	Digest string
}

type ImportChartFilesJob struct {
	Name         string
	Repo         Repo
	ChartVersion ChartVersion
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func ParseRepoUrl(repoURL string) (*url.URL, error) {
	repoURL = strings.TrimSpace(repoURL)
	return url.ParseRequestURI(repoURL)
}

func InitNetClient(additionalCA string, defaultTimeoutSeconds time.Duration) (*http.Client, error) {
	// Get the SystemCertPool, continue with an empty pool on error
	caCertPool, _ := x509.SystemCertPool()
	if caCertPool == nil {
		caCertPool = x509.NewCertPool()
	}

	// If additionalCA exists, load it
	if _, err := os.Stat(additionalCA); !os.IsNotExist(err) {
		certs, err := ioutil.ReadFile(additionalCA)
		if err != nil {
			return nil, fmt.Errorf("Failed to append %s to RootCAs: %v", additionalCA, err)
		}

		// Append our cert to the system pool
		if ok := caCertPool.AppendCertsFromPEM(certs); !ok {
			return nil, fmt.Errorf("Failed to append %s to RootCAs", additionalCA)
		}
	}

	// Return Transport for testing purposes
	return &http.Client{
		Timeout: time.Second * defaultTimeoutSeconds,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}, nil
}

func FetchRepoIndex(r Repo, netClient HttpClient) (*helmrepo.IndexFile, error) {
	indexURL, err := ParseRepoUrl(r.URL)
	if err != nil {
		log.WithFields(log.Fields{"url": r.URL}).WithError(err).Error("failed to parse URL")
		return nil, err
	}
	indexURL.Path = path.Join(indexURL.Path, "index.yaml")
	req, err := http.NewRequest("GET", indexURL.String(), nil)
	if err != nil {
		log.WithFields(log.Fields{"url": req.URL.String()}).WithError(err).Error("could not build repo index request")
		return nil, err
	}

	req.Header.Set("User-Agent", utils.UserAgent())
	if len(r.AuthorizationHeader) > 0 {
		req.Header.Set("Authorization", r.AuthorizationHeader)
	}

	res, err := netClient.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		log.WithFields(log.Fields{"url": req.URL.String()}).WithError(err).Error("error requesting repo index")
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{"url": req.URL.String(), "status": res.StatusCode}).Error("error requesting repo index, are you sure this is a chart repository?")
		return nil, errors.New("repo index request failed")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return ParseRepoIndex(body)
}

func ParseRepoIndex(body []byte) (*helmrepo.IndexFile, error) {
	var index helmrepo.IndexFile
	err := yaml.Unmarshal(body, &index)
	if err != nil {
		return nil, err
	}
	index.SortEntries()
	return &index, nil
}

func ChartsFromIndex(index *helmrepo.IndexFile, r Repo) []Chart {
	var charts []Chart
	for _, entry := range index.Entries {
		if entry[0].GetDeprecated() {
			log.WithFields(log.Fields{"name": entry[0].GetName()}).Info("skipping deprecated chart")
			continue
		}
		charts = append(charts, NewChart(entry, r))
	}
	return charts
}

// Takes an entry from the index and constructs a database representation of the
// object.
func NewChart(entry helmrepo.ChartVersions, r Repo) Chart {
	var c Chart
	copier.Copy(&c, entry[0])
	copier.Copy(&c.ChartVersions, entry)
	c.Repo = r
	c.ID = fmt.Sprintf("%s/%s", r.Name, c.Name)
	return c
}

func ExtractFilesFromTarball(filenames []string, tarf *tar.Reader) (map[string]string, error) {
	ret := make(map[string]string)
	for {
		header, err := tarf.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ret, err
		}

		for _, f := range filenames {
			if header.Name == f {
				var b bytes.Buffer
				io.Copy(&b, tarf)
				ret[f] = string(b.Bytes())
				break
			}
		}
	}
	return ret, nil
}

func ChartTarballURL(r Repo, cv ChartVersion) string {
	source := cv.URLs[0]
	if _, err := ParseRepoUrl(source); err != nil {
		// If the chart URL is not absolute, join with repo URL. It's fine if the
		// URL we build here is invalid as we can catch this error when actually
		// making the request
		u, _ := url.Parse(r.URL)
		u.Path = path.Join(u.Path, source)
		return u.String()
	}
	return source
}
