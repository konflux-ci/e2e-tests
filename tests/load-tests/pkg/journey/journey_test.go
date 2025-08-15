package journey

import "testing"

// Test basic input and output combinations for getRepoNameFromRepoUrl.
func Test_getRepoNameFromRepoUrl(t *testing.T) {
	repoName := "nodejs-devfile-sample"
	repoUrls := []string{
		"https://github.com/abc/nodejs-devfile-sample.git/",
		"https://github.com/abc/nodejs-devfile-sample.git",
		"https://github.com/abc/nodejs-devfile-sample/",
		"https://github.com/abc/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample.git",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample.git/",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample/",
	}
	for _, repoUrl := range repoUrls {
		out, err := getRepoNameFromRepoUrl(repoUrl)
		if err != nil || out != repoName {
			t.Errorf("Failed getting '%s' from '%s': %v", repoName, repoUrl, err)
		}
	}
}

// Test basic input and output combinations for getRepoOrgFromRepoUrl.
func Test_getRepoOrgFromRepoUrl(t *testing.T) {
	repoName := "abc"
	repoUrls := []string{
		"https://github.com/abc/nodejs-devfile-sample.git/",
		"https://github.com/abc/nodejs-devfile-sample.git",
		"https://github.com/abc/nodejs-devfile-sample/",
		"https://github.com/abc/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/nodejs-devfile-sample.git",
		"https://gitlab.example.com/abc/nodejs-devfile-sample.git/",
		"https://gitlab.example.com/abc/nodejs-devfile-sample/",
	}
	for _, repoUrl := range repoUrls {
		out, err := getRepoOrgFromRepoUrl(repoUrl)
		if err != nil || out != repoName {
			t.Errorf("Failed getting '%s' from '%s': %v", repoName, repoUrl, err)
		}
	}

	repoName = "abc/def"
	repoUrls = []string{
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample.git",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample.git/",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample/",
	}
	for _, repoUrl := range repoUrls {
		out, err := getRepoOrgFromRepoUrl(repoUrl)
		if err != nil || out != repoName {
			t.Errorf("Failed getting '%s' from '%s': %v", repoName, repoUrl, err)
		}
	}
}

// Test various input and output combinations for getRepoIdFromRepoUrl.
func Test_getRepoIdFromRepoUrl(t *testing.T) {
	repoName := "abc/nodejs-devfile-sample"
	repoUrls := []string{
		"https://github.com/abc/nodejs-devfile-sample.git/",
		"https://github.com/abc/nodejs-devfile-sample.git",
		"https://github.com/abc/nodejs-devfile-sample/",
		"https://github.com/abc/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/nodejs-devfile-sample",
	}
	for _, repoUrl := range repoUrls {
		out, err := getRepoIdFromRepoUrl(repoUrl)
		if err != nil || out != repoName {
			t.Errorf("Failed getting '%s' from '%s': %v", repoName, repoUrl, err)
		}
	}

	repoName = "abc/def/nodejs-devfile-sample"
	repoUrls = []string{
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample.git",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample.git/",
		"https://gitlab.example.com/abc/def/nodejs-devfile-sample/",
	}
	for _, repoUrl := range repoUrls {
		out, err := getRepoIdFromRepoUrl(repoUrl)
		if err != nil || out != repoName {
			t.Errorf("Failed getting '%s' from '%s': %v", repoName, repoUrl, err)
		}
	}
}
