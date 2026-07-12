variable "cloudflare_account_id" {
  description = "Cloudflare account ID (dashboard -> R2, shown in the right sidebar)."
  type        = string
}

variable "cloudflare_api_token" {
  description = "Cloudflare API token with 'Workers R2 Storage: Edit' permission. See README."
  type        = string
  sensitive   = true
}

variable "railway_token" {
  description = "Railway account or workspace token (https://railway.com/account/tokens)."
  type        = string
  sensitive   = true
}

variable "project_name" {
  description = "Project name, used as a prefix for resource names."
  type        = string
  default     = "zhuch"
}

variable "environment" {
  description = "Environment name."
  type        = string
  default     = "production"
}

# --- Existing Railway resources (for import, see railway.tf) ---

variable "railway_project_id" {
  description = "ID of the existing Railway project (dashboard -> project -> Settings)."
  type        = string
}

variable "railway_environment_id" {
  description = "ID of the Railway environment the game server runs in."
  type        = string
}

variable "railway_service_id" {
  description = "ID of the existing game-server service (dashboard -> service -> Settings). Used only by the import block."
  type        = string
}

variable "railway_service_name" {
  description = "Name of the game-server service as it exists in Railway (must match, or the import will plan a rename)."
  type        = string
  default     = "zhuch-server"
}
