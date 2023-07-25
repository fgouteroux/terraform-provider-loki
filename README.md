# Terraform provider for grafana loki

This terraform provider allows you to interact with grafana loki.

See [Loki API Reference](https://grafana.com/docs/loki/latest/api/)

## Provider `loki`

Example:

```
provider "loki" {
  ruler_uri = "http://localhost:3100"
  org_id = "mytenant"
}
```


### Authentication

Grafana Loki have no authentication support, so this is delegated to a reverse proxy.

The provider support basic auth, token.

#### Basic auth

```
provider "loki" {
  ruler_uri = "http://localhost:3100"
  org_id = "mytenant"
  username = "user"
  password = "password"
}
```

#### Token

```
provider "loki" {
  ruler_uri = "http://localhost:3100"
  org_id = "mytenant"
  token = "supersecrettoken"
}
```

### Headers

```
provider "loki" {
  ruler_uri = "http://localhost:3100"
  org_id = "mytenant"
  header = {
    "Custom-Auth" = "Custom value"
  }
}
```

## Resource `loki_rule_group_alerting`

Example:

```
resource "loki_rule_group_alerting" "test" {
  name      = "test1"
  namespace = "namespace1"
  rule {
    alert       = "HighPercentageError"
    expr        = <<EOT
sum(rate({app="foo", env="production"} |= "error" [5m])) by (job)
  /
sum(rate({app="foo", env="production"}[5m])) by (job)
  > 0.05
    for         = "10m"
    labels      = {
      severity = "warning"
    }
    annotations = {
      summary = "High request latency"
    }
  }
}
```

## Resource `loki_rule_group_recording`

Example:

```
resource "loki_rule_group_recording" "record" {
  name      = "test1"
  namespace = "namespace1"
  rule {
    expr   = "sum(rate({container=\"nginx\"}[1m]))"
    record = "nginx:requests:rate1m"
  }
}
```

## Importing existing resources
This provider supports importing existing resources into the terraform state. Import is done according to the various provider/resource configuation settings to contact the API server and obtain data.

### loki alerting rule group

To import loki rule group alerting
The id is build as `<namespace>/<name>`

Example:

```
terraform import 'loki_rule_group_alerting.alert1' namespace1/alert1
loki_rule_group_alerting.alert1: Importing from ID "namespace1/alert1"...
loki_rule_group_alerting.alert1: Import prepared!
  Prepared loki_rule_group_alerting for import
loki_rule_group_alerting.alert1: Refreshing state... [id=namespace1/alert1]

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.

```

### loki recording rule group

To import loki rule group recording
The id is build as `<namespace>/<name>`

Example:

```
terraform import 'loki_rule_group_recording.record1' namespace1/record1
loki_rule_group_recording.record1: Importing from ID "namespace1/record1"...
loki_rule_group_recording.record1: Import prepared!
  Prepared loki_rule_group_recording for import
loki_rule_group_recording.record1: Refreshing state... [id=namespace1/record1]

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.

```

## Contributing
Pull requests are always welcome! Please be sure the following things are taken care of with your pull request:
* `go fmt` is run before pushing
* Be sure to add a test case for new functionality (or explain why this cannot be done)

