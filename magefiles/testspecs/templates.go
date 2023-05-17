package testspecs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig"
	"github.com/magefile/mage/sh"
	"k8s.io/klog/v2"
)

var templates = map[string]string{
	"test-file":          "templates/test_output_spec.tmpl",
	"framework-describe": "templates/framework_describe_func.tmpl",
}

func NewTemplateData(specOutline TestOutline, destination string) *TemplateData {

	re := regexp.MustCompile(`[A-Z][^A-Z]*`)
	parts := re.FindAllString(specOutline[0].Name, -1)

	for i, word := range parts {
		parts[i] = strings.ToLower(word)
	}
	newSpecName := strings.Join(parts[:len(parts)-1], "-")

	dir := filepath.Dir(destination)
	dirName := strings.Split(dir, "/")[len(strings.Split(dir, "/"))-1]

	return &TemplateData{Outline: specOutline, PackageName: dirName, FrameworkDescribeString: newSpecName}
}

func GetTemplate(name string) (string, error) {
	if s, ok := templates[name]; ok {
		return s, nil
	}
	return "", fmt.Errorf("no Template found for %q", name)
}

func RenderFrameworkDescribeGoFile(t TemplateData) error {

	templatePath, err := GetTemplate("framework-describe")
	if err != nil {
		return err
	}
	var describeFile = "pkg/framework/describe.go"

	err = renderTemplate(describeFile, templatePath, t, true)
	if err != nil {
		klog.Errorf("failed to append to pkg/framework/describe.go with : %s", err)
		return err
	}
	err = goFmt(describeFile)

	if err != nil {

		klog.Errorf("%s", err)
		return err
	}

	return nil

}

func goFmt(path string) error {
	err := sh.RunV("go", "fmt", path)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Could not fmt:\n%s\n", path), err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func renderTemplate(destination, templatePath string, templateData interface{}, appendDestination bool) error {

	var templateText string
	var f *os.File
	var err error

	/* This decision logic feels a little clunky cause initially I wanted to
	to have this func create the new file and render the template into the new
	file. But with the updating the pkg/framework/describe.go use case
	I wanted to reuse leveraging the txt/template package rather than
	rendering/updating using strings/regex.
	*/
	if appendDestination {

		f, err = os.OpenFile(destination, os.O_APPEND|os.O_WRONLY, 0664)
		if err != nil {
			klog.Infof("Failed to open file: %v", err)
			return err
		}
	} else {

		if fileExists(destination) {
			return fmt.Errorf("%s already exists", destination)
		}
		f, err = os.Create(destination)
		if err != nil {
			klog.Infof("Failed to create file: %v", err)
			return err
		}
	}

	defer f.Close()

	tpl, err := os.ReadFile(templatePath)
	if err != nil {
		klog.Infof("error reading file: %v", err)
		return err

	}
	var tmplText = string(tpl)
	templateText = fmt.Sprintf("\n%s", tmplText)
	specTemplate, err := template.New("spec").Funcs(sprig.TxtFuncMap()).Parse(templateText)
	if err != nil {
		klog.Infof("error parsing template file: %v", err)
		return err

	}

	err = specTemplate.Execute(f, templateData)
	if err != nil {
		klog.Infof("error rendering template file: %v", err)
		return err
	}

	return nil
}
