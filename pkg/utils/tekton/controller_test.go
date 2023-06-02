package tekton

import (
	"fmt"
	"testing"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func TestCosignResultShouldPresence(t *testing.T) {
	assert.False(t, CosignResult{}.IsPresent())

	assert.False(t, CosignResult{
		signatureImageRef: "something",
	}.IsPresent())

	assert.False(t, CosignResult{
		attestationImageRef: "something",
	}.IsPresent())

	assert.True(t, CosignResult{
		signatureImageRef:   "something",
		attestationImageRef: "something",
	}.IsPresent())
}

func TestCosignResultMissingFormat(t *testing.T) {
	assert.Equal(t, "prefix.sig and prefix.att", CosignResult{}.Missing("prefix"))

	assert.Equal(t, "prefix.att", CosignResult{
		signatureImageRef: "something",
	}.Missing("prefix"))

	assert.Equal(t, "prefix.sig", CosignResult{
		attestationImageRef: "something",
	}.Missing("prefix"))

	assert.Empty(t, CosignResult{
		signatureImageRef:   "something",
		attestationImageRef: "something",
	}.Missing("prefix"))
}

func createHttpMock(urlPath, tag string, response any) {
	s := gock.New("https://quay.io/api/v1")
	if len(tag) > 0 {
		s.MatchParam("specificTag", tag)
	}
	s.Get(urlPath).
		Reply(200).
		JSON(response)
}

func TestFindingCosignResults(t *testing.T) {
	const imageRegistryName = "quay.io"
	const imageRepo = "test/repo"
	const imageTag = "123"
	const imageDigest = "sha256:abc"
	const cosignImageTag = "sha256-abc"
	const imageRef = imageRegistryName + "/" + imageRepo + ":" + imageTag + "@" + imageDigest
	const signatureImageDigest = "sha256:signature"
	const attestationImageDigest = "sha256:attestation"
	const signatureImageRef = imageRegistryName + "/" + imageRepo + "@" + signatureImageDigest
	const attestationImageRef = imageRegistryName + "/" + imageRepo + "@" + attestationImageDigest

	cases := []struct {
		Name                    string
		SignatureImagePresent   bool
		AttestationImagePresent bool
		AttestationImageLayers  []any
		ExpectedErrors          []string
		Result                  *CosignResult
	}{
		{"happy day", true, true, []any{"", ""}, []string{}, &CosignResult{
			signatureImageRef:   signatureImageRef,
			attestationImageRef: attestationImageRef,
		}},
		{"missing signature", false, true, []any{"", ""}, []string{"error when getting signature"}, &CosignResult{
			signatureImageRef:   "",
			attestationImageRef: attestationImageRef,
		}},
		{"missing attestation", true, false, []any{"", ""}, []string{"error when getting attestation"}, &CosignResult{
			signatureImageRef:   signatureImageRef,
			attestationImageRef: "",
		}},
		{"missing signature and attestation", false, false, []any{"", ""}, []string{"error when getting attestation", "error when getting signature"}, &CosignResult{
			signatureImageRef:   "",
			attestationImageRef: "",
		}},
		{"missing layer in attestation", true, true, []any{""}, []string{"attestation tag doesn't have the expected number of layers"}, &CosignResult{
			signatureImageRef:   signatureImageRef,
			attestationImageRef: attestationImageRef,
		}},
	}

	for _, cse := range cases {
		t.Run(cse.Name, func(t *testing.T) {
			defer gock.Off()

			if cse.SignatureImagePresent {
				createHttpMock(fmt.Sprintf("/repository/%s/tag", imageRepo), cosignImageTag+".sig", &TagResponse{Tags: []Tag{{Digest: signatureImageDigest}}})
			}
			if cse.AttestationImagePresent {
				createHttpMock(fmt.Sprintf("/repository/%s/tag", imageRepo), cosignImageTag+".att", &TagResponse{Tags: []Tag{{Digest: attestationImageDigest}}})
			}
			createHttpMock(fmt.Sprintf("/repository/%s/manifest/%s", imageRepo, attestationImageDigest), "", &ManifestResponse{Layers: cse.AttestationImageLayers})

			result, err := findCosignResultsForImage(imageRef)

			if err != nil {
				assert.NotEmpty(t, cse.ExpectedErrors)
				for _, errSubstring := range cse.ExpectedErrors {
					assert.Contains(t, err.Error(), errSubstring)
				}
			} else {
				assert.Empty(t, cse.ExpectedErrors)
			}
			assert.Equal(t, cse.Result, result)
		})
	}

}
