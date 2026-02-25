IMAGE_REGISTRY ?= $(error IMAGE_REGISTRY is not set. Export it or pass via make, e.g. make deploy IMAGE_REGISTRY=quay.io/myorg)
TAG ?= latest

.PHONY: build-collector build-backend build-frontend build-all
.PHONY: push-collector push-backend push-frontend push-all
.PHONY: deploy undeploy

# ---- Build ----

build-collector:
	podman build -t $(IMAGE_REGISTRY)/audit-collector:$(TAG) collector/

build-backend:
	podman build -t $(IMAGE_REGISTRY)/audit-backend:$(TAG) backend/

build-frontend:
	podman build -t $(IMAGE_REGISTRY)/audit-frontend:$(TAG) frontend/

build-all: build-collector build-backend build-frontend

# ---- Push ----

push-collector:
	podman push $(IMAGE_REGISTRY)/audit-collector:$(TAG)

push-backend:
	podman push $(IMAGE_REGISTRY)/audit-backend:$(TAG)

push-frontend:
	podman push $(IMAGE_REGISTRY)/audit-frontend:$(TAG)

push-all: push-collector push-backend push-frontend

# ---- Deploy ----

deploy:
	oc apply -f deploy/namespace.yaml
	oc apply -f deploy/rbac/
	oc apply -f deploy/postgres/postgres.yaml
	@echo "Waiting for PostgreSQL to be ready..."
	oc -n audit-system wait --for=condition=ready pod -l app=audit-postgres --timeout=120s
	oc apply -f deploy/backend/access-config.yaml
	sed 's|IMAGE_REGISTRY_PLACEHOLDER|$(IMAGE_REGISTRY)|g' deploy/backend/backend.yaml | oc apply -f -
	sed 's|IMAGE_REGISTRY_PLACEHOLDER|$(IMAGE_REGISTRY)|g' deploy/collector/collector.yaml | oc apply -f -
	sed 's|IMAGE_REGISTRY_PLACEHOLDER|$(IMAGE_REGISTRY)|g' deploy/frontend/frontend.yaml | oc apply -f -
	@echo "Deployment complete. Get the route:"
	@oc -n audit-system get route audit-frontend -o jsonpath='{.spec.host}' && echo

undeploy:
	oc delete -f deploy/frontend/frontend.yaml --ignore-not-found
	oc delete -f deploy/collector/collector.yaml --ignore-not-found
	oc delete -f deploy/backend/backend.yaml --ignore-not-found
	oc delete -f deploy/backend/access-config.yaml --ignore-not-found
	oc delete -f deploy/postgres/postgres.yaml --ignore-not-found
	oc delete -f deploy/rbac/ --ignore-not-found
	oc delete -f deploy/namespace.yaml --ignore-not-found

# ---- Local Dev ----

dev-backend:
	cd backend && go run .

dev-frontend:
	cd frontend && npm run dev
