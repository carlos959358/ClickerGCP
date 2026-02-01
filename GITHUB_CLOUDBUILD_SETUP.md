# GitHub + Cloud Build Integration Setup

This guide explains how to set up automatic CI/CD so that pushing to the main branch automatically builds and deploys your application.

## 📋 Prerequisites

- GitHub account with repository
- GCP project with Cloud Build API enabled
- Project Owner or Editor role in GCP

## 🔗 Step 1: Connect GitHub to Cloud Build

### Via Google Cloud Console

1. Go to [Cloud Build Console](https://console.cloud.google.com/cloud-build/builds)
   - Select your GCP project: `dev-trail-475809-v2`

2. Click **Create a Build** or **Manage Repositories**

3. Click **Connect Repository**

4. Select **GitHub** as source

5. Click **Authorize Cloud Build** (authenticates with GitHub)

6. Select your GitHub account and authorize the request

7. Select your ClickerGCP repository

8. Click **Connect Repository**

## 🏗️ Step 2: Create Cloud Build Trigger

1. In Cloud Build, go to **Triggers**

2. Click **Create Trigger**

3. Configure the trigger:
   ```
   Name: ClickerGCP Main Deploy
   Repository: Your GitHub repo (ClickerGCP)
   Branch: ^main$
   Build configuration: Cloud Build configuration file
   Cloud Build configuration file location: cloudbuild.yaml
   ```

4. Click **Create**

## 🔐 Step 3: Grant Cloud Build Permissions

Cloud Build needs permission to deploy resources. Run:

```bash
export PROJECT_ID="dev-trail-475809-v2"
export CLOUD_BUILD_SA="$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')@cloudbuild.gserviceaccount.com"

# Grant Terraform permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/compute.admin

# Grant Cloud Run permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/run.admin

# Grant Firestore permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/datastore.admin

# Grant Pub/Sub permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/pubsub.admin

# Grant Service Account permissions (to create service accounts)
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/iam.serviceAccountAdmin

# Grant Artifact Registry permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/artifactregistry.admin

# Grant Firestore admin access
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/firestore.admin
```

## 📦 Step 4: Prepare Your Repository

Ensure your repository has:

```
ClickerGCP/
├── cloudbuild.yaml          ✅ Created (CI/CD pipeline)
├── terraform/
│   ├── main.tf
│   ├── variables.tf
│   ├── cloudrun.tf
│   ├── iam.tf
│   ├── pubsub.tf
│   ├── firestore.tf
│   ├── outputs.tf
│   ├── backend.tf           ✅ GCS state backend
│   └── .terraform/          ❌ Ignored (.gitignore)
├── backend/
│   ├── main.go
│   ├── firestore.go
│   ├── Dockerfile
│   └── static/
├── consumer/
│   ├── main.go
│   ├── firestore.go
│   ├── notifier.go
│   └── Dockerfile
├── .gitignore               ✅ Protects secrets
├── terraform.tfvars         ❌ Local only, not committed
├── .env                     ❌ Local only, not committed
└── scripts/
    └── deploy.sh
```

**Critical**: Ensure `.gitignore` includes:
```
terraform.tfvars
.env
*.tfstate
*.tfstate.backup
.terraform/
```

## 🚀 Step 5: Deploy

### Option A: Automatic (Push to Main)

```bash
# Make your changes
git add .
git commit -m "Deploy update"
git push origin main
```

Cloud Build will automatically:
1. Build backend and consumer Docker images
2. Push images to Artifact Registry
3. Run Terraform to deploy/update services
4. Output status and service URLs

### Option B: Manual Trigger

In Cloud Build console:
1. Go to **Triggers**
2. Click your trigger
3. Click **Run**

## 📊 Monitoring Builds

### View Build Status
1. Cloud Build console → **Builds**
2. Click on a build to see:
   - Build logs (each step)
   - Build status
   - Duration
   - Errors (if any)

### View Build Logs
```bash
# Stream logs
gcloud builds log $(gcloud builds list --limit=1 --format='value(id)')

# List recent builds
gcloud builds list --limit=10
```

## 🔍 Troubleshooting

### Build Fails: "Image not found"
- Ensure backend/Dockerfile and consumer/Dockerfile exist
- Check paths in cloudbuild.yaml

### Build Fails: "Permission denied"
- Verify Cloud Build service account has required IAM roles
- Check the role assignments from Step 3

### Terraform Apply Fails
- Check Terraform state in GCS bucket
- Verify backend.tf bucket name matches
- Review terraform plan in build logs

### Docker Build Fails
- Check build logs for Go compilation errors
- Verify Dockerfile syntax
- Ensure dependencies are correct

## 📝 Environment Variables

Cloud Build uses variable substitution. Key variables:

| Variable | Value | Used By |
|----------|-------|---------|
| `PROJECT_ID` | GCP Project ID | All resources |
| `COMMIT_SHA` | Git commit hash | Image tagging |
| `BRANCH_NAME` | Git branch name | Build triggers |
| `_REGION` | europe-southwest1 | Artifact Registry location |
| `_ARTIFACT_REPO` | clicker-repo | Image repository |

## 🔐 Security Notes

- Cloud Build uses service accounts (no credentials in repo)
- State files stored in GCS with versioning
- Docker images pushed to Artifact Registry (private)
- Secrets stored in Secret Manager (if needed later)

## ✅ Verification

After first successful build:

```bash
# Check Cloud Run services
gcloud run services list --region=europe-southwest1

# Check Artifact Registry images
gcloud artifacts docker images list europe-southwest1-docker.pkg.dev/dev-trail-475809-v2/clicker-repo

# Check Firestore database
gcloud firestore databases list

# Check Pub/Sub topic
gcloud pubsub topics list
```

## 🚀 Next Steps

1. Push repository to GitHub
2. Create the Cloud Build trigger (Step 2)
3. Grant permissions (Step 3)
4. Make a test commit to main
5. Watch automatic deployment in Cloud Build console

For more info: [Cloud Build Documentation](https://cloud.google.com/build/docs)
