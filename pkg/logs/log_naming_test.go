package logs

import (
	"github.com/onsi/ginkgo/v2/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetClassnameFromReport(t *testing.T) {
	type testCase struct {
		description string
		input       types.SpecReport
		expected    string
	}

	for _, tc := range []testCase{
		{
			description: "Empty report",
			input:       types.SpecReport{},
			expected:    "",
		},
		{
			description: "Empty LeafNodeText",
			input: types.SpecReport{
				ContainerHierarchyTexts: []string{"[foo-suite Foo Suite]", "first", "second", "final"},
			},
			expected: "foo-suite",
		},
		{
			description: "Populated ContainerHierarchyTexts and LeafNodeText",
			input: types.SpecReport{
				ContainerHierarchyTexts: []string{"[foo-suite Foo Suite]", "first", "second", "final"},
				LeafNodeText:            "foo",
			},
			expected: "foo-suite",
		},
		{
			description: "Empty texts, populated LeafNodeText",
			input: types.SpecReport{
				ContainerHierarchyTexts: nil,
				LeafNodeText:            "foo",
			},
			expected: "foo",
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			assert.Equal(t, tc.expected, GetClassnameFromReport(tc.input))
		})
	}
}

func TestShortenStringAddHash(t *testing.T) {
	type testCase struct {
		description    string
		input          types.SpecReport
		result         string
		expectedLength int
	}

	for _, tc := range []testCase{
		{
			description:    "Empty report",
			input:          types.SpecReport{},
			result:         "",
			expectedLength: 0,
		},
		{
			description:    "One char report",
			input:          types.SpecReport{ContainerHierarchyTexts: []string{"f"}},
			result:         "f sha: ",
			expectedLength: len("f sha: ") + 64,
		},
		{
			description: "Short report",
			input: types.SpecReport{
				ContainerHierarchyTexts: []string{
					"[foo-suite Foo Suite]",
					"BEGIN Lorem ipsum END",
				},
			},
			result:         "[Foo Suite] BEGIN Lorem ipsum END sha: ",
			expectedLength: len("[Foo Suite] BEGIN Lorem ipsum END sha: ") + 64, // Expecting hash
		},
		{
			description: "Limit with exact limit length",
			input: types.SpecReport{
				ContainerHierarchyTexts: []string{
					"[foo-suite Foo Suite] BEGIN Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. MIDDLE Ut enim ad minim veniam, quis nostrud exercitation ullam CUT",
				},
			},
			result: "[Foo Suite] " +
				"BEGIN Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
				"MIDDLE Ut enim ad minim veniam, quis nostrud exercitation ullam CUT sha: ",
			expectedLength: 255,
		},
		{
			description: "Long report",
			input: types.SpecReport{
				ContainerHierarchyTexts: []string{
					"[foo-suite Foo Suite]",
					"BEGIN Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
					"MIDDLE Ut enim ad minim veniam, quis nostrud exercitation ullam CUT laboris nisi ut aliquip ex ea commodo consequat.",
					"END",
				},
			},
			result: "[Foo Suite] " +
				"BEGIN Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
				"MIDDLE Ut enim ad minim veniam, quis nostrud exercitation ullam CUT sha: ",
			expectedLength: 255,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			funcResult := ShortenStringAddHash(tc.input)

			assert.Contains(t, funcResult[:tc.expectedLength], funcResult) // Not checking hash
			assert.Len(t, funcResult, tc.expectedLength)
		})
	}
}
