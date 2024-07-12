resource "loki_rule_group_recording" "test" {
  name      = "test1"
  namespace = "namespace1"
  rule {
    expr   = "sum by (job) (http_inprogress_requests)"
    record = "job:http_inprogress_requests:sum"
  }

  # can define multiple rules
  rule {
    expr   = "max by (job) (http_inprogress_requests)"
    record = "job:http_inprogress_requests:max"
    labels      = {
      foo = "bar"
    }
  }
}
