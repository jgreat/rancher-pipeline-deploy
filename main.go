package main

// Scan through all the projects that this user has access to.
// Find apps that match my app and "rancher.autoUpdate: true".
// Update those apps.

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

var version = "0.0.1"

func main() {
	val, ok := os.LookupEnv("LOG_LEVEL")
	if ok {
		level, _ := log.ParseLevel(val)
		log.SetLevel(level)
	}

	app := cli.NewApp()
	app.Name = "rancher-pipeline-deploy"
	app.Usage = "rancher-pipeline-deploy"
	app.Action = run
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "catalog-name",
			Usage:  "Name of Rancher Catalog. Will use git repo if not defined.",
			EnvVar: "RANCHER_CATALOG_NAME,CICD_GIT_REPO",
		},
		cli.StringFlag{
			Name:   "chart-name",
			Usage:  "Chart name to update. Will use git branch if not defined.",
			EnvVar: "CHART_NAME,CICD_GIT_BRANCH",
		},
		cli.StringSliceFlag{
			Name:     "chart-tags",
			Usage:    "Semver Tag or .tags file. Will use first semver found.",
			EnvVar:   "CHART_TAG,CHART_TAGS",
			FilePath: ".tags",
		},
		cli.BoolFlag{
			Name:   "dry-run",
			Usage:  "Don't post upgrades.",
			EnvVar: "DRY_RUN",
		},
		cli.StringFlag{
			Name:   "rancher-api-token",
			Usage:  "Bearer Token for Rancher API",
			EnvVar: "RANCHER_API_TOKEN",
		},
		cli.StringFlag{
			Name:   "rancher-url",
			Usage:  "Rancher URL",
			EnvVar: "RANCHER_URL",
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	plugin := Plugin{
		RancherURL:      c.String("rancher-url"),
		RancherAPIToken: c.String("rancher-api-token"),
		CatalogName:     c.String("catalog-name"),
		ChartTags:       c.StringSlice("chart-tags"),
		ChartName:       c.String("chart-name"),
		DryRun:          c.Bool("dry-run"),
	}

	// there has to be a better way to check flags
	required := []string{
		"rancher-url",
		"rancher-api-token",
		"catalog-name",
		"chart-tags",
		"chart-name",
	}

	// cli package doesn't seem have a way to return the Usage for a flag :(
	for _, flag := range required {
		present := c.IsSet(flag)
		if !present {
			log.Fatalf("Missing Required Flag %s", flag)
		}
	}

	return plugin.Exec()
}
