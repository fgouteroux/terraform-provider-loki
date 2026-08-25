# The whole namespace is imported: content is rebuilt from the rule groups loki
# already holds, so no configuration has to be written beforehand.
terraform import loki_rules.test {{namespace}}

# With a tenant:
terraform import loki_rules.test {{org_id/namespace}}
