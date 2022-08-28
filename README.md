# Helm Kubeconform Action

A flexible GitHub Action to validate [Helm charts](https://helm.sh/docs/topics/charts/) with [Kubeconform](https://github.com/yannh/kubeconform/).

## Usage

Assuming you have a <kbd>charts</kbd> directory under which you have a
set of charts and a <kbd>schemas</kbd> directory containing any custom
resource schemas, like this:

```
charts
└───foo
│  ├───templates
│  └───tests
└───bar
│  ├───templates
│  └───tests
└───schemas
```

You can validate the charts in your workflow using the Docker image
directly, which is quicker but requires adding
[docker/login-action](https://github.com/docker/login-action) and
supplying the environment variables yourself:

```yaml
  kubeconform:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@master

    - name: Login to GitHub Container Registry
      uses: docker/login-action@v1
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - name: Generate and validate releases
      uses: docker://ghcr.io/shivjm/helm-kubeconform-action:v0.2.0
      env:
        ADDITIONAL_SCHEMA_PATHS: |
          schemas/{{ .ResourceKind }}.json
        CHARTS_DIRECTORY: "charts"
        KUBECONFORM_STRICT: "true"
        HELM_UPDATE_DEPENDENCIES: "true"
```

Or by using the action, which will rebuild the Docker image every time
but is easier to use:

```yaml
jobs:
    kubeconform:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@master

    - name: Generate and validate releases
      uses: shivjm/helm-kubeconform-action@v0.2.0
      with:
        additionalSchemaPaths: |
          schemas/{{ .ResourceKind }}.json
        chartsDirectory: "charts"
```

[See action.yml for more information on the parameters.](action.yml)

### Schemas

The [default Kubernetes
schema](https://github.com/yannh/kubernetes-json-schema/) will always
be automatically included. If you need to add custom schemas,
`additionalSchemaPaths` should be a list of paths, one per line, [in
the format expected by
Kubeconform](https://github.com/yannh/kubeconform/blob/d536a659bdb20ee6d06ab55886b348cd1c0fa21b/Readme.md#overriding-schemas-location---crd-and-openshift-support).
These are relative to the root of your repository.

### Tests

Every chart subdirectory must have a <kbd>tests</kbd> subdirectory
containing values files [as you would pass to
Helm](https://helm.sh/docs/intro/using_helm/#customizing-the-chart-before-installing).
Each file will be passed on its own to <kbd>helm template release
charts/<var>chart</var></kbd> and the results will be validated by
Kubeconform.

### Strict Mode

Kubeconform will be run in strict mode. Pass `strict: "false"` to
disable this.

## Rationale

I needed an action to validate some Helm charts.
[nlamirault/helm-kubeconform-action](https://github.com/nlamirault/helm-kubeconform-action/blob/d29c4d227a42190dae7b25e668a267539d68a6ce/entrypoint.sh#L31-L51)
doesn’t offer enough flexibility and [downloads two Git repositories
during
execution](https://github.com/nlamirault/helm-kubeconform-action/blob/d29c4d227a42190dae7b25e668a267539d68a6ce/entrypoint.sh#L31-L51).
It was a good opportunity to try writing some bad Go ([more about
that](https://shivjm.blog/helm-kubeconform-action/)) and dip my toes
into the world of writing GitHub Actions—specifically, [a Docker
container
action](https://docs.github.com/en/actions/creating-actions/creating-a-docker-container-action).
