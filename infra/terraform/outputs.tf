output "r2_bucket_name" {
  description = "R2 bucket holding MLflow artifacts."
  value       = cloudflare_r2_bucket.mlflow_artifacts.name
}

output "mlflow_s3_endpoint_url" {
  description = "Set as MLFLOW_S3_ENDPOINT_URL in the MLflow docker-compose environment."
  value       = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
}

output "mlflow_artifact_root" {
  description = "Pass to `mlflow server --default-artifact-root`."
  value       = "s3://${cloudflare_r2_bucket.mlflow_artifacts.name}"
}

output "next_steps" {
  description = "Manual step terraform deliberately does not do."
  value       = "Create an R2 API token (dashboard -> R2 -> Manage API Tokens, Object Read & Write, scope: this bucket) and put the key pair in the MLflow compose env as AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY. See infra/terraform/README.md."
}
