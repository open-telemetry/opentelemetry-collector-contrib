// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestHTMLStrip(t *testing.T) {
	type testCase struct {
		name          string
		value         any
		want          any
		expectedError string
	}
	var testCases = []testCase{
		// Comment
		{name: "HTML Comment", value: "<!--This is a comment-->", want: "", expectedError: ""},

		// Document Type
		{name: "DOCTYPE Tag", value: "<!DOCTYPE html>", want: "", expectedError: ""},

		// Common Tags
		{name: "Anchor Tag", value: "<a href='https://example.com'>Link</a>", want: "Link", expectedError: ""},
		{name: "Abbreviation Tag", value: "<abbr title='World Health Organization'>WHO</abbr>", want: "WHO", expectedError: ""},
		{name: "Acronym Tag", value: "<acronym>HTML4</acronym>", want: "HTML4", expectedError: ""},
		{name: "Address Tag", value: "<address>123 Main St</address>", want: "123 Main St", expectedError: ""},
		{name: "Article Tag", value: "<article>News content</article>", want: "News content", expectedError: ""},
		{name: "Aside Tag", value: "<aside>Sidebar content</aside>", want: "Sidebar content", expectedError: ""},

		// Text Formatting Tags
		{name: "Bold Tag", value: "<b>Bold Text</b>", want: "Bold Text", expectedError: ""},
		{name: "Italic Tag", value: "<i>Italicized Text</i>", want: "Italicized Text", expectedError: ""},
		{name: "Strong Tag", value: "<strong>Strong Text</strong>", want: "Strong Text", expectedError: ""},
		{name: "Emphasized Tag", value: "<em>Emphasized Text</em>", want: "Emphasized Text", expectedError: ""},
		{name: "Small Text Tag", value: "<small>Small Text</small>", want: "Small Text", expectedError: ""},
		{name: "Deleted Text Tag", value: "<del>Deleted Text</del>", want: "Deleted Text", expectedError: ""},
		{name: "Inserted Text Tag", value: "<ins>Inserted Text</ins>", want: "Inserted Text", expectedError: ""},
		{name: "Strikethrough Tag", value: "<s>Strikethrough Text</s>", want: "Strikethrough Text", expectedError: ""},

		// Lists
		{name: "Ordered List", value: "<ol><li>Item 1</li><li>Item 2</li></ol>", want: "Item 1Item 2", expectedError: ""},
		{name: "Unordered List", value: "<ul><li>Item 1</li><li>Item 2</li></ul>", want: "Item 1Item 2", expectedError: ""},
		{name: "Description List", value: "<dl><dt>Term</dt><dd>Definition</dd></dl>", want: "TermDefinition", expectedError: ""},

		// Tables
		{name: "Table", value: "<table><tr><td>Cell</td></tr></table>", want: "Cell", expectedError: ""},
		{name: "Table Header", value: "<th>Header</th>", want: "Header", expectedError: ""},
		{name: "Table Row", value: "<tr>Row</tr>", want: "Row", expectedError: ""},

		// Forms
		{name: "Form Tag", value: "<form><input type='text' value='Input'></form>", want: "", expectedError: ""},
		{name: "Button Tag", value: "<button>Click Me</button>", want: "Click Me", expectedError: ""},
		{name: "Textarea Tag", value: "<textarea>Text Area Content</textarea>", want: "Text Area Content", expectedError: ""},
		{name: "Select and Option Tags", value: "<select><option>Option 1</option><option>Option 2</option></select>", want: "Option 1Option 2", expectedError: ""},

		// Media and Graphics
		{name: "Image Tag", value: "<img src='image.jpg' alt='An image'>", want: "", expectedError: ""},
		{name: "Audio Tag", value: "<audio controls><source src='audio.mp3'></audio>", want: "", expectedError: ""},
		{name: "Video Tag", value: "<video controls><source src='video.mp4'></video>", want: "", expectedError: ""},
		{name: "Canvas Tag", value: "<canvas>Graphics</canvas>", want: "Graphics", expectedError: ""},

		// Miscellaneous
		{name: "Div Tag", value: "<div>Division Content</div>", want: "Division Content", expectedError: ""},
		{name: "Span Tag", value: "<span>Span Content</span>", want: "Span Content", expectedError: ""},
		{name: "Break Line Tag", value: "Line 1<br>Line 2", want: "Line 1Line 2", expectedError: ""},
		{name: "Header Tag", value: "<h1>Header Content</h1>", want: "Header Content", expectedError: ""},
		{name: "Script Tag", value: "<script>console.log('Hello')</script>", want: "", expectedError: ""},
		{name: "Style Tag", value: "<style>body {color: red;}</style>", want: "", expectedError: ""},

		// Special Cases
		{name: "Empty Input", value: "", want: "", expectedError: ""},
		{name: "Plain Text", value: "This is plain text.", want: "This is plain text.", expectedError: ""},
		{name: "Nested Tags", value: "<div><p>Nested <strong>text</strong></p></div>", want: "Nested text", expectedError: ""},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createHTMLStripFunction[any](ottl.FunctionContext{}, &HTMLStripArguments[any]{
				HTMLSource: &ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return tt.value, nil
					},
				},
			})

			require.NoError(t, err)

			result, err := expressionFunc(nil, nil)
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}
