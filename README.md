# Helm Kubeconform Action

A GitHub Action to validate Helm charts with Kubeconform.

## Usage

TODO.

## Rationale

I needed an action to validate some Helm charts.
nlamirault/helm-kubeconform-action doesn’t offer enough flexibility
and [downloads two Git repositories during
execution](https://github.com/nlamirault/helm-kubeconform-action/blob/d29c4d227a42190dae7b25e668a267539d68a6ce/entrypoint.sh#L31-L51).
It was a good opportunity to write some bad Go and dip my toes into
the world of writing GitHub Actions—specifically, [a Docker container
action](https://docs.github.com/en/actions/creating-actions/creating-a-docker-container-action).
