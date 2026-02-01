#!/bin/bash

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ID="${GCP_PROJECT_ID:-}"
DATABASE_ID="${FIRESTORE_DATABASE:-clicker-db}"

echo -e "${YELLOW}=== Firestore Initialization ===${NC}"

# Validate configuration
if [ -z "$PROJECT_ID" ]; then
    echo -e "${RED}Error: GCP_PROJECT_ID not set${NC}"
    echo "Usage: export GCP_PROJECT_ID=your-project-id && ./init-firestore.sh"
    exit 1
fi

echo -e "${YELLOW}Project ID: ${PROJECT_ID}${NC}"
echo -e "${YELLOW}Database ID: ${DATABASE_ID}${NC}"

# Set up Google Cloud configuration
gcloud config set project "$PROJECT_ID"

# Check if database exists
echo -e "${YELLOW}Checking Firestore database...${NC}"
if ! gcloud firestore databases describe --database="$DATABASE_ID" &>/dev/null; then
    echo -e "${RED}Error: Database $DATABASE_ID not found${NC}"
    echo "Create it using Terraform first: terraform apply"
    exit 1
fi

echo -e "${GREEN}Database found: $DATABASE_ID${NC}"

# Note: Global counter will be auto-initialized by the backend on first use
echo -e "${YELLOW}Firestore database is ready. Global counter will be auto-initialized by backend.${NC}"

echo -e "${GREEN}Firestore initialization complete!${NC}"
