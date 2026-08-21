provider "loki" {
  uri    = "https://private-alb.example.com"
  org_id = "mytenant"

  aws_sigv4 {
    region  = "us-east-1"   # Optional, defaults to AWS_REGION env var
    service = "execute-api" # Optional, defaults to "execute-api"
  }
}
