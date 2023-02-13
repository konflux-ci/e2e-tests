package tekton

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func newTag(name string, hash string, noLayers int) client.Object {
	tag := unstructured.Unstructured{}
	tag.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Kind:    "ImageStreamTag",
		Version: "v1",
	})
	tag.SetNamespace("test-namespace")
	tag.SetName(name)
	layers := make([]interface{}, noLayers)
	tag.Object["image"] = map[string]interface{}{
		"dockerImageLayers": layers,
	}

	if hash != "" {
		if err := unstructured.SetNestedField(tag.Object, hash, "image", "metadata", "name"); err != nil {
			panic(err)
		}
	}

	return &tag
}

func TestFindingCosignResults(t *testing.T) {
	cases := []struct {
		Name          string
		Tags          []client.Object
		ExpectedError string
		Result        *CosignResult
	}{
		{"happy day", []client.Object{
			newTag("test-image:latest", "sha256:hash", 1),
			newTag("test-image:sha256-hash.sig", "", 1),
			newTag("test-image:sha256-hash.att", "", 2),
		}, "", &CosignResult{
			signatureImageRef:   "test-image:sha256-hash.sig",
			attestationImageRef: "test-image:sha256-hash.att",
		}},
		{"missing signature", []client.Object{
			newTag("test-image:latest", "sha256:hash", 1),
			newTag("test-image:sha256-hash.att", "", 2),
		}, "ImageStreamTag.image.openshift.io \"test-image:sha256-hash.sig\" not found", nil},
		{"missing attestation", []client.Object{
			newTag("test-image:latest", "sha256:hash", 1),
			newTag("test-image:sha256-hash.sig", "", 1),
		}, "ImageStreamTag.image.openshift.io \"test-image:sha256-hash.att\" not found", nil},
		{"missing signature and attestation", []client.Object{
			newTag("test-image:latest", "sha256:hash", 1),
		}, "ImageStreamTag.image.openshift.io \"test-image:sha256-hash.sig and test-image:sha256-hash.att\" not found", nil},
		{"everything missing", []client.Object{}, "ImageStreamTag.image.openshift.io \"test-image:sha256-hash.sig and test-image:sha256-hash.att\" not found", nil},
		{"missing layer in attestation", []client.Object{
			newTag("test-image:latest", "sha256:hash", 1),
			newTag("test-image:sha256-hash.sig", "", 1),
			newTag("test-image:sha256-hash.att", "", 1),
		}, "ImageStreamTag.image.openshift.io \"test-image:sha256-hash.att\" not found", nil},
	}

	for _, cse := range cases {
		t.Run(cse.Name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithObjects(cse.Tags...).Build()

			result, err := findCosignResultsForImage("image-registry.openshift-image-registry.svc:5000/test-namespace/test-image@sha256-hash", client)

			if err != nil || cse.ExpectedError != "" {
				assert.EqualError(t, err, cse.ExpectedError)
				assert.True(t, errors.IsNotFound(err))
			}

			assert.Equal(t, cse.Result, result)
		})
	}

}
