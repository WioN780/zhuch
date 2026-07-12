# The Railway project and game-server service ALREADY EXIST (deployed
# manually). This config adopts them as code instead of recreating them.
#
# Import workflow (one-time):
#   1. Fill railway_project_id / railway_environment_id / railway_service_id
#      in terraform.tfvars (Railway dashboard -> Settings pages).
#   2. Uncomment the import block below.
#   3. `terraform plan`  -> the service must show "will be imported",
#      NOT "will be created". If the plan wants to change attributes
#      (e.g. rename), adjust the resource config to match reality first.
#   4. `terraform apply`
#   5. Delete the import block (it is a one-shot instruction, not config).
#
# import {
#   to = railway_service.game_server
#   id = var.railway_service_id
# }

resource "railway_service" "game_server" {
  name       = var.railway_service_name
  project_id = var.railway_project_id
  # Source (repo/branch) is managed in the Railway dashboard; after import,
  # `terraform plan` will show the real values — copy them here if you want
  # terraform to own them, or leave unset to keep dashboard-managed.
}

# Environment variables for the game server. Existing variables can also be
# imported: terraform import railway_variable.bot_model '<service_id>:<environment_name>:BOT_MODEL'
resource "railway_variable" "bot_model" {
  environment_id = var.railway_environment_id
  service_id     = railway_service.game_server.id
  name           = "BOT_MODEL"
  value          = "baseline-v0"
}

resource "railway_variable" "kafka_brokers" {
  environment_id = var.railway_environment_id
  service_id     = railway_service.game_server.id
  name           = "KAFKA_BROKERS"
  # ponytail: placeholder — no managed Kafka yet ($0 budget). Point this at
  # a real broker (e.g. a local tunnel or Upstash Kafka) when the ML
  # event pipeline goes live.
  value = "localhost:9092"
}
