# first source 1.env.sh with your env variables PROJECT_ID and REGION and set cloudrun.yaml with your storage bucket.

SECONDS=0
gcloud builds submit --tag "$REGION-docker.pkg.dev/$PROJECT_ID/cloud-run-source-deploy/upstream-blog" --project $PROJECT_ID

sed -i "/              value: DEPLOY_DATE_/c\              value: DEPLOY_DATE_$(date +%d-%m-%Y_%H-%M-%S)" cloudrun.local.yaml
gcloud run services replace cloudrun.local.yaml --project $PROJECT_ID --region $REGION
echo | gcloud run services set-iam-policy upstream-blog cloudrun.policy.yaml --project $PROJECT_ID --region $REGION

duration=$SECONDS
echo "Total deployment finished in $((duration / 60)) minutes and $((duration % 60)) seconds."
