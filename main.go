package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sethvargo/go-githubactions"
	"github.com/zegl/kube-score/config"
	ks "github.com/zegl/kube-score/domain"
	"github.com/zegl/kube-score/parser"
	"github.com/zegl/kube-score/renderer/human"
	"github.com/zegl/kube-score/score"
	"github.com/zegl/kube-score/scorecard"
)

type namedReader struct {
	io.Reader
	name string
}

func (n namedReader) Name() string {
	return n.name
}

func main() {
	csvFilesStr := githubactions.GetInput("files")
	if csvFilesStr == "" {
		githubactions.Fatalf("missing files")
	}

	filePatternsToRead := strings.Split(csvFilesStr, ",")

	var allFilePointers []ks.NamedReader

	for _, filePat := range filePatternsToRead {
		filesToRead, err := filepath.Glob(filePat)

		for _, file := range filesToRead {
			if err != nil {
				githubactions.Fatalf("Failed to Glob the pattern: %v", err)
			}
			var fp io.Reader
			var filename string

			file = strings.TrimSpace(file)
			fp, err := os.Open(file)
			if err != nil {
				githubactions.Fatalf("Failed to read file: %v", err)
			}
			filename, _ = filepath.Abs(file)
			allFilePointers = append(allFilePointers, namedReader{Reader: fp, name: filename})
		}
	}

	verboseOutput := 0
	verboseOutputStr := githubactions.GetInput("verbose_output")
	if verboseOutputStr != "" {
		n, err := strconv.ParseInt(verboseOutputStr, 10, 32)
		if err != nil {
			githubactions.Fatalf("Couldn't parse 'verbose-output'. ", err)
		}
		verboseOutput = int(n)
	}

	kubeVerStr := githubactions.GetInput("kubernetes_version")
	if kubeVerStr == "" {
		kubeVerStr = "v1.18"
	}
	kubeVer, err := config.ParseSemver(kubeVerStr)
	if err != nil {
		githubactions.Fatalf("Invalid 'kubernetes-version'. Use format \"vN.NN\"")
	}

	cnf := config.Configuration{
		AllFiles:                              allFilePointers,
		VerboseOutput:                         verboseOutput,
		IgnoreContainerCpuLimitRequirement:    false,
		IgnoreContainerMemoryLimitRequirement: false,
		IgnoredTests:                          map[string]struct{}{},
		EnabledOptionalTests:                  map[string]struct{}{},
		UseIgnoreChecksAnnotation:             true,
		KubernetesVersion:                     kubeVer,
	}

	exitCode, err := doScore(cnf)
	if err != nil {
		githubactions.Fatalf("Error when scoring", err)
	}

	os.Exit(exitCode)
}

func doScore(cnf config.Configuration) (int, error) {
	parsedFiles, err := parser.ParseFiles(cnf)
	if err != nil {
		return 1, fmt.Errorf("failed to initializer parser: %w", err)
	}

	scoreCard, err := score.Score(parsedFiles, cnf)
	if err != nil {
		return 1, err
	}

	exitOneOnWarning := true
	var exitCode int
	switch {
	case scoreCard.AnyBelowOrEqualToGrade(scorecard.GradeCritical):
		exitCode = 1
	case exitOneOnWarning && scoreCard.AnyBelowOrEqualToGrade(scorecard.GradeWarning):
		exitCode = 1
	default:
		exitCode = 0
	}

	termWidth := 80
	verboseOutput := 0
	r := human.Human(scoreCard, verboseOutput, termWidth)

	output, _ := io.ReadAll(r)
	fmt.Print(string(output))

	return exitCode, nil
}
