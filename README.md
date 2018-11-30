# rancher-pipeline-deploy

1. Refresh Rancher Catalog
1. Scan through Rancher 2.x Projects. 
1. Look for deployed "Catalog Apps" that match our Catalog and Chart. 
1. Update if chart values have `rancher.autoUpdate: true`.

## Environment Vars

### Required

| VAR | Description |
| --- | --- |
| `RANCHER_CATALOG_NAME` | Name of the Rancher Catalog to refresh and look for updates. |
| `RANCHER_URL` | Base URL for your Rancher Server instance. |
| `RANCHER_API_TOKEN` | Bearer token for the Rancher user. Use k8s secret and `envFrom` syntax with Rancher Pipelines. |

### Optional (Overrides)

| VAR | Default | Description |
| --- | --- | --- |
| `CHART_NAME` | `CICD_GIT_BRANCH` | Use git branch by default. |
| `CHART_TAGS` | Use `.tags` file | Use `.tags` file or a comma separated list of tags. |
| `DRY_RUN` | `false` | Do everything but update apps found |

## Rancher User Setup

### Create User
Create a "local" user for restricted automation access.

`Global` -> `Users` -> `Add User`

Under Global Permissions select Custom and assign the following permissions:

* `Manage Catalogs`
* `Use Catalog Templates`

### Create Bearer Token

Login with you new automation that user and generate an API Key.

`User Icon` -> `API & Keys` -> `Add Key`

Use the Bearer Token value (`<userid>:<secret_key>`) for the `RANCHER_API_TOKEN`

## Picking what to Auto Update

What gets evaluated for upgrades is controlled through two levels.

1. Which Projects the API user has access to.
2. Deployed Catalog Apps with `rancher.autoUpgrade: true`.

First control access to projects. Assign the automation user the `Member` role on Clusters or individual Projects you want to auto update.

Second make sure `rancher.autoUpgrade: true` is included in the `set` values for your helm chart.  You can create a simple toggle for this in the Catalog App GUI by including a `questions.yaml` file in your chart.

### Example .chart/questions.yaml

```yaml
questions:
- label: Automatic Update
  description: Automatically update on pipeline chart update.
  variable: rancher.autoUpdate
  default: "false"
  type: boolean
  required: true
```

## Example `.rancher-pipeline.yml`

`rancher-pipeline-publish-chart` is used in the "Render and Publish Helm Charts" stage.

```yaml
stages:
- name: Create Build Tag
  steps:
  - runScriptConfig:
      image: jgreat/drone-build-tag:0.1.0
      shellScript: build-tags.sh --include-feature-tag
- name: Build and Publish Image
  steps:
  - publishImageConfig:
      dockerfilePath: ./Dockerfile
      buildContext: .
      tag: jgreat/vote-demo-web:use-tags-file
      pushRemote: true
      registry: index.docker.io
- name: Render and Publish Helm Charts
  steps:
  - runScriptConfig:
      image: jgreat/rancher-pipeline-publish-chart:0.0.2
      shellScript: publish-chart.sh
    env:
      HELM_REPO_URL: https://vote-demo-charts.eng.rancher.space/vote-demo-web/
    envFrom:
    - sourceName: chart-creds
      sourceKey: BASIC_AUTH_PASS
      targetKey: HELM_REPO_PASSWORD
    - sourceName: chart-creds
      sourceKey: BASIC_AUTH_USER
      targetKey: HELM_REPO_USERNAME
- name: Upgrade Catalog Apps
  steps:
  - runScriptConfig:
      image: jgreat/rancher-pipeline-deploy:0.0.2
      shellScript: rancher-pipeline-deploy
    env:
      RANCHER_URL: https://jgreat-vote-rancher.eng.rancher.space
      RANCHER_CATALOG_NAME: vote-demo-web
    envFrom:
    - sourceName: chart-creds
      sourceKey: RANCHER_API_TOKEN
      targetKey: RANCHER_API_TOKEN
timeout: 10
```