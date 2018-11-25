package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

type (
	// Plugin - Using drone style plugin
	Plugin struct {
		CatalogName     string
		ChartName       string
		ChartTags       []string
		DryRun          bool
		RancherAPIToken string
		RancherURL      string
	}
	// Projects -
	Projects struct {
		Data []Project `json:"data"`
	}
	// Project -
	Project struct {
		Name  string            `json:"name"`
		ID    string            `json:"id"`
		Links map[string]string `json:"links"`
	}
	// Apps -
	Apps struct {
		Data []App `json:"data"`
	}
	// App -
	App struct {
		Actions    map[string]string `json:"actions,omitempty"`
		Answers    map[string]string `json:"answers,omitempty"`
		ExternalID string            `json:"externalId,omitempty"`
		ID         string            `json:"id,omitempty"`
		Links      map[string]string `json:"links,omitempty"`
		Name       string            `json:"name,omitempty"`
	}
	// Catalog -
	Catalog struct {
		State                string `json:"state"`
		Transitioning        string `json:"transitioning"`
		TransitioningMessage string `json:"transitioningMessage"`
	}
)

// Exec -
func (p Plugin) Exec() error {
	val, ok := os.LookupEnv("LOG_LEVEL")
	if ok {
		level, _ := log.ParseLevel(val)
		log.SetLevel(level)
	}

	tag, err := p.parseTags()
	if err != nil {
		return err
	}

	err = p.refreshCatalog()
	if err != nil {
		return err
	}

	projects, err := p.getProjects()
	if err != nil {
		return err
	}

	for _, project := range projects.Data {
		apps, err := p.getApps(project.Links["apps"])
		if err != nil {
			return err
		}

		for _, app := range apps.Data {
			log.Debug("App: ", app.ID)
			// check for autoUpdate
			autoUpdate, ok := app.Answers["rancher.autoUpdate"]
			if ok {
				if autoUpdate == "true" {
					extID, err := url.Parse(app.ExternalID)
					if err != nil {
						log.Debugf("Failed to parse externalID: %s - %v", app.ExternalID, err)
						continue
					}

					q := extID.Query()
					catalog := q["catalog"][0]
					template := q["template"][0]
					version := q["version"][0]
					if catalog == p.CatalogName && template == p.ChartName {
						log.Infof("Found Catalog App to Update: %s/%s", project.Name, app.Name)
						log.Infof("Upgrade Version: %s -> %s", version, tag)
						// post
						if !p.DryRun {
							newExtID := fmt.Sprintf("catalog://?catalog=%s&template=%s&version=%s", catalog, template, tag)
							body := &App{
								ExternalID: newExtID,
								Answers:    app.Answers,
							}
							jsonBody := new(bytes.Buffer)
							enc := json.NewEncoder(jsonBody)
							enc.SetEscapeHTML(false)
							err = enc.Encode(body)
							// jsonBody, err := json.Marshal(body)
							log.Debug(jsonBody.String())
							if err != nil {
								log.Errorf("Failed to Marshal json for upgrade body: %v - %v", body, err)
								continue
							}
							_, err := p.httpDo("POST", app.Actions["upgrade"], jsonBody)
							if err != nil {
								log.Errorf("Failed to Post Upgrade: %v - %v", app.Actions["upgrade"], err)
								continue
							}
							log.Info("Upgrade Successful")
						} else {
							log.Info("Dry-Run: Skipping Upgrade")
						}
					}
				}
			}
		}
	}

	return nil
}

func (p Plugin) getApps(url string) (*Apps, error) {
	log.Debug("Get Apps")
	apps := &Apps{}

	resp, err := p.httpDo("GET", url, nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp, apps)
	if err != nil {
		return nil, err
	}
	log.Debug(printPretty(apps))

	return apps, nil
}

func (p Plugin) getProjects() (*Projects, error) {
	// TODO: paging for more than 1000 projects
	log.Debug("Get Projects")
	projects := &Projects{}

	url := fmt.Sprintf("%s/v3/projects", p.RancherURL)
	resp, err := p.httpDo("GET", url, nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp, projects)
	if err != nil {
		return nil, err
	}
	log.Debug(printPretty(projects))

	return projects, nil
}

func (p Plugin) parseTags() (string, error) {
	// return first semver tag
	tagRegex := regexp.MustCompile("^[v]*\\d+\\.\\d+\\.\\d+")
	for _, tag := range p.ChartTags {
		if tagRegex.MatchString(tag) {
			return tag, nil
		}
	}

	return "", fmt.Errorf("No semver tags found")
}

func (p Plugin) refreshCatalog() error {
	//refresh catalog
	log.Info("Refreshing Catalog: ", p.CatalogName)
	refreshURL := fmt.Sprintf("%s/v3/catalogs/%s?action=refresh", p.RancherURL, p.CatalogName)
	_, err := p.httpDo("POST", refreshURL, nil)
	if err != nil {
		return err
	}

	//poll for update
	log.Info("Waiting for Catalog to sync.")
	catalogURL := fmt.Sprintf("%s/v3/catalogs/%s", p.RancherURL, p.CatalogName)
	catalog := &Catalog{}
	for {
		resp, err := p.httpDo("GET", catalogURL, nil)
		if err != nil {
			return err
		}
		err = json.Unmarshal(resp, catalog)
		if err != nil {
			return err
		}
		if catalog.Transitioning == "yes" {
			log.Info("Catalog Sync: ", catalog.TransitioningMessage)
		} else {
			log.Info("Catalog Sync Complete.")
			break
		}
		time.Sleep(10 * time.Second)
	}

	return nil
}

func (p Plugin) httpDo(method string, url string, body *bytes.Buffer) ([]byte, error) {
	if body == nil {
		body = bytes.NewBuffer(nil)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(method, url, body)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprint("Bearer ", p.RancherAPIToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	goodStatus := regexp.MustCompile("^2\\d\\d")
	if !goodStatus.MatchString(resp.Status) {
		return nil, fmt.Errorf("%s %s returned %s", req.Method, req.URL, resp.Status)
	}

	defer resp.Body.Close()
	jsonBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return jsonBody, nil
}

func printPretty(data interface{}) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ""
	}

	return string(jsonData)
}
