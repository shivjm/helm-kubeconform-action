package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
	_ "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

const (
	TestsPath = "tests"
)

type Path struct {
	path string
}

type Config struct {
	Strict                bool   `env:"KUBECONFORM_STRICT" envDefault:"true"`
	AdditionalSchemaPaths []Path `env:"ADDITIONAL_SCHEMA_PATHS" envSeparator:"\n"`
	ChartsDirectory       Path   `env:"CHARTS_DIRECTORY"`
	K8sVersion            String `env:"K8S_VERSION"`
	Kubeconform           Path   `env:"KUBECONFORM"`
	Helm                  Path   `env:"HELM"`
	UpdateDependencies    bool   `env:"HELM_UPDATE_DEPENDENCIES"`
}

func main() {
	godotenv.Load()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	cfg := Config{}

	if err := env.ParseWithFuncs(&cfg, map[reflect.Type]env.ParserFunc{
		reflect.TypeOf(Path{}): parsePath,
	}); err != nil {
		log.Fatal().Stack().Err(err).Msgf("%+v\n", err)
		return
	}

	additionalSchemaPaths := []string{}

	for _, path := range cfg.AdditionalSchemaPaths {
		additionalSchemaPaths = append(additionalSchemaPaths, path.path)
	}

	feErr := run(cfg, additionalSchemaPaths, cfg.UpdateDependencies)

	if feErr != nil {
		log.Fatal().Stack().Err(feErr).Msgf("Validation failed: %s", feErr)
		return
	}
}

func run(cfg Config, additionalSchemaPaths []string, updateDependencies bool) error {
	return foreachChart(cfg.ChartsDirectory.path, func(base string) error {
		logger := log.With().Str("chart", filepath.Base(base)).Logger()
		valuesFiles, err := os.ReadDir(filepath.Join(base, TestsPath))

		if err != nil {
			logger.Error().Stack().Err(err).Msgf("Could not open directory %s", base)
			return err
		}

		for _, file := range valuesFiles {
			name := file.Name()
			fileLogger := logger.With().Str("file", name).Logger()
			fileLogger.Printf("Validating chart %s with values file %s...\n", filepath.Base(base), name)
			manifests, err := runHelm(cfg.Helm.path, base, name, updateDependencies)

			if err != nil {
				fileLogger.Printf("Could not run Helm: %s\nStdout: %s\n", err, manifests.String())
				return err
			}

			// to use kubeconform as a library would need us to practically
			// reimplement its CLI
			// <https://github.com/yannh/kubeconform/blob/dcc77ac3a39ed1fb538b54fab57bbe87d1ece490/cmd/kubeconform/main.go#L47>,
			// so instead we shell out to it
			output, err := runKubeconform(manifests, cfg.Kubeconform.path, cfg.Strict, additionalSchemaPaths, cfg.K8sVersion)

			fileLogger.Info().Msgf("Output: %s", output)

			if err != nil {
				return err
			}
		}

		return nil
	})
}

func foreachChart(path string, fn func(path string) error) error {
	files, err := os.ReadDir(path)

	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			return errors.New(fmt.Sprintf("Non-directory file in charts directory: %s", file.Name()))
		}

		p := filepath.Join(path, file.Name())

		if err := fn(p); err != nil {
			return err
		}
	}

	return nil
}

func runHelm(path string, directory string, valuesFile string, updateDependencies bool) (bytes.Buffer, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if updateDependencies {
		if err := runHelmUpdateDependencies(path, directory); err != nil {
			return stdout, err
		}
	}

	cmd := helmTemplateCommand(path, directory, filepath.Join(directory, TestsPath, valuesFile))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		log.Printf("Failed to run Helm: %s\n", stderr.String())
		return stdout, err
	}

	return stdout, nil
}

func helmTemplateCommand(path string, directory string, valuesFile string) *exec.Cmd {
	return exec.Command(path, "template", "release", directory, "-f", valuesFile)
}

func runHelmUpdateDependencies(path string, directory string) error {
	log.Info().Msgf("Updating dependencies for %s...", filepath.Base(directory))
	cmd := exec.Command(path, "dependency", "build")
	cmd.Dir = directory
	return cmd.Run()
}

func runKubeconform(manifests bytes.Buffer, path string, strict bool, additionalSchemaPaths []string, k8sVersion string) (string, error) {
	cmd := kubeconformCommand(path, strict, additionalSchemaPaths, k8sVersion)

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

func kubeconformCommand(path string, strict bool, additionalSchemaPaths []string, k8sVersion string) *exec.Cmd {
	return exec.Command(path, kubeconformArgs(strict, additionalSchemaPaths, k8sVersion)...)
}

func kubeconformArgs(strict bool, additionalSchemaPaths []string, k8sVersion string) []string {
	args := []string{
		"-schema-location",
		"default",
		"-summary",
		"-kubernetes-version",
		k8sVersion,
	}

	if strict {
		args = append(args, "-strict")
	}

	for _, location := range additionalSchemaPaths {
		if location == "" {
			continue
		}

		args = append(args, "-schema-location")
		args = append(args, location)
	}

	return args
}

func parsePath(raw string) (interface{}, error) {
	v := strings.TrimSpace(raw)

	if v == "" {
		return Path{path: ""}, nil
	}

	parsed, err := filepath.Abs(v)

	if err != nil {
		return Path{}, err
	}

	return Path{path: parsed}, nil
}
