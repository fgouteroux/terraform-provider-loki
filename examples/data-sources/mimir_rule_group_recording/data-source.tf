data "loki_rule_group_recording" "record" {
  name      = "test1"
  namespace = "namespace1"
}