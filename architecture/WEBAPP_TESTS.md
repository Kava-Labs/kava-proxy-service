## WEBAPP TESTS

We decided to run webapp (https://github.com/Kava-Labs/webapp) e2e tests on every push to main to have more confidence before deploying to public-testnet and mainnet.

Webapp e2e tests help to catch bugs, especially bugs that happens only in browser environemnt: related to CORS, etc...

Webapp e2e tests is triggered in this job: `Continuous Integration (Trigger Webapp E2E Tests)` https://github.com/Kava-Labs/kava-proxy-service/blob/main/.github/workflows/ci-webapp-e2e-tests.yml

We use https://github.com/convictional/trigger-workflow-and-wait `github actions plugin` to facilitate triggering `webapp github actions job` from our github actions setup.

Plugin requires using of `GITHUB_PERSONAL_ACCESS_TOKEN`, we used devops account for this purposes. We created `Personal access tokens (classic)` on devops account with such permissions:
<img width="771" alt="image" src="https://github.com/Kava-Labs/kava-proxy-service/assets/37836031/93e7388c-3e00-4a49-8332-dbdf747c0c3b">

Token name: `trigger-workflow-and-wait-token`

## Job execution order

`Continuous Integration (Main Branch)` -> `Continuous Deployment (Internal Testnet)` -> `Continuous Integration (Trigger Webapp E2E Tests)`

In another words:

`backend e2e-tests` -> `Deploy to Internal Testnet` -> `Trigger Webapp E2E Tests`

## Accessing Dev Ops Account

Email: devops@kava.io

Credentials can be found in 1Password, look for: `Credentials for Github DevOps/Service Account`
