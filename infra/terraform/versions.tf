terraform {
  required_version = ">= 1.7"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0" # v5 is the current major (5.22.0 as of 2026-07)
    }
    railway = {
      source  = "terraform-community-providers/railway"
      version = "~> 0.6" # most-maintained community Railway provider (0.6.2)
    }
  }

  # ponytail: local state on purpose — solo project, no CI applies.
  # For a team: backend "s3" pointed at an R2 bucket (R2 is S3-compatible)
  # + use_lockfile = true for state locking. See README.
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

provider "railway" {
  token = var.railway_token
}
