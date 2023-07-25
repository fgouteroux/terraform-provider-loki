provider "loki" {
  ruler_uri = "http://127.0.0.1:3100"
  org_id = "mytenant"
  username = "user"
  password = "password"
}