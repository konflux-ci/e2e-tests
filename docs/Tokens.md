# Getting the required tokens

## How to get GitHub token

[Instructions for creating a Personal Access Token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)

Make sure to give the token the permissions listed in [Requirements](/README.md#requirements).

Copy the resulting token (should look similar to `ghp_Iq...`) and save it off somewhere as you'll be using it for the GITHUB_TOKEN environment variable whenever you want to run the e2e suite(e.g `export GITHUB_TOKEN=ghp_Iq...`).

## How to get Quay token

Go to your profile (in Quay click your username in the upper right, click Account Settings). In your profile look for CLI Password and click the Generate Encrypted Password link. Click on Kubernetes Secret in the left panel. Click on the link for View username-secret.yml. Copy the string listed after `.dockerconfigjson` (should look similar to `ewogI3...`). Save the string off somewhere as you'll be using it for the QUAY_TOKEN environment variable whenever you want to run the e2e suite(e.g `export QUAY_TOKEN=ewogI3...`).
