#!/bin/bash

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ID="${GCP_PROJECT_ID:-}"
REGION="${GCP_REGION:-europe-southwest1}"
BACKEND_SERVICE="${BACKEND_SERVICE_NAME:-clicker-backend}"
CONSUMER_SERVICE="${CONSUMER_SERVICE_NAME:-clicker-consumer}"
ARTIFACT_REPO="${ARTIFACT_REPO:-clicker-repo}"

echo -e "${YELLOW}=== Clicker GCP Deployment ===${NC}"

# Validate configuration
if [ -z "$PROJECT_ID" ]; then
    echo -e "${RED}Error: GCP_PROJECT_ID not set${NC}"
    echo "Usage: export GCP_PROJECT_ID=your-project-id && ./deploy.sh"
    exit 1
fi

echo -e "${YELLOW}Project ID: ${PROJECT_ID}${NC}"
echo -e "${YELLOW}Region: ${REGION}${NC}"

# Set up Google Cloud configuration
echo -e "${YELLOW}Configuring gcloud...${NC}"
gcloud config set project "$PROJECT_ID"
gcloud config set compute/region "$REGION"

# Create Artifact Registry repository if it doesn't exist
echo -e "${YELLOW}Setting up Artifact Registry...${NC}"
if ! gcloud artifacts repositories describe "$ARTIFACT_REPO" --location="$REGION" &>/dev/null; then
    echo "Creating repository $ARTIFACT_REPO..."
    gcloud artifacts repositories create "$ARTIFACT_REPO" \
        --repository-format=docker \
        --location="$REGION" \
        --description="Docker repository for Clicker services"
fi

# Build and push backend image
echo -e "${YELLOW}Building and pushing backend image...${NC}"
BACKEND_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$ARTIFACT_REPO/$BACKEND_SERVICE:latest"
docker build -t "$BACKEND_IMAGE" ./backend
docker push "$BACKEND_IMAGE"
echo -e "${GREEN}Backend image pushed: $BACKEND_IMAGE${NC}"

# Build and push consumer image
echo -e "${YELLOW}Building and pushing consumer image...${NC}"
CONSUMER_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$ARTIFACT_REPO/$CONSUMER_SERVICE:latest"
docker build -t "$CONSUMER_IMAGE" ./consumer
docker push "$CONSUMER_IMAGE"
echo -e "${GREEN}Consumer image pushed: $CONSUMER_IMAGE${NC}"

# Apply Terraform configuration
echo -e "${YELLOW}Applying Terraform configuration...${NC}"
cd terraform

terraform init

terraform apply -auto-approve \
    -var="gcp_project_id=$PROJECT_ID" \
    -var="gcp_region=$REGION" \
    -var="backend_docker_image=$BACKEND_IMAGE" \
    -var="consumer_docker_image=$CONSUMER_IMAGE"

# Get outputs
echo -e "${YELLOW}Getting service URLs...${NC}"
BACKEND_URL=$(terraform output -raw backend_url)
CONSUMER_URL=$(terraform output -raw consumer_url)

echo -e "${GREEN}Deployment complete!${NC}"
echo ""
echo -e "${YELLOW}Service URLs:${NC}"
echo -e "  Backend:  ${GREEN}$BACKEND_URL${NC}"
echo -e "  Consumer: ${GREEN}$CONSUMER_URL${NC}"

echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Initialize Firestore: ./scripts/init-firestore.sh"
echo "2. Access the application at the backend URL above"
echo "3. Test the application - frontend is served from the backend"

# Return to root directory
cd ..

echo -e "${GREEN}Done!${NC}"
