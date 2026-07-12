# S3-compatible artifact store for MLflow.
#
# Free-tier budget (R2, per month): 10 GB-month storage, 1M class A ops,
# 10M class B ops, zero egress fees. MLflow artifacts (model checkpoints,
# metrics dumps) fit comfortably; prune old runs if you approach 10 GB.
# If you ever want auto-expiry of old artifacts, the provider has a
# cloudflare_r2_bucket_lifecycle resource — deliberately not used here,
# because silently deleting trained models is worse than a manual prune.
resource "cloudflare_r2_bucket" "mlflow_artifacts" {
  account_id = var.cloudflare_account_id
  name       = "${var.project_name}-mlflow-artifacts"
  # location left unset: Cloudflare picks the closest region.
}

# R2 API token (the S3 access key / secret pair) is created MANUALLY —
# see README. The cloudflare provider can create generic API tokens
# (cloudflare_api_token), but wiring one up for R2 S3 access requires the
# bootstrap token to hold "API Tokens: Edit" (privilege escalation) and
# the derived S3 secret would land in plaintext state anyway.
# One click in the dashboard is safer and simpler.
