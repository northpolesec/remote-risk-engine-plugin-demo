# Configuration
PROJECT_ID = your-project-here
REGION = us-central1
SERVICE_NAME = risk-engine
IMAGE_NAME = itunes
REPO_NAME = remote-plugin
IMAGE_URL = $(REGION)-docker.pkg.dev/$(PROJECT_ID)/$(REPO_NAME)/$(IMAGE_NAME)

.PHONY: all build push deploy logs tail clean

# Build, push, and deploy everything
all: build push deploy

# Build the static Go binary
build:
	@echo "Building static Go binary..."
	CGO_ENABLED=0 go build -a -installsuffix cgo -o app ./cmd/server.go

# Build and push container to Artifact Registry
push:
	@echo "Building and pushing container..."
	gcloud builds submit --tag $(IMAGE_URL)

# Deploy to Cloud Run
deploy:
	@echo "Deploying to Cloud Run..."
	gcloud run deploy $(SERVICE_NAME) \
		--image $(IMAGE_URL) \
		--region $(REGION)

# Read recent logs
logs:
	gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=$(SERVICE_NAME)" \
		--limit=50 \
		--project=$(PROJECT_ID) \
		--format="table(timestamp,severity,textPayload)"
