package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
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
	RegexSkipDir          string `env:"REGEX_SKIP_DIR" envDefault:"\.git"`
	KubernetesVersion     string `env:"KUBERNETES_VERSION" envDefault:"master"`
	Kubeconform           Path   `env:"KUBECONFORM"`
	Helm                  Path   `env:"HELM"`
	UpdateDependencies    bool   `env:"HELM_UPDATE_DEPENDENCIES"`
	LogLevel              string `env:"LOG_LEVEL" envDefault:"debug"`
	LogJson               bool   `env:"LOG_JSON" envDefault:"true"`
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

	if !cfg.LogJson {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)

	if err != nil {
		log.Fatal().Err(err).Msgf("Could not parse log level")
		return
	}

	zerolog.SetGlobalLevel(level)

	log.Trace().Msgf("Config: %s", cfg)

	additionalSchemaPaths := []string{}

	for _, path := range cfg.AdditionalSchemaPaths {
		additionalSchemaPaths = append(additionalSchemaPaths, path.path)
	}

	log.Trace().Msgf("Additional schema paths: %s", additionalSchemaPaths)

	feErr := run(cfg, additionalSchemaPaths, cfg.UpdateDependencies)

	if feErr != nil {
		log.Fatal().Stack().Err(feErr).Msg("")
		return
	}
}

func run(cfg Config, additionalSchemaPaths []string, updateDependencies bool) error {
	var validationsErrors []error
	skipRegex := regexp.MustCompile("^" + cfg.RegexSkipDir + "$")

	filepath.WalkDir(cfg.ChartsDirectory.path, func(path string, dirent fs.DirEntry, err error) error {
		logger := log.With().Str("path", path).Logger()
		if err != nil {
			logger.Warn().Err(err).Msg("skipping path")
			return nil
		}

		if dirent.IsDir() && skipRegex.MatchString(dirent.Name()) {
			logger.Info().Msg("matching skip regex, skipping")
			return fs.SkipDir
		}

		if dirent.Name() != "Chart.yaml" && dirent.Name() != "Chart.yml" {
			return nil
		}

		chart_dir := filepath.Dir(path)
		logger = log.With().Str("chart", filepath.Base(chart_dir)).Logger()
		filepath.WalkDir(filepath.Join(chart_dir, TestsPath), func(values_file string, dirent fs.DirEntry, err error) error {
			logger := logger.With().Str("values", dirent.Name()).Logger()
			if err != nil {
				logger.Err(err).Stack().Msg("Could not open directory")
				return err
			}
			if dirent.Name() == TestsPath {
				return nil
			}
			manifests, err := runHelm(cfg.Helm.path, chart_dir, dirent.Name(), updateDependencies)

			if err != nil {
				logger.Err(err).Str("stdout", manifests.String()).Msg("Could not run Helm")
				return err
			}

			// to use kubeconform as a library would need us to practically
			// reimplement its CLI
			// <https://github.com/yannh/kubeconform/blob/dcc77ac3a39ed1fb538b54fab57bbe87d1ece490/cmd/kubeconform/main.go#L47>,
			// so instead we shell out to it
			output, err := runKubeconform(manifests, cfg.Kubeconform.path, cfg.Strict, additionalSchemaPaths, cfg.KubernetesVersion)

			// if kubeconform could not be executed, the output will not be
			// in JSON format
			var js json.RawMessage
			if json.Unmarshal([]byte(output), &js) == nil {
				logger.Err(err).RawJSON("kubeconform", output).Msg("")
			} else {
				logger.Err(err).Msgf("kubeconform failed: %s", output)
			}

			if err != nil {
				validationsErrors = append(validationsErrors, err)
			}

			return nil

		})

		return fs.SkipDir // We processed the chart, so skip its directory
	})

	if validationsErrors != nil {
		return errors.New("Validation failed")
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

func runKubeconform(manifests bytes.Buffer, path string, strict bool, additionalSchemaPaths []string, kubernetesVersion string) ([]byte, error) {
	cmd := kubeconformCommand(path, strict, additionalSchemaPaths, kubernetesVersion)

	stdin, err := cmd.StdinPipe()

	if err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		stdin.Write(manifests.Bytes())
	}()

	output, err := cmd.CombinedOutput()

	return output, err
}

func kubeconformCommand(path string, strict bool, additionalSchemaPaths []string, kubernetesVersion string) *exec.Cmd {
	return exec.Command(path, kubeconformArgs(strict, additionalSchemaPaths, kubernetesVersion)...)
}

func kubeconformArgs(strict bool, additionalSchemaPaths []string, kubernetesVersion string) []string {
	args := []string{
		"-schema-location",
		"default",
		"-summary",
		"-kubernetes-version",
		kubernetesVersion,
		"-output",
		"json",
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

	log.Trace().Msgf("Arguments: %s", args)

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
