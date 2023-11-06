/*
Copyright paskal.maksim@gmail.com
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func main() {
	if err := updateReadme(); err != nil {
		panic(err)
	}
}

type ReadmeExample struct {
	raw string
}

func (e *ReadmeExample) GetHeader() string {
	return strings.Split(e.raw, "\n")[0]
}

func (e *ReadmeExample) GetFormattedHeader() string {
	return strings.TrimPrefix(e.GetHeader(), "# ")
}

func (e *ReadmeExample) GetContent() string {
	return strings.TrimPrefix(e.raw, e.GetHeader())
}

func updateReadme() error {
	files, err := filepath.Glob("./e2e/configs/*.yaml")
	if err != nil {
		return errors.Wrap(err, "filepath.Glob")
	}

	b := strings.Builder{}

	for _, file := range files {
		if strings.HasSuffix(file, "full.yaml") {
			continue
		}

		fileContent, err := os.ReadFile(file)
		if err != nil {
			return errors.Wrap(err, "os.ReadFile")
		}

		article := &ReadmeExample{raw: string(fileContent)}

		b.WriteString("<details>")
		b.WriteString("<summary>")
		b.WriteString(article.GetFormattedHeader())
		b.WriteString("</summary>\n")
		b.WriteString("\n```yaml")
		b.WriteString(article.GetContent())
		b.WriteString("\n```\n")
		b.WriteString("</details>\n")
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		return errors.Wrap(err, "os.ReadFile")
	}

	readmeContent := string(readme)

	const (
		startMarker = "<!--- move_e2e_details_start -->"
		endMarker   = "<!--- move_e2e_details_end -->"
	)

	// remove old content
	sPosition := strings.Index(readmeContent, startMarker)
	ePosition := strings.Index(readmeContent, endMarker)

	readmeContent = readmeContent[0:sPosition] + readmeContent[ePosition:]
	readmeContent = strings.ReplaceAll(readmeContent, endMarker, startMarker+"\n"+b.String()+"\n"+endMarker)

	if err := os.WriteFile("README.md", []byte(readmeContent), 0o644); err != nil { //nolint:gomnd,gosec
		return errors.Wrap(err, "os.WriteFile")
	}

	return nil
}
