# enable services
gcloud services enable appengine.googleapis.com --project $PROJECT_ID
gcloud services enable firestore.googleapis.com --project $PROJECT_ID
gcloud services enable cloudbuild.googleapis.com --project $PROJECT_ID
gcloud services enable run.googleapis.com --project $PROJECT_ID

# create artifact artifactregistry
gcloud artifacts repositories create cloud-run-source-deploy --repository-format=docker \
--location="$REGION" --description="Docker registry" --project $PROJECT_ID

# get compute user
PROJECTNUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
# assign needed roles to build and do stuff
gcloud projects add-iam-policy-binding $PROJECT_ID --member="serviceAccount:$PROJECTNUMBER-compute@developer.gserviceaccount.com" --role='roles/storage.objectUser'
gcloud projects add-iam-policy-binding $PROJECT_ID --member="serviceAccount:$PROJECTNUMBER-compute@developer.gserviceaccount.com" --role='roles/artifactregistry.writer'
gcloud projects add-iam-policy-binding $PROJECT_ID --member="serviceAccount:$PROJECTNUMBER-compute@developer.gserviceaccount.com" --role='roles/logging.logWriter'
