package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	TestsPath = "tests"
)

func main() {
	strict, err := strconv.ParseBool(env("KUBECONFORM_STRICT"))

	if err != nil {
		log.Fatal("KUBECONFORM_STRICT must be `true` or `false`")
	}

	rawKubernetesSchemaPath, err := filepath.Abs(env("KUBERNETES_SCHEMA_PATH"))

	if err != nil {
		log.Fatal("KUBERNETES_SCHEMA_PATH must be a valid path")
	}

	kubernetesSchemaPath := filepath.Join(rawKubernetesSchemaPath, "{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}", "{{ .ResourceKind }}{{ .KindSuffix }}.json")

	additionalSchemaPaths, err := parseLocations(env("ADDITIONAL_SCHEMA_PATHS"))

	if err != nil {
		log.Fatalf("ADDITIONAL_SCHEMA_PATHS must be a valid list of paths: %s", err)
	}

	chartsDirectory, err := filepath.Abs(env("HELM_CHARTS_DIRECTORY"))

	if err != nil {
		log.Fatal("HELM_CHARTS_DIRECTORY must be a valid path")
	}

	kubeconformPath, err := filepath.Abs(env("KUBECONFORM_PATH"))

	if err != nil {
		log.Fatal("KUBECONFORM_PATH must be a valid path")
	}

	outputFormat := env("KUBECONFORM_OUTPUT_FORMAT")

	helmPath, err := filepath.Abs(env("HELM_PATH"))

	if err != nil {
		log.Fatalf("HELM_PATH must be a valid path")
	}

	// to use kubeconform as a library would need us to practically
	// reimplement its CLI
	// <https://github.com/yannh/kubeconform/blob/dcc77ac3a39ed1fb538b54fab57bbe87d1ece490/cmd/kubeconform/main.go#L47>,
	// so instead we shell out to it

	feErr := foreachChart(chartsDirectory, func(base string) error {
		valuesFiles, err := os.ReadDir(filepath.Join(base, TestsPath))

		if err != nil {
			return err
		}

		for _, file := range valuesFiles {
			log.Printf("Validating chart %s with values file %s...\n", filepath.Base(base), file.Name())
			manifests, err := runHelm(helmPath, base, file.Name())

			if err != nil {
				fmt.Printf("Could not run Helm: %s\nStdout: %s\n", err, manifests.String())
				return err
			}

			output, err := runKubeconform(manifests, kubeconformPath, kubernetesSchemaPath, strict, outputFormat, additionalSchemaPaths)

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

func parseLocations(s string) ([]string, error) {
	raw := lines(s)

	parsed := []string{}

	for _, line := range raw {
		p, err := filepath.Abs(line)

		if err != nil {
			return parsed, err
		}

		parsed = append(parsed, p)
	}

	return parsed, nil
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
		"-verbose",
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

func env(s string) string {
	return os.Getenv(s)
}

func lines(s string) []string {
	return strings.Split(s, "\n")
}
