# calendar-ui-events-service

CRUD service lambda for calendar-ui

### Deploy manually

-   `make deploy`

### Run locally

-   `make build && sam local start-api --port 8000`

### Setup GitHub actions

Once the repo is setup on GitHub, add AWS secrets to GitHub Actions for this repo:

-   `gh secret set AWS_ACCESS_KEY_ID`
-   `gh secret set AWS_SECRET_ACCESS_KEY`

### Test

-   `curl -X POST http://localhost:8000/events -H "Content-Type: application/json" -d @post-data.json |jq .`
-   `curl http://localhost:8000/events |jq .`
