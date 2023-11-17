## WEBAPP TESTS

We decided to run webapp (https://github.com/Kava-Labs/webapp) e2e tests on every push to main to have more confidence before deploying to public-testnet and mainnet.

Webapp e2e tests help to catch bugs, especially bugs that happens only in browser environemnt: related to CORS, etc...

Webapp e2e tests is triggered in this job: `Continuous Integration (Trigger Webapp E2E Tests)` https://github.com/Kava-Labs/kava-proxy-service/blob/main/.github/workflows/ci-webapp-e2e-tests.yml

We use https://github.com/convictional/trigger-workflow-and-wait github actions plugin to facilitate triggering webapp github actions job from our github actions setup.

## Accessing Dev Ops Account

TODO...
