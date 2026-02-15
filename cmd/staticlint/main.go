// Package main реализует multichecker для статического анализа Go кода.
//
// Данный multichecker объединяет несколько анализаторов для всесторонней проверки
// кода на наличие ошибок, неэффективных конструкций и потенциальных уязвимостей.
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

// main является точкой входа в программу multichecker.
//
// Для запуска анализа необходимо выполнить команду:
//
//	go run cmd/staticlint/main.go ./...
//
// Также можно использовать предварительно скомпилированную версию:
//
//	staticlint ./...
//
// Все анализаторы запускаются параллельно для максимальной производительности.
// Результаты работы анализаторов выводятся в формате, совместимом с текстовыми
// редакторами и IDE, что позволяет быстро переходить к местам обнаруженных проблем.
func main() {
	analyzers := []*analysis.Analyzer{
		// Проверяет корректность форматирующих строк в функциях Printf-семейства
		printf.Analyzer,
		// Выявляет затенение переменных (объявление переменной с именем, уже используемым во внешней области видимости)
		shadow.Analyzer,
		// Проверяет корректность тегов структур на соответствие стандарту reflect.StructTag
		structtag.Analyzer,
		// Анализатор из набора 'simple' (Staticcheck), предлагает упрощения кода: использование одиночных каналов вместо булевых флагов
		s1000.Analyzer,
		// go-critic - мощный линтер, реализующий множество проверок, выходящих за рамки стандартного набора
		critic.Analyzer,
		// gosec - анализатор безопасности, ищет потенциальные уязвимости в коде
		goanalysis.Analyzer,
		// Пользовательский анализатор, запрещающий прямой вызов os.Exit в функции main пакета main
		exit.Analyzer,
	}

	// Добавление всех анализаторов группы SA из пакета staticcheck
	// Группа SA включает в себя анализаторы для обнаружения ошибок и багов:
	// - SA1000: некорректное использование регулярных выражений
	// - SA1001: шаблоны с ошибками
	// - SA1002: некорректное использование времени и дат
	// - и многие другие (всего более 50 анализаторов)
	for _, v := range staticcheck.Analyzers {
		analyzers = append(analyzers, v.Analyzer)
	}

	// Запуск всех анализаторов через multichecker
	// multichecker.Main обрабатывает флаги командной строки, запускает анализ
	// и выводит результаты в консоль в удобочитаемом формате
	multichecker.Main(analyzers...)
}
