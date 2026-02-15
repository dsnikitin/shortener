package main

import (
	"github.com/dsnikitin/shortener/cmd/staticlint/custom/exit"
	critic "github.com/go-critic/go-critic/checkers/analyzer"
	"github.com/securego/gosec/v2/goanalysis"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"honnef.co/go/tools/simple/s1000"
	"honnef.co/go/tools/staticcheck"
)

func main() {
	analyzers := []*analysis.Analyzer{
		printf.Analyzer,
		shadow.Analyzer,
		structtag.Analyzer,
		s1000.Analyzer,      // simple staticcheck
		critic.Analyzer,     // go-critic
		goanalysis.Analyzer, // gosec
		exit.Analyzer,       // custom
	}

	// SA staticcheck
	for _, v := range staticcheck.Analyzers {
		analyzers = append(analyzers, v.Analyzer)
	}

	multichecker.Main(analyzers...)
}
