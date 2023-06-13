# Requirements

To go through Github flow, the following environement variable are expected: 
- CYPRESS_GH_USER: the github username
- CYPRESS_GH_PASSWORD: the github account password 
- CYPRESS_SPI_OAUTH_URL: the oauth url provided by an SPI resource

To reproduce:

- navigate to cypress folder in this repo
- run npm install
- run the spec.cy.js spec:
  - either opern cypress ( $(npm bin)/cypress open ) and run the spec manually 
  - or run the spec headless from command-line ( $(npm bin)/cypress run --spec cypress/e2e/test.cy.js )