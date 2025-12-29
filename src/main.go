package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	MaxRows         = 254
	MaxCols         = 26
	ScreenRows      = 20
	ScreenCols      = 9
	Epsilon         = 1e-9
	Version         = "1.0.4"
	MaxFormulaDepth = 100
	MaxFormulaLen   = 1000
	MaxRangeCells   = 10000
)

type Cell struct {
	Formula     string
	Value       float64
	DisplayText string
	IsFormula   bool
	IsText      bool
	Error       string
}

type Spreadsheet struct {
	cells       map[string]*Cell
	currentRow  int
	currentCol  int
	topRow      int
	leftCol     int
	mode        string
	inputBuffer string
	colWidths   map[int]int
	calculating map[string]bool
	filename    string
	modified    bool
	evalDepth   int
}

func NewSpreadsheet() *Spreadsheet {
	return &Spreadsheet{
		cells:       make(map[string]*Cell),
		currentRow:  1,
		currentCol:  0,
		topRow:      1,
		leftCol:     0,
		mode:        "READY",
		colWidths:   make(map[int]int),
		calculating: make(map[string]bool),
		filename:    "",
		modified:    false,
		evalDepth:   0,
	}
}

func GetCellRef(row, col int) string {
	if col < 0 || col >= MaxCols || row < 1 || row > MaxRows {
		return ""
	}
	return fmt.Sprintf("%c%d", 'A'+col, row)
}

func ParseCellRef(ref string) (int, int, error) {
	if ref == "" {
		return 0, 0, fmt.Errorf("empty reference")
	}
	
	ref = strings.TrimSpace(ref)
	ref = strings.ToUpper(ref)
	
	if len(ref) < 2 {
		return 0, 0, fmt.Errorf("reference too short")
	}
	
	colEnd := 0
	for colEnd < len(ref) && unicode.IsLetter(rune(ref[colEnd])) {
		colEnd++
	}
	
	if colEnd == 0 {
		return 0, 0, fmt.Errorf("no column letter")
	}
	
	if colEnd >= len(ref) {
		return 0, 0, fmt.Errorf("no row number")
	}
	
	colStr := ref[:colEnd]
	rowStr := ref[colEnd:]
	
	if len(colStr) != 1 {
		return 0, 0, fmt.Errorf("multi-letter columns not supported")
	}
	
	col := int(colStr[0] - 'A')
	if col < 0 || col >= MaxCols {
		return 0, 0, fmt.Errorf("column out of range")
	}
	
	row, err := strconv.Atoi(rowStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid row number")
	}
	
	if row < 1 || row > MaxRows {
		return 0, 0, fmt.Errorf("row out of range")
	}
	
	return row, col, nil
}

func (s *Spreadsheet) GetCell(row, col int) *Cell {
	if s == nil {
		return &Cell{Error: "INVALID SPREADSHEET"}
	}
	if row < 1 || row > MaxRows || col < 0 || col >= MaxCols {
		return &Cell{Error: "OUT OF RANGE"}
	}
	ref := GetCellRef(row, col)
	if ref == "" {
		return &Cell{Error: "INVALID REF"}
	}
	if s.cells == nil {
		s.cells = make(map[string]*Cell)
	}
	if cell, exists := s.cells[ref]; exists && cell != nil {
		return cell
	}
	cell := &Cell{}
	s.cells[ref] = cell
	return cell
}

func (s *Spreadsheet) GetColWidth(col int) int {
	if s == nil || col < 0 || col >= MaxCols {
		return 9
	}
	if s.colWidths == nil {
		return 9
	}
	if width, exists := s.colWidths[col]; exists {
		if width > 0 && width <= 40 {
			return width
		}
	}
	return 9
}

func floatEquals(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return a == b
	}
	return math.Abs(a-b) < Epsilon
}

func safeSubstring(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start > end {
		return ""
	}
	if start >= len(s) {
		return ""
	}
	return s[start:end]
}

func (s *Spreadsheet) parseRange(rangeStr string) ([]string, error) {
	rangeStr = strings.TrimSpace(rangeStr)
	
	if rangeStr == "" {
		return nil, fmt.Errorf("empty range")
	}
	
	rangeStr = strings.ReplaceAll(rangeStr, "…", "...")
	
	if strings.Contains(rangeStr, "...") {
		parts := strings.Split(rangeStr, "...")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}
		
		startRef := strings.TrimSpace(parts[0])
		endRef := strings.TrimSpace(parts[1])
		
		startRow, startCol, err := ParseCellRef(startRef)
		if err != nil {
			return nil, fmt.Errorf("start cell error: %v", err)
		}
		
		endRow, endCol, err := ParseCellRef(endRef)
		if err != nil {
			return nil, fmt.Errorf("end cell error: %v", err)
		}
		
		if startRow > endRow {
			startRow, endRow = endRow, startRow
		}
		if startCol > endCol {
			startCol, endCol = endCol, startCol
		}
		
		cells := make([]string, 0)
		cellCount := 0
		for row := startRow; row <= endRow; row++ {
			for col := startCol; col <= endCol; col++ {
				if cellCount >= MaxRangeCells {
					return nil, fmt.Errorf("range too large")
				}
				ref := GetCellRef(row, col)
				if ref != "" {
					cells = append(cells, ref)
					cellCount++
				}
			}
		}
		return cells, nil
	}
	
	_, _, err := ParseCellRef(rangeStr)
	if err != nil {
		return nil, err
	}
	return []string{strings.ToUpper(rangeStr)}, nil
}

func (s *Spreadsheet) EvaluateFormula(formula string, cellRef string) (float64, error) {
	if s == nil {
		return 0, fmt.Errorf("INVALID SPREADSHEET")
	}
	s.evalDepth++
	defer func() {
		s.evalDepth--
	}()
	if s.evalDepth > MaxFormulaDepth {
		return 0, fmt.Errorf("TOO DEEP")
	}
	if formula == "" {
		return 0, nil
	}
	if cellRef == "" {
		return 0, fmt.Errorf("INVALID REF")
	}
	if s.calculating == nil {
		s.calculating = make(map[string]bool)
	}
	if s.calculating[cellRef] {
		return 0, fmt.Errorf("CIRCULAR")
	}
	s.calculating[cellRef] = true
	defer func() {
		if s.calculating != nil {
			delete(s.calculating, cellRef)
		}
	}()
	
	formula = strings.TrimSpace(formula)
	if formula == "" {
		return 0, nil
	}
	if len(formula) > MaxFormulaLen {
		return 0, fmt.Errorf("TOO LONG")
	}
	
	if len(formula) >= 2 && len(formula) <= 10 && unicode.IsLetter(rune(formula[0])) {
		isValidRef := true
		for i, ch := range formula {
			if i == 0 {
				if !unicode.IsLetter(ch) {
					isValidRef = false
					break
				}
			} else {
				if !unicode.IsDigit(ch) {
					isValidRef = false
					break
				}
			}
		}
		if isValidRef {
			row, col, err := ParseCellRef(formula)
			if err == nil {
				targetRef := GetCellRef(row, col)
				if targetRef == "" {
					return 0, fmt.Errorf("INVALID REF")
				}
				cell := s.GetCell(row, col)
				if cell == nil {
					return 0, fmt.Errorf("INVALID REF")
				}
				if cell.Error != "" {
					return 0, fmt.Errorf(cell.Error)
				}
				if cell.IsFormula && len(cell.Formula) > 0 {
					return s.EvaluateFormula(cell.Formula[1:], targetRef)
				}
				return cell.Value, nil
			}
		}
	}
	
	if len(formula) > 4 && formula[0] == '@' {
		upperFormula := strings.ToUpper(formula)
		if strings.HasPrefix(upperFormula, "@SUM(") {
			return s.evaluateSUM(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@AVG(") {
			return s.evaluateAVG(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@MIN(") {
			return s.evaluateMIN(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@MAX(") {
			return s.evaluateMAX(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@COUNT(") {
			return s.evaluateCOUNT(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@SQRT(") {
			return s.evaluateSQRT(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@ABS(") {
			return s.evaluateABS(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@INT(") {
			return s.evaluateINT(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@ROUND(") {
			return s.evaluateROUND(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@IF(") {
			return s.evaluateIF(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@PI(") || strings.HasPrefix(upperFormula, "@PI)") {
			return math.Pi, nil
		}
		if strings.HasPrefix(upperFormula, "@EXP(") {
			return s.evaluateEXP(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@LN(") {
			return s.evaluateLN(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@LOG(") {
			return s.evaluateLOG(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@SIN(") {
			return s.evaluateSIN(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@COS(") {
			return s.evaluateCOS(formula, cellRef)
		}
		if strings.HasPrefix(upperFormula, "@TAN(") {
			return s.evaluateTAN(formula, cellRef)
		}
	}
	return s.evaluateExpression(formula, cellRef)
}

func (s *Spreadsheet) evaluateSUM(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@SUM(")
	if startIdx == -1 {
		return 0, fmt.Errorf("@SUM not found")
	}
	
	openParen := startIdx + 5
	if openParen >= len(formula) {
		return 0, fmt.Errorf("missing range")
	}
	
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	
	rangeStr := formula[openParen:closeParen]
	rangeStr = strings.TrimSpace(rangeStr)
	
	cells, err := s.parseRange(rangeStr)
	if err != nil {
		return 0, err
	}
	
	sum := 0.0
	for _, targetRef := range cells {
		row, col, err := ParseCellRef(targetRef)
		if err != nil {
			continue
		}
		cell := s.GetCell(row, col)
		if cell == nil || cell.Error != "" || cell.IsText {
			continue
		}
		if cell.IsFormula && len(cell.Formula) > 0 {
			val, err := s.EvaluateFormula(cell.Formula[1:], targetRef)
			if err == nil && !math.IsNaN(val) && !math.IsInf(val, 0) {
				sum += val
			}
		} else if !math.IsNaN(cell.Value) && !math.IsInf(cell.Value, 0) {
			sum += cell.Value
		}
	}
	return sum, nil
}

func (s *Spreadsheet) evaluateAVG(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@AVG(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	rangeStr := strings.TrimSpace(formula[openParen:closeParen])
	
	cells, err := s.parseRange(rangeStr)
	if err != nil {
		return 0, err
	}
	sum := 0.0
	count := 0
	for _, targetRef := range cells {
		row, col, err := ParseCellRef(targetRef)
		if err != nil {
			continue
		}
		cell := s.GetCell(row, col)
		if cell == nil || cell.Error != "" || cell.IsText {
			continue
		}
		var val float64
		var valErr error
		if cell.IsFormula && len(cell.Formula) > 0 {
			val, valErr = s.EvaluateFormula(cell.Formula[1:], targetRef)
		} else {
			val = cell.Value
			valErr = nil
		}
		if valErr == nil && !math.IsNaN(val) && !math.IsInf(val, 0) {
			sum += val
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	return sum / float64(count), nil
}

func (s *Spreadsheet) evaluateMIN(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@MIN(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	rangeStr := strings.TrimSpace(formula[openParen:closeParen])
	
	cells, err := s.parseRange(rangeStr)
	if err != nil {
		return 0, err
	}
	min := math.MaxFloat64
	found := false
	for _, targetRef := range cells {
		row, col, err := ParseCellRef(targetRef)
		if err != nil {
			continue
		}
		cell := s.GetCell(row, col)
		if cell == nil || cell.Error != "" || cell.IsText {
			continue
		}
		var val float64
		if cell.IsFormula && len(cell.Formula) > 0 {
			v, err := s.EvaluateFormula(cell.Formula[1:], targetRef)
			if err != nil {
				continue
			}
			val = v
		} else {
			val = cell.Value
		}
		if !math.IsNaN(val) && !math.IsInf(val, 0) && val < min {
			min = val
			found = true
		}
	}
	if !found {
		return 0, nil
	}
	return min, nil
}

func (s *Spreadsheet) evaluateMAX(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@MAX(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	rangeStr := strings.TrimSpace(formula[openParen:closeParen])
	
	cells, err := s.parseRange(rangeStr)
	if err != nil {
		return 0, err
	}
	max := -math.MaxFloat64
	found := false
	for _, targetRef := range cells {
		row, col, err := ParseCellRef(targetRef)
		if err != nil {
			continue
		}
		cell := s.GetCell(row, col)
		if cell == nil || cell.Error != "" || cell.IsText {
			continue
		}
		var val float64
		if cell.IsFormula && len(cell.Formula) > 0 {
			v, err := s.EvaluateFormula(cell.Formula[1:], targetRef)
			if err != nil {
				continue
			}
			val = v
		} else {
			val = cell.Value
		}
		if !math.IsNaN(val) && !math.IsInf(val, 0) && val > max {
			max = val
			found = true
		}
	}
	if !found {
		return 0, nil
	}
	return max, nil
}

func (s *Spreadsheet) evaluateCOUNT(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@COUNT(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 7
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	rangeStr := strings.TrimSpace(formula[openParen:closeParen])
	
	cells, err := s.parseRange(rangeStr)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, targetRef := range cells {
		row, col, err := ParseCellRef(targetRef)
		if err != nil {
			continue
		}
		cell := s.GetCell(row, col)
		if cell != nil && !cell.IsText && cell.Formula != "" && cell.Error == "" {
			count++
		}
	}
	return float64(count), nil
}

func (s *Spreadsheet) evaluateSQRT(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@SQRT(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 6
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	if val < 0 {
		return 0, fmt.Errorf("SQRT NEG")
	}
	return math.Sqrt(val), nil
}

func (s *Spreadsheet) evaluateABS(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@ABS(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Abs(val), nil
}

func (s *Spreadsheet) evaluateINT(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@INT(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Floor(val), nil
}

func (s *Spreadsheet) evaluateROUND(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@ROUND(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 7
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Round(val), nil
}

func (s *Spreadsheet) evaluateIF(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@IF(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 4
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	args := strings.TrimSpace(formula[openParen:closeParen])
	
	parts := strings.Split(args, ",")
	if len(parts) != 3 {
		return 0, fmt.Errorf("IF needs 3 args")
	}
	condition := strings.TrimSpace(parts[0])
	trueVal := strings.TrimSpace(parts[1])
	falseVal := strings.TrimSpace(parts[2])
	condResult, err := s.evaluateCondition(condition, cellRef)
	if err != nil {
		return 0, err
	}
	if condResult {
		return s.evaluateExpression(trueVal, cellRef)
	}
	return s.evaluateExpression(falseVal, cellRef)
}

func (s *Spreadsheet) evaluateCondition(cond string, cellRef string) (bool, error) {
	cond = strings.TrimSpace(cond)
	operators := []string{">=", "<=", "!=", "<>", "=", ">", "<"}
	for _, op := range operators {
		if strings.Contains(cond, op) {
			parts := strings.Split(cond, op)
			if len(parts) == 2 {
				left, err := s.evaluateExpression(strings.TrimSpace(parts[0]), cellRef)
				if err != nil {
					return false, err
				}
				right, err := s.evaluateExpression(strings.TrimSpace(parts[1]), cellRef)
				if err != nil {
					return false, err
				}
				switch op {
				case ">=":
					return left >= right, nil
				case "<=":
					return left <= right, nil
				case "!=", "<>":
					return !floatEquals(left, right), nil
				case "=":
					return floatEquals(left, right), nil
				case ">":
					return left > right, nil
				case "<":
					return left < right, nil
				}
			}
		}
	}
	val, err := s.evaluateExpression(cond, cellRef)
	if err != nil {
		return false, err
	}
	return !floatEquals(val, 0), nil
}

func (s *Spreadsheet) evaluateEXP(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@EXP(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Exp(val), nil
}

func (s *Spreadsheet) evaluateLN(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@LN(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 4
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, fmt.Errorf("LN NEG")
	}
	return math.Log(val), nil
}

func (s *Spreadsheet) evaluateLOG(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@LOG(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, fmt.Errorf("LOG NEG")
	}
	return math.Log10(val), nil
}

func (s *Spreadsheet) evaluateSIN(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@SIN(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Sin(val), nil
}

func (s *Spreadsheet) evaluateCOS(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@COS(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Cos(val), nil
}

func (s *Spreadsheet) evaluateTAN(formula string, cellRef string) (float64, error) {
	startIdx := strings.Index(strings.ToUpper(formula), "@TAN(")
	if startIdx == -1 {
		return 0, fmt.Errorf("SYNTAX")
	}
	openParen := startIdx + 5
	closeParen := strings.LastIndex(formula, ")")
	if closeParen == -1 || closeParen <= openParen {
		return 0, fmt.Errorf("missing )")
	}
	arg := strings.TrimSpace(formula[openParen:closeParen])
	
	val, err := s.evaluateExpression(arg, cellRef)
	if err != nil {
		return 0, err
	}
	return math.Tan(val), nil
}

func (s *Spreadsheet) evaluateExpression(expr string, cellRef string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, nil
	}
	parenDepth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			parenDepth++
		} else if expr[i] == '(' {
			parenDepth--
		}
		if parenDepth == 0 && (expr[i] == '+' || expr[i] == '-') {
			if i > 0 {
				prevChar := expr[i-1]
				if prevChar == 'E' || prevChar == 'e' {
					continue
				}
				if prevChar == '+' || prevChar == '-' || prevChar == '*' || prevChar == '/' || prevChar == '^' || prevChar == '(' {
					continue
				}
			}
			if i == 0 {
				continue
			}
			left, err := s.evaluateExpression(safeSubstring(expr, 0, i), cellRef)
			if err != nil {
				return 0, err
			}
			right, err := s.evaluateExpression(safeSubstring(expr, i+1, len(expr)), cellRef)
			if err != nil {
				return 0, err
			}
			if expr[i] == '+' {
				return left + right, nil
			}
			return left - right, nil
		}
	}
	parenDepth = 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			parenDepth++
		} else if expr[i] == '(' {
			parenDepth--
		}
		if parenDepth == 0 && (expr[i] == '*' || expr[i] == '/') {
			left, err := s.evaluateExpression(safeSubstring(expr, 0, i), cellRef)
			if err != nil {
				return 0, err
			}
			right, err := s.evaluateExpression(safeSubstring(expr, i+1, len(expr)), cellRef)
			if err != nil {
				return 0, err
			}
			if expr[i] == '*' {
				return left * right, nil
			}
			if floatEquals(right, 0) {
				return 0, fmt.Errorf("DIV/0")
			}
			return left / right, nil
		}
	}
	parenDepth = 0
	for i := 0; i < len(expr); i++ {
		if expr[i] == '(' {
			parenDepth++
		} else if expr[i] == ')' {
			parenDepth--
		}
		if parenDepth == 0 && expr[i] == '^' {
			left, err := s.evaluateExpression(safeSubstring(expr, 0, i), cellRef)
			if err != nil {
				return 0, err
			}
			right, err := s.evaluateExpression(safeSubstring(expr, i+1, len(expr)), cellRef)
			if err != nil {
				return 0, err
			}
			return math.Pow(left, right), nil
		}
	}
	if len(expr) > 2 && expr[0] == '(' && expr[len(expr)-1] == ')' {
		return s.evaluateExpression(safeSubstring(expr, 1, len(expr)-1), cellRef)
	}
	if len(expr) >= 2 && unicode.IsLetter(rune(expr[0])) {
		row, col, err := ParseCellRef(expr)
		if err == nil {
			cell := s.GetCell(row, col)
			if cell != nil && cell.Error == "" {
				if cell.IsFormula && len(cell.Formula) > 0 {
					return s.EvaluateFormula(cell.Formula[1:], GetCellRef(row, col))
				}
				return cell.Value, nil
			}
		}
	}
	if len(expr) > 1 && expr[0] == '-' {
		val, err := s.evaluateExpression(safeSubstring(expr, 1, len(expr)), cellRef)
		if err != nil {
			return 0, err
		}
		return -val, nil
	}
	if len(expr) > 1 && expr[0] == '+' {
		return s.evaluateExpression(safeSubstring(expr, 1, len(expr)), cellRef)
	}
	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse '%s'", expr)
	}
	return val, nil
}

func (s *Spreadsheet) SetCell(row, col int, content string) {
	if s == nil {
		return
	}
	if row < 1 || row > MaxRows || col < 0 || col >= MaxCols {
		return
	}
	cell := s.GetCell(row, col)
	if cell == nil {
		return
	}
	content = strings.TrimSpace(content)
	cell.Formula = content
	cell.Value = 0
	cell.DisplayText = ""
	cell.IsFormula = false
	cell.IsText = false
	cell.Error = ""
	if content == "" {
		s.modified = true
		return
	}
	if len(content) > 0 && content[0] == '"' {
		cell.IsText = true
		cell.DisplayText = strings.Trim(content, "\"")
		s.modified = true
		return
	}
	if len(content) > 1 && content[0] == '-' && content[1] == '-' {
		cell.IsText = true
		char := "-"
		if len(content) > 2 {
			char = string(content[2])
		}
		width := s.GetColWidth(col)
		cell.DisplayText = strings.Repeat(char, width)
		s.modified = true
		return
	}
	if len(content) > 1 && content[0] == '=' && content[1] == '=' {
		cell.IsText = true
		char := "="
		if len(content) > 2 {
			char = string(content[2])
		}
		width := s.GetColWidth(col)
		cell.DisplayText = strings.Repeat(char, width)
		s.modified = true
		return
	}
	if len(content) > 0 && content[0] == '+' {
		cell.IsFormula = true
		if len(content) == 1 {
			cell.Error = "SYNTAX"
			cell.DisplayText = "SYNTAX"
			s.modified = true
			return
		}
		formula := content[1:]
		cellRef := GetCellRef(row, col)
		if s.calculating == nil {
			s.calculating = make(map[string]bool)
		}
		s.calculating = make(map[string]bool)
		s.evalDepth = 0
		value, err := s.EvaluateFormula(formula, cellRef)
		if err != nil {
			cell.Error = err.Error()
			cell.DisplayText = err.Error()
		} else {
			cell.Value = value
			cell.DisplayText = formatNumber(value)
		}
		s.modified = true
		return
	}
	val, err := strconv.ParseFloat(content, 64)
	if err == nil {
		cell.Value = val
		cell.DisplayText = formatNumber(val)
	} else {
		cell.IsText = true
		cell.DisplayText = content
	}
	s.modified = true
}

func formatNumber(val float64) string {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return "ERROR"
	}
	if floatEquals(val, math.Round(val)) && math.Abs(val) < 1e10 {
		return fmt.Sprintf("%d", int64(math.Round(val)))
	}
	str := fmt.Sprintf("%.6f", val)
	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	if len(str) > 12 {
		str = fmt.Sprintf("%.2e", val)
	}
	return str
}

func (s *Spreadsheet) Recalculate() {
	if s == nil || s.cells == nil {
		return
	}
	for row := 1; row <= MaxRows; row++ {
		for col := 0; col < MaxCols; col++ {
			ref := GetCellRef(row, col)
			if ref == "" {
				continue
			}
			if cell, exists := s.cells[ref]; exists && cell != nil && cell.IsFormula {
				if s.calculating == nil {
					s.calculating = make(map[string]bool)
				}
				s.calculating = make(map[string]bool)
				s.evalDepth = 0
				if len(cell.Formula) > 0 {
					formula := cell.Formula[1:]
					value, err := s.EvaluateFormula(formula, ref)
					if err != nil {
						cell.Error = err.Error()
						cell.DisplayText = err.Error()
					} else {
						cell.Value = value
						cell.DisplayText = formatNumber(value)
						cell.Error = ""
					}
				}
			}
		}
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func centerString(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) >= width {
		return s[:width]
	}
	leftPad := (width - len(s)) / 2
	rightPad := width - len(s) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

func padLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Helper to check if a cell is empty (no content)
func (s *Spreadsheet) isCellEmpty(row, col int) bool {
	cell := s.GetCell(row, col)
	return cell == nil || cell.Formula == ""
}

func (s *Spreadsheet) Display() {
	if s == nil {
		fmt.Println("Error: Invalid spreadsheet")
		return
	}
	clearScreen()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                         VisiCalc (Go Edition) v1.0.4                           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Print("     ")
	for col := s.leftCol; col < s.leftCol+ScreenCols && col < MaxCols; col++ {
		width := s.GetColWidth(col)
		header := fmt.Sprintf("%c", 'A'+col)
		fmt.Print(centerString(header, width) + " ")
	}
	fmt.Println()
	fmt.Print("    ")
	for col := s.leftCol; col < s.leftCol+ScreenCols && col < MaxCols; col++ {
		width := s.GetColWidth(col)
		fmt.Print(strings.Repeat("─", width) + " ")
	}
	fmt.Println()
	
	// Display rows
	for row := s.topRow; row < s.topRow+ScreenRows && row <= MaxRows; row++ {
		fmt.Printf("%3d│", row)
		
		// Track which columns have been consumed by overflow
		skipUntilCol := -1
		
		for col := s.leftCol; col < s.leftCol+ScreenCols && col < MaxCols; col++ {
			width := s.GetColWidth(col)
			
			// Skip this column if it's been consumed by overflow
			if col < skipUntilCol {
				// Don't print anything, the overflow text already covered this space
				continue
			}
			
			cell := s.GetCell(row, col)
			display := ""
			
			if cell != nil && cell.Error == "" {
				display = cell.DisplayText
			} else if cell != nil && cell.Error != "" {
				display = cell.Error
			}
			
			// Calculate how much space we actually have for this cell
			actualWidth := width
			textToPrint := display
			
			// Handle text overflow for text cells
			if cell != nil && cell.IsText {
				// Check if text is longer than cell width
				if len(display) > width {
					// Calculate total available space by checking empty adjacent cells
					totalSpace := width
					checkCol := col + 1
					
					// Look ahead to find empty cells
					for checkCol < s.leftCol+ScreenCols && checkCol < MaxCols {
						if !s.isCellEmpty(row, checkCol) {
							// Hit a non-empty cell, stop
							break
						}
						totalSpace += s.GetColWidth(checkCol) + 1 // +1 for space separator
						checkCol++
					}
					
					// Set skip marker so we don't print those columns
					skipUntilCol = checkCol
					
					// Truncate text to available space
					if len(display) > totalSpace {
						textToPrint = display[:totalSpace]
					} else {
						textToPrint = display
					}
					actualWidth = totalSpace
				}
			}
			
			// Print cell with cursor indicator
			if row == s.currentRow && col == s.currentCol {
				fmt.Print("►")
				if actualWidth > 1 {
					if cell != nil && cell.IsText {
						fmt.Print(padRight(textToPrint, actualWidth-1))
					} else {
						fmt.Print(padLeft(textToPrint, actualWidth-1))
					}
				}
			} else {
				if cell != nil && cell.IsText {
					fmt.Print(padRight(textToPrint, actualWidth))
				} else {
					fmt.Print(padLeft(textToPrint, actualWidth))
				}
			}
			
			// Only print space separator if we're not at the last visible column
			// and this isn't an overflow situation
			if col < s.leftCol+ScreenCols-1 && col < MaxCols-1 {
				if skipUntilCol <= col+1 {
					fmt.Print(" ")
				}
			}
		}
		fmt.Println()
	}
	
	fmt.Println(strings.Repeat("─", 84))
	currentCellRef := GetCellRef(s.currentRow, s.currentCol)
	currentCell := s.GetCell(s.currentRow, s.currentCol)
	fmt.Printf("Cell: %s | Mode: %s", currentCellRef, s.mode)
	if currentCell != nil && currentCell.Formula != "" {
		displayFormula := currentCell.Formula
		if len(displayFormula) > 40 {
			displayFormula = displayFormula[:37] + "..."
		}
		fmt.Printf(" | Content: %s", displayFormula)
	}
	fmt.Println()
	fmt.Println("Navigation: w=UP s=DOWN a=LEFT d=RIGHT | Commands: /S /L /Q /H /C /E")
	fmt.Print("> ")
}

func (s *Spreadsheet) AdjustView() {
	if s == nil {
		return
	}
	if s.currentRow < 1 {
		s.currentRow = 1
	}
	if s.currentRow > MaxRows {
		s.currentRow = MaxRows
	}
	if s.currentCol < 0 {
		s.currentCol = 0
	}
	if s.currentCol >= MaxCols {
		s.currentCol = MaxCols - 1
	}
	if s.currentRow < s.topRow {
		s.topRow = s.currentRow
	}
	if s.currentRow >= s.topRow+ScreenRows {
		s.topRow = s.currentRow - ScreenRows + 1
	}
	if s.currentCol < s.leftCol {
		s.leftCol = s.currentCol
	}
	if s.currentCol >= s.leftCol+ScreenCols {
		s.leftCol = s.currentCol - ScreenCols + 1
	}
	if s.topRow < 1 {
		s.topRow = 1
	}
	if s.leftCol < 0 {
		s.leftCol = 0
	}
}

func (s *Spreadsheet) SaveVC(filename string) error {
	if s == nil {
		return fmt.Errorf("invalid spreadsheet")
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "# VisiCalc File v%s\n", Version)
	fmt.Fprintf(writer, "# Created: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	if s.cells != nil {
		for row := 1; row <= MaxRows; row++ {
			for col := 0; col < MaxCols; col++ {
				ref := GetCellRef(row, col)
				if ref == "" {
					continue
				}
				if cell, exists := s.cells[ref]; exists && cell != nil && cell.Formula != "" {
					fmt.Fprintf(writer, "C:%s:%s\n", ref, cell.Formula)
				}
			}
		}
	}
	writer.Flush()
	return nil
}

func (s *Spreadsheet) LoadVC(filename string) error {
	if s == nil {
		return fmt.Errorf("invalid spreadsheet")
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	s.cells = make(map[string]*Cell)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		if parts[0] == "C" {
			row, col, err := ParseCellRef(parts[1])
			if err == nil {
				s.SetCell(row, col, parts[2])
			}
		}
	}
	s.Recalculate()
	return nil
}

func (s *Spreadsheet) ExportCSV(filename string) error {
	if s == nil {
		return fmt.Errorf("invalid spreadsheet")
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	maxRow := 1
	maxCol := 0
	for row := 1; row <= MaxRows; row++ {
		for col := 0; col < MaxCols; col++ {
			ref := GetCellRef(row, col)
			if cell, exists := s.cells[ref]; exists && cell != nil && cell.Formula != "" {
				if row > maxRow {
					maxRow = row
				}
				if col > maxCol {
					maxCol = col
				}
			}
		}
	}
	for row := 1; row <= maxRow; row++ {
		for col := 0; col <= maxCol; col++ {
			cell := s.GetCell(row, col)
			if cell != nil && cell.Formula != "" {
				if cell.IsText {
					fmt.Fprintf(writer, "\"%s\"", strings.ReplaceAll(cell.DisplayText, "\"", "\"\""))
				} else {
					fmt.Fprintf(writer, "%s", cell.DisplayText)
				}
			}
			if col < maxCol {
				fmt.Fprint(writer, ",")
			}
		}
		fmt.Fprintln(writer)
	}
	writer.Flush()
	return nil
}

func sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.TrimSpace(filename)
	return filename
}

func ShowHelp() {
	clearScreen()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                            VisiCalc - Help Screen                              ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("NAVIGATION (Type letter then press ENTER):")
	fmt.Println("  w - Move UP       s - Move DOWN       a - Move LEFT       d - Move RIGHT")
	fmt.Println()
	fmt.Println("ENTERING DATA:")
	fmt.Println("  123       - Enter number")
	fmt.Println("  \"Hello   - Enter text label (starts with \")")
	fmt.Println("            Text will overflow into adjacent empty cells")
	fmt.Println("  +A1+B1    - Enter formula (starts with +)")
	fmt.Println("  --        - Repeating dash line")
	fmt.Println("  ==        - Repeating equal line")
	fmt.Println()
	fmt.Println("IMPORTANT: Use THREE DOTS (...) not ellipsis (…) in ranges!")
	fmt.Println("  CORRECT:   +@SUM(A1...A3)")
	fmt.Println("  Type: + @ S U M ( A 1 . . . A 3 )")
	fmt.Println()
	fmt.Println("STATISTICAL FUNCTIONS:")
	fmt.Println("  +@SUM(A1...A10)     - Sum of range")
	fmt.Println("  +@AVG(A1...A10)     - Average of range")
	fmt.Println("  +@MIN(A1...A10)     - Minimum value in range")
	fmt.Println("  +@MAX(A1...A10)     - Maximum value in range")
	fmt.Println("  +@COUNT(A1...A10)   - Count non-empty cells")
	fmt.Println()
	fmt.Println("MATHEMATICAL FUNCTIONS:")
	fmt.Println("  +@SQRT(A1)          - Square root")
	fmt.Println("  +@ABS(A1)           - Absolute value")
	fmt.Println("  +@INT(A1)           - Integer part (floor)")
	fmt.Println("  +@ROUND(A1)         - Round to nearest integer")
	fmt.Println("  +@EXP(A1)           - e^x (exponential)")
	fmt.Println("  +@LN(A1)            - Natural logarithm")
	fmt.Println("  +@LOG(A1)           - Base-10 logarithm")
	fmt.Println("  +@PI()              - Pi constant (3.14159...)")
	fmt.Println()
	fmt.Println("TRIGONOMETRIC FUNCTIONS:")
	fmt.Println("  +@SIN(A1)           - Sine (radians)")
	fmt.Println("  +@COS(A1)           - Cosine (radians)")
	fmt.Println("  +@TAN(A1)           - Tangent (radians)")
	fmt.Println()
	fmt.Println("CONDITIONAL FUNCTION:")
	fmt.Println("  +@IF(A1>10,100,50)  - If A1>10 then 100 else 50")
	fmt.Println("  Operators: = < > >= <= != <>")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  /S filename  - Save to file (.vc extension added automatically)")
	fmt.Println("  /L filename  - Load from file")
	fmt.Println("  /E filename  - Export to CSV (.csv extension added automatically)")
	fmt.Println("  /C           - Clear current cell")
	fmt.Println("  /Q           - Quit")
	fmt.Println("  /H           - Show this help")
	fmt.Println()
	fmt.Println("Press ENTER to continue...")
}

func main() {
	spreadsheet := NewSpreadsheet()
	reader := bufio.NewReader(os.Stdin)
	message := "Welcome to VisiCalc! Type /H for help. Use w/a/s/d to navigate."
	
	for {
		spreadsheet.Display()
		if message != "" {
			fmt.Println("Status:", message)
			message = ""
		}
		
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		
		input = strings.TrimSpace(input)
		
		if input == "" {
			continue
		}
		
		inputLower := strings.ToLower(input)
		
		if inputLower == "w" {
			if spreadsheet.currentRow > 1 {
				spreadsheet.currentRow--
				spreadsheet.AdjustView()
			}
			continue
		}
		
		if inputLower == "s" {
			if spreadsheet.currentRow < MaxRows {
				spreadsheet.currentRow++
				spreadsheet.AdjustView()
			}
			continue
		}
		
		if inputLower == "a" {
			if spreadsheet.currentCol > 0 {
				spreadsheet.currentCol--
				spreadsheet.AdjustView()
			}
			continue
		}
		
		if inputLower == "d" {
			if spreadsheet.currentCol < MaxCols-1 {
				spreadsheet.currentCol++
				spreadsheet.AdjustView()
			}
			continue
		}
		
		if len(input) > 0 && (input[0] == '/' || input[0] == ':') {
			cmd := strings.TrimSpace(input[1:])
			cmdUpper := strings.ToUpper(cmd)
			
			if cmdUpper == "Q" || cmdUpper == "QUIT" {
				clearScreen()
				fmt.Println("Thank you for using VisiCalc!")
				return
			}
			
			if cmdUpper == "H" || cmdUpper == "HELP" {
				ShowHelp()
				reader.ReadString('\n')
				continue
			}
			
			if cmdUpper == "C" || cmdUpper == "CLEAR" {
				spreadsheet.SetCell(spreadsheet.currentRow, spreadsheet.currentCol, "")
				spreadsheet.Recalculate()
				message = "Cell cleared"
				continue
			}
			
			if strings.HasPrefix(cmdUpper, "S ") {
				parts := strings.Fields(cmd)
				if len(parts) >= 2 {
					filename := sanitizeFilename(parts[1])
					if !strings.HasSuffix(filename, ".vc") {
						filename += ".vc"
					}
					err := spreadsheet.SaveVC(filename)
					if err != nil {
						message = "Error saving: " + err.Error()
					} else {
						spreadsheet.filename = filename
						spreadsheet.modified = false
						message = "Saved to " + filename
					}
				} else {
					message = "Usage: /S filename"
				}
				continue
			}
			
			if strings.HasPrefix(cmdUpper, "L ") {
				parts := strings.Fields(cmd)
				if len(parts) >= 2 {
					filename := sanitizeFilename(parts[1])
					if !strings.HasSuffix(filename, ".vc") {
						filename += ".vc"
					}
					err := spreadsheet.LoadVC(filename)
					if err != nil {
						message = "Error loading: " + err.Error()
					} else {
						spreadsheet.filename = filename
						spreadsheet.modified = false
						message = "Loaded from " + filename
					}
				} else {
					message = "Usage: /L filename"
				}
				continue
			}
			
			if strings.HasPrefix(cmdUpper, "E ") {
				parts := strings.Fields(cmd)
				if len(parts) >= 2 {
					filename := sanitizeFilename(parts[1])
					if !strings.HasSuffix(filename, ".csv") {
						filename += ".csv"
					}
					err := spreadsheet.ExportCSV(filename)
					if err != nil {
						message = "Error exporting: " + err.Error()
					} else {
						message = "Exported to " + filename
					}
				} else {
					message = "Usage: /E filename"
				}
				continue
			}
			
			message = "Unknown command. Type /H for help."
			continue
		}
		
		spreadsheet.SetCell(spreadsheet.currentRow, spreadsheet.currentCol, input)
		spreadsheet.Recalculate()
		cellRef := GetCellRef(spreadsheet.currentRow, spreadsheet.currentCol)
		message = "Cell " + cellRef + " updated"
		
		if spreadsheet.currentRow < MaxRows {
			spreadsheet.currentRow++
			spreadsheet.AdjustView()
		}
	}
}