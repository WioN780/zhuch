# zhuch infra (Terraform)

Small, honest Terraform config for the zhuch ML stack. Budget: **$0/month** — everything fits free tiers.

## What this manages

- **Cloudflare R2 bucket** `zhuch-mlflow-artifacts` — S3-compatible artifact store for MLflow (R2 free tier: 10 GB storage, zero egress).
- **Railway game-server service** (adopted via import, not recreated) plus its environment variables (`BOT_MODEL`, `KAFKA_BROKERS` placeholder).

## What this deliberately does NOT manage

- **R2 API tokens** — created manually (below). Terraform-managing them would require a bootstrap token with "API Tokens: Edit" (privilege escalation) and the S3 secret would sit in plaintext state anyway.
- **Railway deployments / source config** — Railway builds from the repo on push; the dashboard owns build settings.
- **The MLflow stack itself** — that runs locally via docker compose; only its artifact bucket lives here.
- **Remote state / locking** — local state is fine for a solo project with no CI applies. On a team: `backend "s3"` pointed at an R2 bucket (R2 speaks S3) with `use_lockfile = true` for locking.

## One-time setup: two tokens

1. **Cloudflare API token** (`cloudflare_api_token` var): dashboard -> My Profile -> API Tokens -> Create Token -> Custom, permission **Account / Workers R2 Storage / Edit**, scoped to your account.
2. **Railway token** (`railway_token` var): https://railway.com/account/tokens -> create an **account or workspace token** (project tokens can't manage services).

And the manual one Terraform skips:

3. **R2 S3 credentials for MLflow**: dashboard -> R2 -> Manage API Tokens -> Create API Token -> **Object Read & Write**, scope: **Apply to specific buckets** -> `zhuch-mlflow-artifacts`. Copy the Access Key ID and Secret Access Key.

## Usage

```sh
cd infra/terraform
cp terraform.tfvars.example terraform.tfvars   # fill in real values
terraform init
terraform plan     # review before touching anything
terraform apply
terraform destroy  # tears down the bucket + managed variables; empty the bucket first
```

## Adopting the existing Railway service (import)

The project/service already exist — do **not** let Terraform create a duplicate.

1. Fill `railway_project_id`, `railway_environment_id`, `railway_service_id` in `terraform.tfvars` (each Settings page in the Railway dashboard shows its ID).
2. Uncomment the `import` block in `railway.tf`.
3. `terraform plan` — the service must say **"will be imported"**, not "will be created". If the plan shows changes (e.g. a rename), edit the resource to match reality until the plan is clean.
4. `terraform apply`, then delete the import block (it's a one-shot instruction).

Existing env vars can be imported too:
`terraform import railway_variable.bot_model '<service_id>:<environment_name>:BOT_MODEL'`

## Wiring MLflow to R2

R2 is S3-compatible; MLflow just needs the endpoint override and credentials. In the MLflow docker-compose environment:

```yaml
environment:
  MLFLOW_S3_ENDPOINT_URL: https://<account_id>.r2.cloudflarestorage.com  # terraform output mlflow_s3_endpoint_url
  AWS_ACCESS_KEY_ID: <r2 access key id>        # from step 3 above
  AWS_SECRET_ACCESS_KEY: <r2 secret access key>
# server command:
#   mlflow server --default-artifact-root s3://zhuch-mlflow-artifacts ...
```

Training clients that log artifacts need the same three variables in their environment.

## Free-tier limits worth knowing

- R2: 10 GB-month storage, 1M class A / 10M class B ops per month, no egress fees. Prune old MLflow runs before hitting 10 GB.
- Railway: existing service unchanged; this config only sets env vars on it.
