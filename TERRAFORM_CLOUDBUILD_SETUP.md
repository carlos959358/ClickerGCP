# Terraform: Cloud Build + GitHub Setup

This guide explains how to set up automatic CI/CD using **Terraform** (Infrastructure as Code).

## ✨ Advantages of Using Terraform

- ✅ **Infrastructure as Code** - Everything version controlled
- ✅ **Reproducible** - Same setup every time, everywhere
- ✅ **Easy to Modify** - Change trigger logic in terraform/cloudbuild.tf
- ✅ **Audit Trail** - Git history shows all changes
- ✅ **Team Friendly** - Share configuration via Git

## 📋 Prerequisites

- GCP Project: `dev-trail-475809-v2`
- GitHub repository (with this project code)
- GitHub personal access token (for authentication)

## 🔧 Step 1: Create GitHub Personal Access Token

1. Go to [GitHub Settings → Developer settings → Personal Access Tokens](https://github.com/settings/tokens)

2. Click **Generate new token (classic)**

3. Set scopes:
   ```
   ✓ repo (Full control of private repositories)
   ✓ read:user
   ✓ user:email
   ```

4. Copy the token (you won't see it again!)

## 🔐 Step 2: Store Token in GCP Secret Manager

Store your GitHub token securely:

```bash
export GITHUB_TOKEN="your-github-token"
export PROJECT_ID="dev-trail-475809-v2"

# Create secret in Secret Manager
echo -n "$GITHUB_TOKEN" | gcloud secrets create github-token \
  --data-file=- \
  --project=$PROJECT_ID

# Grant Cloud Build service account access to the secret
CLOUD_BUILD_SA="$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')@cloudbuild.gserviceaccount.com"

gcloud secrets add-iam-policy-binding github-token \
  --member=serviceAccount:$CLOUD_BUILD_SA \
  --role=roles/secretmanager.secretAccessor \
  --project=$PROJECT_ID
```

## 📝 Step 3: Update terraform.tfvars

Edit `terraform.tfvars` and update GitHub details:

```hcl
# GitHub Configuration for Cloud Build CI/CD
github_owner = "your-github-username"  # Your GitHub username
github_repo  = "ClickerGCP"            # Your repository name
```

## 🚀 Step 4: Deploy Cloud Build Trigger with Terraform

```bash
cd /home/carlos/Desktop/DevProjects/ClickerGCP/terraform

# Copy terraform.tfvars if not already in terraform dir
cp ../terraform.tfvars .

# Initialize Terraform (if not already done)
terraform init

# Plan the Cloud Build setup
terraform plan

# Apply the configuration
terraform apply -auto-approve
```

Terraform will create:
- ✅ Cloud Build Trigger (GitHub → GCP integration)
- ✅ Service Account IAM bindings
- ✅ All necessary permissions

## 🔗 Step 5: Authorize Cloud Build with GitHub

After Terraform applies, you need to authorize Cloud Build:

1. Go to [Cloud Build Triggers Console](https://console.cloud.google.com/cloud-build/triggers)

2. Click on your trigger: `clicker-gcp-github-main`

3. Click **Connect** (or **Reconnect Repository**)

4. Click **Authorize Cloud Build**

5. Select your GitHub account

6. Click **Install** to give Cloud Build access to your repo

## 🎯 Step 6: Enable Cloud Build Access in GitHub

1. In GitHub, go to **Settings → Applications → Authorized OAuth Apps**

2. Click **Google Cloud Build**

3. Grant it access to your ClickerGCP repository

## 📤 Step 7: Push to GitHub

```bash
# From repo root
git add .
git commit -m "Add Cloud Build CI/CD configuration"
git push origin main
```

Cloud Build will automatically:
1. Detect the push to main
2. Start the build
3. Execute cloudbuild.yaml steps:
   - Build backend image
   - Build consumer image
   - Push to Artifact Registry
   - Run terraform apply

## 📊 Monitor the Build

View the build status:

```bash
# List recent builds
gcloud builds list --limit=10

# Stream logs for latest build
gcloud builds log $(gcloud builds list --limit=1 --format='value(id)') --stream
```

Or via [Cloud Build Console](https://console.cloud.google.com/cloud-build/builds)

## 🔄 How the CI/CD Pipeline Works

```
┌──────────────────────────────────┐
│ Developer: git push to main      │
└─────────────┬────────────────────┘
              │
              ▼
┌──────────────────────────────────┐
│ GitHub notifies Cloud Build      │
│ (via webhook)                    │
└─────────────┬────────────────────┘
              │
              ▼
┌──────────────────────────────────┐
│ Cloud Build executes:            │
│                                  │
│ 1. Build backend Docker image    │
│ 2. Build consumer Docker image   │
│ 3. Push images to Artifact Reg   │
│ 4. Run: terraform init           │
│ 5. Run: terraform plan           │
│ 6. Run: terraform apply          │
└─────────────┬────────────────────┘
              │
              ▼
┌──────────────────────────────────┐
│ GCP Resources Updated:           │
│ - Cloud Run services             │
│ - Firestore database             │
│ - Pub/Sub topic                  │
│ - Service accounts & IAM         │
└──────────────────────────────────┘
```

## 🛠️ Modifying the Trigger

All trigger logic is in `terraform/cloudbuild.tf`:

### Change trigger branch:
```hcl
push {
  branch = "^develop$"  # Now triggers on develop instead of main
}
```

### Add new trigger for staging:
```hcl
resource "google_cloudbuild_trigger" "github_staging" {
  name = "clicker-gcp-github-staging"

  github {
    owner = var.github_owner
    name  = var.github_repo
    push {
      branch = "^staging$"
    }
  }

  filename = "cloudbuild.yaml"
}
```

Then apply:
```bash
terraform apply
```

## 🐛 Troubleshooting

### Build fails: "Repository not connected"
- Go to Cloud Build console
- Click **Connect Repository** on your trigger
- Authorize Cloud Build in GitHub

### Build fails: "No such file or directory: cloudbuild.yaml"
- Ensure `cloudbuild.yaml` is in repo root
- Ensure it's committed and pushed to main

### Build fails: "Permission denied"
- Check Cloud Build service account has required roles
- Verify Terraform applied successfully

### Build fails in Terraform step
- Check Terraform plan output in Cloud Build logs
- Run `terraform validate` locally to find errors
- Fix errors and push to main again

## 📈 Next Steps

1. **Set up branch protection**:
   - Go to GitHub repo → Settings → Branches
   - Require builds to pass before PR merge

2. **Add notifications**:
   - Cloud Build → Integrations
   - Set up Slack/Email notifications

3. **Scale up**:
   - Add triggers for `staging` branch
   - Add triggers for `develop` branch
   - Different environments per branch

## 🔐 Security Best Practices

- ✅ GitHub token stored in Secret Manager (not in code)
- ✅ Terraform state in GCS with versioning
- ✅ Cloud Build uses service account (not personal credentials)
- ✅ IAM roles follow least-privilege principle
- ✅ All secrets protected by GCP security

## 📚 Terraform Files Reference

| File | Purpose |
|------|---------|
| `terraform/cloudbuild.tf` | Cloud Build trigger and permissions |
| `terraform/variables.tf` | GitHub owner/repo variables |
| `terraform.tfvars` | Your GitHub configuration |
| `cloudbuild.yaml` | CI/CD pipeline steps |

## ✅ Verification

After successful setup:

```bash
# Check trigger exists
gcloud builds triggers list

# Check recent builds
gcloud builds list --limit=5

# Check Cloud Build service account permissions
gcloud projects get-iam-policy dev-trail-475809-v2 \
  --flatten="bindings[].members" \
  --filter="bindings.members:cloudbuild.gserviceaccount.com"
```

## 🎉 Done!

Your CI/CD pipeline is now fully automated:
- Every push to main triggers builds automatically
- Images built in GCP
- Services deployed to Cloud Run
- All tracked in Git and Terraform
- Everything Infrastructure as Code

For more info: [Cloud Build Terraform Provider](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloudbuild_trigger)
