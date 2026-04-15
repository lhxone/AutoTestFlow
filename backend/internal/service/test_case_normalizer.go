package service

import (
	"regexp"
	"strings"
)

var markdownCaseTitlePattern = regexp.MustCompile(`^#+\s*用例标题[:：]\s*(.+?)\s*$`)
var markdownPlainTitlePattern = regexp.MustCompile(`^#\s+(.+?)\s*$`)
var markdownFrontMatterTitlePattern = regexp.MustCompile(`^title:\s*(.+?)\s*$`)

type markdownTestCase struct {
	Title          string
	Precondition   string
	Steps          string
	Expected       string
	preconditionLn []string
	stepLines      []string
	expectedLines  []string
	tables         []markdownTable
}

type markdownTable struct {
	Section string
	Rows    [][]string
}

func normalizeGeneratedTestCases(output *GenTestOutput) {
	if output == nil || len(output.TestCases) == 0 {
		return
	}

	docCases := parseMarkdownTestCases(output.TestDoc.Content)
	if len(docCases) == 0 {
		return
	}

	usedDocIndexes := make(map[int]struct{}, len(docCases))
	for index := range output.TestCases {
		matchIndex := findMatchingMarkdownCase(output.TestCases, output.TestCases[index], docCases, usedDocIndexes, index)
		if matchIndex < 0 {
			continue
		}
		usedDocIndexes[matchIndex] = struct{}{}
		applyMarkdownCase(&output.TestCases[index], docCases[matchIndex])
	}
}

func parseMarkdownTestCases(content string) []markdownTestCase {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	cases := make([]markdownTestCase, 0, 4)

	var current *markdownTestCase
	section := ""
	pendingTitle := ""
	var currentTable *markdownTable

	flushCurrent := func() {
		if current == nil {
			return
		}
		flushTable := func() {
			if current == nil || currentTable == nil || len(currentTable.Rows) == 0 {
				currentTable = nil
				return
			}
			current.tables = append(current.tables, *currentTable)
			currentTable = nil
		}
		flushTable()
		if len(current.stepLines) == 0 || len(current.stepLines) != len(current.expectedLines) {
			current.stepLines, current.expectedLines = inferStepExpectedLines(current.tables)
		}
		current.Precondition = strings.Join(current.preconditionLn, "\n")
		current.Steps = strings.Join(current.stepLines, "\n")
		current.Expected = strings.Join(current.expectedLines, "\n")
		if current.Title != "" || current.Precondition != "" || current.Steps != "" || current.Expected != "" {
			cases = append(cases, *current)
		}
		current = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if matches := markdownCaseTitlePattern.FindStringSubmatch(line); len(matches) == 2 {
			flushCurrent()
			current = &markdownTestCase{Title: strings.TrimSpace(matches[1])}
			section = ""
			continue
		}
		if matches := markdownFrontMatterTitlePattern.FindStringSubmatch(line); len(matches) == 2 && current == nil {
			pendingTitle = strings.TrimSpace(matches[1])
			continue
		}
		if matches := markdownPlainTitlePattern.FindStringSubmatch(line); len(matches) == 2 {
			flushCurrent()
			current = &markdownTestCase{Title: strings.TrimSpace(matches[1])}
			section = ""
			continue
		}
		if current == nil && pendingTitle != "" && (strings.HasPrefix(line, "##") || strings.HasPrefix(line, "|") || parseableMarkdownListLine(line)) {
			current = &markdownTestCase{Title: pendingTitle}
			section = ""
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "##") {
			if currentTable != nil && len(currentTable.Rows) > 0 {
				current.tables = append(current.tables, *currentTable)
				currentTable = nil
			}
			section = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}

		if cells, ok := parseMarkdownTableRow(line); ok {
			if isMarkdownTableSeparator(cells) {
				continue
			}
			if currentTable == nil {
				currentTable = &markdownTable{Section: section}
			}
			currentTable.Rows = append(currentTable.Rows, cells)
			continue
		}
		if currentTable != nil && len(currentTable.Rows) > 0 {
			current.tables = append(current.tables, *currentTable)
			currentTable = nil
		}

		switch {
		case strings.Contains(section, "前置条件"):
			if item, ok := parseMarkdownListItem(line); ok {
				current.preconditionLn = append(current.preconditionLn, item)
			}
		}
	}

	flushCurrent()
	return cases
}

func parseableMarkdownListLine(line string) bool {
	_, ok := parseMarkdownListItem(line)
	return ok
}

func isMarkdownStepSection(section string) bool {
	section = strings.TrimSpace(strings.ToLower(section))
	return strings.Contains(section, "用例步骤") || strings.Contains(section, "测试步骤") || strings.Contains(section, "test steps")
}

func parseMarkdownListItem(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	for _, separator := range []string{". ", "、", ") ", "）"} {
		if idx := strings.Index(line, separator); idx > 0 && isDigits(line[:idx]) {
			return strings.TrimSpace(line), true
		}
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:]), true
	}
	return "", false
}

func parseMarkdownTableRow(line string) ([]string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.Contains(line[1:], "|") {
		return nil, false
	}
	parts := strings.Split(line, "|")
	if len(parts) < 4 {
		return nil, false
	}
	cells := make([]string, 0, len(parts)-2)
	for _, part := range parts[1 : len(parts)-1] {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells, true
}

func inferStepExpectedLines(tables []markdownTable) ([]string, []string) {
	bestScore := -1
	var bestSteps []string
	var bestExpected []string

	for _, table := range tables {
		steps, expected, score := inferStepExpectedFromTable(table)
		if len(steps) == 0 || len(steps) != len(expected) {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestSteps = steps
			bestExpected = expected
		}
	}

	return bestSteps, bestExpected
}

func inferStepExpectedFromTable(table markdownTable) ([]string, []string, int) {
	rows := normalizeTableRows(table.Rows)
	if len(rows) == 0 {
		return nil, nil, -1
	}
	if shouldSkipFirstRowAsHeader(rows) {
		rows = rows[1:]
	}
	if len(rows) == 0 {
		return nil, nil, -1
	}

	stepCol, expectedCol := inferStepExpectedColumns(rows)
	if stepCol < 0 || expectedCol < 0 || stepCol == expectedCol {
		return nil, nil, -1
	}

	steps := make([]string, 0, len(rows))
	expected := make([]string, 0, len(rows))
	totalTextLen := 0
	for _, row := range rows {
		if stepCol >= len(row) || expectedCol >= len(row) {
			continue
		}
		step := strings.TrimSpace(row[stepCol])
		expect := strings.TrimSpace(row[expectedCol])
		if step == "" && expect == "" {
			continue
		}
		steps = append(steps, step)
		expected = append(expected, expect)
		totalTextLen += len(step) + len(expect)
	}
	if len(steps) == 0 || len(steps) != len(expected) {
		return nil, nil, -1
	}

	score := len(steps)*10 + totalTextLen/20
	if isMarkdownStepSection(table.Section) {
		score += 30
	}
	if isHistorySection(table.Section) {
		score -= 40
	}
	if rowsLookIndexed(rows) && stepCol > 0 {
		score += 10
	}

	return steps, expected, score
}

func normalizeTableRows(rows [][]string) [][]string {
	normalized := make([][]string, 0, len(rows))
	for _, row := range rows {
		trimmed := trimTrailingEmptyCells(row)
		if len(trimmed) >= 2 {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func trimTrailingEmptyCells(row []string) []string {
	end := len(row)
	for end > 0 && strings.TrimSpace(row[end-1]) == "" {
		end--
	}
	if end == 0 {
		return nil
	}
	return append([]string(nil), row[:end]...)
}

func shouldSkipFirstRowAsHeader(rows [][]string) bool {
	if len(rows) < 2 {
		return false
	}
	first := rows[0]
	second := rows[1]
	if rowLooksLikeHeader(first) && !rowLooksLikeHeader(second) {
		return true
	}
	if !isIndexLikeCell(cellAt(first, 0)) && isIndexLikeCell(cellAt(second, 0)) && rowLooksCompact(first) {
		return true
	}
	return false
}

func rowLooksLikeHeader(row []string) bool {
	if len(row) < 2 {
		return false
	}
	return rowLooksCompact(row) && !rowsContainLongSentence([][]string{row})
}

func rowLooksCompact(row []string) bool {
	for _, cell := range row {
		text := strings.TrimSpace(cell)
		if text == "" {
			continue
		}
		if len([]rune(text)) > 32 {
			return false
		}
		if strings.ContainsAny(text, "。！？；，:：") {
			return false
		}
	}
	return true
}

func rowsContainLongSentence(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if len([]rune(strings.TrimSpace(cell))) > 20 {
				return true
			}
		}
	}
	return false
}

func inferStepExpectedColumns(rows [][]string) (int, int) {
	columnCount := maxColumnCount(rows)
	if columnCount < 2 {
		return -1, -1
	}
	if columnCount == 2 {
		return 0, 1
	}

	indexColumn := -1
	bestIndexRatio := 0.0
	for col := 0; col < columnCount; col++ {
		ratio := columnIndexRatio(rows, col)
		if ratio > bestIndexRatio {
			bestIndexRatio = ratio
			indexColumn = col
		}
	}
	if bestIndexRatio >= 0.6 && indexColumn >= 0 && indexColumn < columnCount-1 {
		nonIndexCols := make([]int, 0, 2)
		for col := indexColumn + 1; col < columnCount; col++ {
			if columnTextPresence(rows, col) > 0.3 {
				nonIndexCols = append(nonIndexCols, col)
			}
			if len(nonIndexCols) == 2 {
				break
			}
		}
		if len(nonIndexCols) == 2 {
			return nonIndexCols[0], nonIndexCols[1]
		}
	}

	bestLeft, bestRight := 0, 1
	bestScore := -1.0
	for left := 0; left < columnCount-1; left++ {
		for right := left + 1; right < columnCount; right++ {
			score := columnTextScore(rows, left) + columnTextScore(rows, right)
			score -= columnIndexRatio(rows, left) * 30
			score -= columnIndexRatio(rows, right) * 20
			if right == left+1 {
				score += 3
			}
			if score > bestScore {
				bestScore = score
				bestLeft, bestRight = left, right
			}
		}
	}
	return bestLeft, bestRight
}

func maxColumnCount(rows [][]string) int {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

func columnIndexRatio(rows [][]string, col int) float64 {
	total := 0
	indexLike := 0
	for _, row := range rows {
		if col >= len(row) {
			continue
		}
		text := strings.TrimSpace(row[col])
		if text == "" {
			continue
		}
		total++
		if isIndexLikeCell(text) {
			indexLike++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(indexLike) / float64(total)
}

func columnTextPresence(rows [][]string, col int) float64 {
	total := 0
	withText := 0
	for _, row := range rows {
		if col >= len(row) {
			continue
		}
		total++
		if strings.TrimSpace(row[col]) != "" {
			withText++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(withText) / float64(total)
}

func columnTextScore(rows [][]string, col int) float64 {
	score := 0.0
	for _, row := range rows {
		if col >= len(row) {
			continue
		}
		text := strings.TrimSpace(row[col])
		if text == "" {
			continue
		}
		score += float64(minInt(len([]rune(text)), 80))
	}
	return score
}

func rowsLookIndexed(rows [][]string) bool {
	return columnIndexRatio(rows, 0) >= 0.6
}

func isIndexLikeCell(text string) bool {
	text = strings.TrimSpace(strings.Trim(text, "[]()"))
	if text == "" {
		return false
	}
	if isDigits(strings.TrimRight(text, ".)、:：-")) {
		return true
	}
	markers := []string{"step ", "步骤", "step-"}
	lower := strings.ToLower(text)
	for _, marker := range markers {
		if strings.HasPrefix(lower, marker) {
			return true
		}
	}
	return false
}

func cellAt(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return row[col]
}

func isHistorySection(section string) bool {
	section = strings.ToLower(strings.TrimSpace(section))
	return strings.Contains(section, "变更历史") || strings.Contains(section, "history") || strings.Contains(section, "版本")
}

func isMarkdownTableSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		if trimmed == "" {
			return false
		}
		trimmed = strings.ReplaceAll(trimmed, "-", "")
		trimmed = strings.ReplaceAll(trimmed, ":", "")
		if trimmed != "" {
			return false
		}
	}
	return true
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func findMatchingMarkdownCase(allGenerated []GenTestCase, generated GenTestCase, docCases []markdownTestCase, used map[int]struct{}, generatedIndex int) int {
	generatedTitle := normalizeCaseTitle(generated.Title)
	for index, docCase := range docCases {
		if _, exists := used[index]; exists {
			continue
		}
		docTitle := normalizeCaseTitle(docCase.Title)
		if generatedTitle != "" && generatedTitle == docTitle {
			return index
		}
	}

	for index, docCase := range docCases {
		if _, exists := used[index]; exists {
			continue
		}
		docTitle := normalizeCaseTitle(docCase.Title)
		if generatedTitle != "" && docTitle != "" &&
			(strings.Contains(generatedTitle, docTitle) || strings.Contains(docTitle, generatedTitle)) {
			return index
		}
	}

	if len(allGenerated) == len(docCases) {
		if _, exists := used[generatedIndex]; !exists && generatedIndex < len(docCases) {
			return generatedIndex
		}
	}

	if len(allGenerated) == 1 && len(docCases) == 1 {
		return 0
	}

	return -1
}

func normalizeCaseTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	if title == "" {
		return ""
	}
	var builder strings.Builder
	lastWasSpace := false
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastWasSpace = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastWasSpace = false
		case r >= 0x4e00 && r <= 0x9fff:
			builder.WriteRune(r)
			lastWasSpace = false
		default:
			if !lastWasSpace {
				builder.WriteByte(' ')
				lastWasSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func applyMarkdownCase(target *GenTestCase, source markdownTestCase) {
	if target == nil {
		return
	}
	if strings.TrimSpace(target.Title) == "" && source.Title != "" {
		target.Title = source.Title
	}
	if strings.TrimSpace(target.Precondition) == "" && source.Precondition != "" {
		target.Precondition = source.Precondition
	}

	if len(source.stepLines) == 0 || len(source.stepLines) != len(source.expectedLines) {
		return
	}
	if target.Steps == source.Steps && target.Expected == source.Expected {
		return
	}

	target.Steps = source.Steps
	target.Expected = source.Expected
}
