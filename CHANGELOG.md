# Changelog

## [0.3.0](https://github.com/shivjm/helm-kubeconform-action/compare/v0.2.0...v0.3.0) (2024-02-14)


### Features

* allow skipping directories ([d231947](https://github.com/shivjm/helm-kubeconform-action/commit/d231947060daf79af952e5756b95eddda2b43c50))
* allow validating a single chart directory ([8292f61](https://github.com/shivjm/helm-kubeconform-action/commit/8292f611662fa1f409b370e3837dc40c9ff2ca41))
* propagate JSON output from kubeconform upon validation error ([bb9fc5c](https://github.com/shivjm/helm-kubeconform-action/commit/bb9fc5cbd80c2d9882c260ed0b30c3a4f91f98ef))
* validate all charts and report all failures ([e45a5c8](https://github.com/shivjm/helm-kubeconform-action/commit/e45a5c8a6dce87e8bb79785a37879a659b532b71))


### Bug Fixes

* correctly handle kubeconform not being executed ([52c4151](https://github.com/shivjm/helm-kubeconform-action/commit/52c4151e3b4129f945040ec8648a9ff549e92237))
* remove extraneous output when log level is unparseable ([c8c211b](https://github.com/shivjm/helm-kubeconform-action/commit/c8c211bea42ffc3c54987364d5571c57840e8f70))
* simplify logging ([c89dac8](https://github.com/shivjm/helm-kubeconform-action/commit/c89dac8d99f1fca42ce1971062df1c3412af4b90))
