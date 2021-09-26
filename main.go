package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"github.com/joho/godotenv"
	"github.com/caarlos0/env/v6"
)

const (
	TestsPath = "tests"
)

type Path struct {
	path string
}

type Config struct {
	Strict                bool   `env:"KUBECONFORM_STRICT" envDefault:"true"`
	KubernetesSchemaPath  Path   `env:"KUBERNETES_SCHEMA_PATH"`
	AdditionalSchemaPaths []Path `env:"ADDITIONAL_SCHEMA_PATHS" envSeparator:"\n"`
	ChartsDirectory       Path   `env:"CHARTS_DIRECTORY"`
	Kubeconform           Path   `env:"KUBECONFORM"`
	Helm                  Path   `env:"HELM"`
	OutputFormat          string `env:"OUTPUT_FORMAT"`
}

func main() {
	godotenv.Load()

	cfg := Config{}

	if err := env.ParseWithFuncs(&cfg, map[reflect.Type]env.ParserFunc{
		reflect.TypeOf(Path{}): parsePath,
	}); err != nil {
		log.Fatalf("%+v\n", err)
	}

	kubernetesSchemaPath := filepath.Join(cfg.KubernetesSchemaPath.path, "{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}", "{{ .ResourceKind }}{{ .KindSuffix }}.json")

	// to use kubeconform as a library would need us to practically
	// reimplement its CLI
	// <https://github.com/yannh/kubeconform/blob/dcc77ac3a39ed1fb538b54fab57bbe87d1ece490/cmd/kubeconform/main.go#L47>,
	// so instead we shell out to it

	additionalSchemaPaths := []string{}

	for _, path := range cfg.AdditionalSchemaPaths {
		additionalSchemaPaths = append(additionalSchemaPaths, path.path)
	}

	feErr := foreachChart(cfg.ChartsDirectory.path, func(base string) error {
		valuesFiles, err := os.ReadDir(filepath.Join(base, TestsPath))

		if err != nil {
			return err
		}

		for _, file := range valuesFiles {
			log.Printf("Validating chart %s with values file %s...\n", filepath.Base(base), file.Name())
			manifests, err := runHelm(cfg.Helm.path, base, file.Name())

			if err != nil {
				fmt.Printf("Could not run Helm: %s\nStdout: %s\n", err, manifests.String())
				return err
			}

			output, err := runKubeconform(manifests, cfg.Kubeconform.path, kubernetesSchemaPath, cfg.Strict, cfg.OutputFormat, additionalSchemaPaths)

			fmt.Println(output)

			if err != nil {
				return err
			}
		}

		return nil
	})

	if feErr != nil {
		log.Fatalf("Validation failed: %s", feErr)
	}
}

func foreachChart(path string, fn func(path string) error) error {
	files, err := os.ReadDir(path)

	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			return fmt.Errorf("Non-directory file in charts directory: %s", file.Name())
		}

		p := filepath.Join(path, file.Name())

		if err := fn(p); err != nil {
			return err
		}
	}

	return nil
}

func runHelm(path string, directory string, valuesFile string) (bytes.Buffer, error) {
	cmd := helmCommand(path, directory, filepath.Join(directory, TestsPath, valuesFile))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		log.Printf("Failed to run Helm: %s\n", stderr.String())
		return stdout, err
	}

	return stdout, nil
}

func helmCommand(path string, directory string, valuesFile string) *exec.Cmd {
	return exec.Command(path, "template", "release", directory, "-f", valuesFile)
}

func runKubeconform(manifests bytes.Buffer, path string, kubernetesSchemaPath string, strict bool, outputFormat string, additionalSchemaPaths []string) (string, error) {
	cmd := kubeconformCommand(path, kubernetesSchemaPath, strict, outputFormat, additionalSchemaPaths)

	stdin, err := cmd.StdinPipe()

	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		stdin.Write(manifests.Bytes())
	}()

	output, err := cmd.CombinedOutput()

	// whatever the output is, we want to display it, and we want to return the error if there is one
	if err != nil {
		log.Printf("Failed to run kubeconform command %s: %s\n", cmd, string(output[:]))
		return "", err
	}

	return string(output[:]), err
}

func kubeconformCommand(path string, kubernetesSchemaPath string, strict bool, outputFormat string, additionalSchemaPaths []string) *exec.Cmd {
	return exec.Command(path, kubeconformArgs(kubernetesSchemaPath, strict, outputFormat, additionalSchemaPaths)...)
}

func kubeconformArgs(kubernetesSchemaPath string, strict bool, outputFormat string, additionalSchemaPaths []string) []string {
	args := []string{
		"-schema-location",
		kubernetesSchemaPath,
		"-summary",
	}

	if strict {
		args = append(args, "-strict")
	}

	if outputFormat != "" {
		args = append(args, "-output")
		args = append(args, outputFormat)
	}

	for _, location := range additionalSchemaPaths {
		args = append(args, "-schema-location")
		args = append(args, location)
	}

	return args
}

func parsePath(v string) (interface{}, error) {
	if v == "" {
		return nil, errors.New("No path specified")
	}

	parsed, err := filepath.Abs(v)

	if err != nil {
		return Path{}, err
	}

	return Path{path: parsed}, nil
}
