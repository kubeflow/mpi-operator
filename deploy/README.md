# How to create a validation CRD for mpijob

* Submit a PR to update the vendor version for [crd-validation](https://github.com/github.com/crd-validation)
* Build and generate a new API validation CRD
  * build the [crd-validation](https://github.com/github.com/crd-validation)
  * `./crd-validation -c ./examples/mpijob/crd-validation.yaml mpijob`
* Copy it to this directory
