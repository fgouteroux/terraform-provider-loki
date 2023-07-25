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