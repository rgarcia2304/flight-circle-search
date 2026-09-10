GCP_PROJECT   := circle-search-508212
GCP_REGION    := us-central1
SQL_INSTANCE  := main-db
WORKER_SVC    := flight-worker
API_SVC       := flight-api

FRONTEND_URL  := https://circle-search-508212.web.app
API_URL       := https://flight-api-1048739205149.us-central1.run.app

.PHONY: demo-up demo-down demo-status

## Bring the deployed demo stack back up: resume Cloud SQL, scale the worker
## back to 1 always-on instance. flight-api needs no action — it already
## scales to zero on its own and cold-starts on the next request.
demo-up:
	@echo "Resuming Cloud SQL instance $(SQL_INSTANCE)..."
	gcloud sql instances patch $(SQL_INSTANCE) --project=$(GCP_PROJECT) --activation-policy=ALWAYS --quiet
	@echo "Waiting for Cloud SQL to become RUNNABLE..."
	@until [ "$$(gcloud sql instances describe $(SQL_INSTANCE) --project=$(GCP_PROJECT) --format='value(state)')" = "RUNNABLE" ]; do \
		echo "  still starting..."; sleep 5; \
	done
	@echo "Cloud SQL is up."
	@echo "Scaling $(WORKER_SVC) to 1 instance..."
	gcloud run services update $(WORKER_SVC) --project=$(GCP_PROJECT) --region=$(GCP_REGION) --min-instances=1 --quiet
	@echo ""
	@echo "Demo stack is up:"
	@echo "  Frontend: $(FRONTEND_URL)"
	@echo "  API:      $(API_URL)"

## Take the demo stack back down between sessions: scale the worker to 0
## (stops the always-on cost), stop Cloud SQL (keeps all data, stops compute
## billing). Memorystore + the VPC connector are left running — they don't
## support stop/start, only delete/recreate, and their idle cost is small
## enough that the operational overhead of tearing them down isn't worth it.
demo-down:
	@echo "Scaling $(WORKER_SVC) to 0 instances..."
	gcloud run services update $(WORKER_SVC) --project=$(GCP_PROJECT) --region=$(GCP_REGION) --min-instances=0 --quiet
	@echo "Stopping Cloud SQL instance $(SQL_INSTANCE)..."
	gcloud sql instances patch $(SQL_INSTANCE) --project=$(GCP_PROJECT) --activation-policy=NEVER --quiet
	@echo ""
	@echo "Demo stack is down. Memorystore + VPC connector still running (cheap, intentionally left up)."

## Show whether the demo stack is currently up or down.
demo-status:
	@echo "Cloud SQL:"
	@gcloud sql instances describe $(SQL_INSTANCE) --project=$(GCP_PROJECT) --format='value(state,settings.activationPolicy)'
	@echo "Worker min-instances:"
	@gcloud run services describe $(WORKER_SVC) --project=$(GCP_PROJECT) --region=$(GCP_REGION) --format='value(spec.template.metadata.annotations["autoscaling.knative.dev/minScale"])'
