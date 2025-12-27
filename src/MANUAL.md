VisiCalc for Go - Complete User Manual
Table of Contents

Introduction
Getting Started
Navigation
Data Entry
Formulas and Functions
Commands
Examples and Tutorials
Tips and Tricks
Troubleshooting


Introduction
VisiCalc for Go is a terminal-based spreadsheet application inspired by the original VisiCalc from 1979. It provides powerful calculation capabilities, formula support, and file management features—all from your command line.
Key Features

254 rows × 26 columns (A-Z)
Real-time formula calculation
Statistical, mathematical, and trigonometric functions
File save/load functionality
CSV export capability
Circular reference detection
Error handling


Getting Started
Installation

Compile the program:
go build -o vc main.go


Run VisiCalc:
./vc



Screen Layout
╔════════════════════════════════════════════════════════════════════════════════╗
║                         VisiCalc (Go Edition) v1.0                             ║
╚════════════════════════════════════════════════════════════════════════════════╝
     A         B         C         D         E         F         G         H         I    
    ───────── ───────── ───────── ───────── ───────── ───────── ───────── ───────── ─────────
  1│►                                                                                        
  2│                                                                                         
  3│                                                                                         
────────────────────────────────────────────────────────────────────────────────
Cell: A1 | Mode: READY
Navigation: w=UP s=DOWN a=LEFT d=RIGHT | Commands: /S /L /Q /H /C /E
> 

Components:

Header: Shows the application name and version
Column Headers: Letters A-Z indicating columns
Row Numbers: Numbers 1-254 on the left
Current Cell Indicator: ► symbol shows selected cell
Status Bar: Displays current cell reference and mode
Command Line: Shows available commands
Input Prompt: > where you type commands and data


Navigation
Basic Movement
Type a single letter followed by ENTER:



Key
Action



w
Move UP one row


s
Move DOWN one row


a
Move LEFT one column


d
Move RIGHT one column


Example:
> w [ENTER]     # Moves to row above
> s [ENTER]     # Moves to row below
> a [ENTER]     # Moves to column on left
> d [ENTER]     # Moves to column on right

Screen Scrolling
The display shows 20 rows and 9 columns at a time. When you navigate beyond the visible area, the screen automatically scrolls to keep the current cell in view.

Data Entry
Entering Numbers
Simply type the number and press ENTER:
> 123 [ENTER]           # Enters 123
> 3.14159 [ENTER]       # Enters 3.14159
> -42 [ENTER]           # Enters -42
> 1.5e6 [ENTER]         # Enters 1500000 (scientific notation)

Display: Numbers are right-aligned in cells.
Entering Text Labels
Start with a double quote ("):
> "Sales [ENTER]        # Enters "Sales"
> "Total Revenue [ENTER] # Enters "Total Revenue"
> "Q1 2024 [ENTER]      # Enters "Q1 2024"

Display: Text is left-aligned in cells.
Entering Formulas
Start with a plus sign (+):
> +A1+B1 [ENTER]        # Adds A1 and B1
> +A1*1.15 [ENTER]      # Multiplies A1 by 1.15
> +@SUM(A1...A10) [ENTER] # Sums cells A1 through A10

Display: Shows the calculated result, not the formula.
Special Text Formatting
Repeating Lines:
> -- [ENTER]            # Creates a line of dashes
> --= [ENTER]           # Creates a line of equal signs
> == [ENTER]            # Creates a line of equal signs

These expand to fill the column width.

Formulas and Functions
Formula Syntax
All formulas must start with +:
+A1+B1              # Addition
+A1-B1              # Subtraction
+A1*B1              # Multiplication
+A1/B1              # Division
+A1^2               # Power (A1 squared)
+(A1+B1)*C1         # Parentheses for order of operations

Cell References
Reference other cells by their column letter and row number:
+A1                 # Value from cell A1
+B5                 # Value from cell B5
+Z254               # Value from cell Z254

Cell references are case-insensitive: A1, a1, and A1 are all the same.
Arithmetic Operators



Operator
Operation
Example
Result (if A1=10, B1=3)



+
Addition
+A1+B1
13


-
Subtraction
+A1-B1
7


*
Multiplication
+A1*B1
30


/
Division
+A1/B1
3.333333


^
Power
+A1^B1
1000


()
Parentheses
+(A1+B1)*2
26


Operator Precedence

Parentheses ()
Exponentiation ^
Multiplication * and Division /
Addition + and Subtraction -

Example:
+2+3*4              # Result: 14 (not 20)
+(2+3)*4            # Result: 20
+2^3*4              # Result: 32 (8*4)


Statistical Functions
@SUM - Sum of Range
Adds all numbers in a range.
Syntax: +@SUM(start...end)
Examples:
+@SUM(A1...A10)     # Sum of A1 through A10
+@SUM(B5...B20)     # Sum of B5 through B20
+@SUM(A1...C1)      # Sum of A1, B1, C1 (horizontal)

@AVG - Average of Range
Calculates the arithmetic mean.
Syntax: +@AVG(start...end)
Examples:
+@AVG(A1...A10)     # Average of A1 through A10
+@AVG(C1...C100)    # Average of C1 through C100

@MIN - Minimum Value
Finds the smallest number in a range.
Syntax: +@MIN(start...end)
Examples:
+@MIN(A1...A10)     # Smallest value in A1-A10
+@MIN(B1...Z1)      # Smallest value in row 1

@MAX - Maximum Value
Finds the largest number in a range.
Syntax: +@MAX(start...end)
Examples:
+@MAX(A1...A10)     # Largest value in A1-A10
+@MAX(B1...Z1)      # Largest value in row 1

@COUNT - Count Cells
Counts non-empty numeric cells in a range.
Syntax: +@COUNT(start...end)
Examples:
+@COUNT(A1...A10)   # Number of filled cells in A1-A10
+@COUNT(B1...B100)  # Number of filled cells in B1-B100

Note: Text cells are not counted.

Mathematical Functions
@SQRT - Square Root
Syntax: +@SQRT(value)
Examples:
+@SQRT(16)          # Result: 4
+@SQRT(A1)          # Square root of A1
+@SQRT(A1^2+B1^2)   # Pythagorean theorem

Error: Returns "SQRT NEG" if value is negative.
@ABS - Absolute Value
Syntax: +@ABS(value)
Examples:
+@ABS(-5)           # Result: 5
+@ABS(A1-B1)        # Absolute difference

@INT - Integer Part (Floor)
Syntax: +@INT(value)
Examples:
+@INT(3.7)          # Result: 3
+@INT(-2.3)         # Result: -3 (floor)
+@INT(A1/B1)        # Integer division

@ROUND - Round to Nearest Integer
Syntax: +@ROUND(value)
Examples:
+@ROUND(3.7)        # Result: 4
+@ROUND(3.4)        # Result: 3
+@ROUND(A1*1.15)    # Round result

@EXP - Exponential (e^x)
Syntax: +@EXP(value)
Examples:
+@EXP(1)            # Result: 2.718281828... (e)
+@EXP(0)            # Result: 1
+@EXP(A1)           # e raised to A1

@LN - Natural Logarithm
Syntax: +@LN(value)
Examples:
+@LN(2.718281828)   # Result: 1
+@LN(10)            # Result: 2.302585
+@LN(A1)            # Natural log of A1

Error: Returns "LN NEG" if value ≤ 0.
@LOG - Base-10 Logarithm
Syntax: +@LOG(value)
Examples:
+@LOG(100)          # Result: 2
+@LOG(1000)         # Result: 3
+@LOG(A1)           # Log base 10 of A1

Error: Returns "LOG NEG" if value ≤ 0.
@PI - Pi Constant
Syntax: +@PI()
Examples:
+@PI()              # Result: 3.141592653589793
+2*@PI()*A1         # Circumference (A1 = radius)
+@PI()*A1^2         # Area of circle (A1 = radius)


Trigonometric Functions
Note: All angles are in radians, not degrees.
@SIN - Sine
Syntax: +@SIN(radians)
Examples:
+@SIN(0)            # Result: 0
+@SIN(@PI()/2)      # Result: 1 (90 degrees)
+@SIN(@PI()/4)      # Result: 0.707... (45 degrees)

@COS - Cosine
Syntax: +@COS(radians)
Examples:
+@COS(0)            # Result: 1
+@COS(@PI())        # Result: -1 (180 degrees)
+@COS(@PI()/3)      # Result: 0.5 (60 degrees)

@TAN - Tangent
Syntax: +@TAN(radians)
Examples:
+@TAN(0)            # Result: 0
+@TAN(@PI()/4)      # Result: 1 (45 degrees)

Converting Degrees to Radians
Use the formula: radians = degrees × π / 180
Example:
+@SIN(A1*@PI()/180)     # Sine of A1 degrees
+@COS(45*@PI()/180)     # Cosine of 45 degrees


Conditional Function
@IF - If-Then-Else
Syntax: +@IF(condition, true_value, false_value)
Comparison Operators:

= Equal to
< Less than
> Greater than
<= Less than or equal
>= Greater than or equal
!= or <> Not equal

Examples:
+@IF(A1>100,1,0)            # 1 if A1>100, else 0
+@IF(A1=B1,A1,B1)           # Return A1 if equal, else B1
+@IF(A1>=50,A1*0.9,A1)      # 10% discount if A1>=50
+@IF(A1<0,0,A1)             # Prevent negative values
+@IF(A1>B1,A1,B1)           # Maximum of A1 and B1

Nested IF (complex):
+@IF(A1>90,@IF(A1>95,100,95),@IF(A1>80,85,75))


Commands
All commands start with / or : followed by a command letter.
/S - Save File
Saves the spreadsheet to a file.
Syntax: /S filename
Examples:
> /S budget [ENTER]         # Saves as budget.vc
> /S report2024 [ENTER]     # Saves as report2024.vc
> /S mydata.vc [ENTER]      # Saves as mydata.vc

Notes:

.vc extension is added automatically if not provided
Overwrites existing files without warning
Saves all cells with content

/L - Load File
Loads a spreadsheet from a file.
Syntax: /L filename
Examples:
> /L budget [ENTER]         # Loads budget.vc
> /L report2024 [ENTER]     # Loads report2024.vc
> /L mydata.vc [ENTER]      # Loads mydata.vc

Notes:

.vc extension is added automatically if not provided
Clears current spreadsheet before loading
Recalculates all formulas after loading

/E - Export to CSV
Exports the spreadsheet to CSV format.
Syntax: /E filename
Examples:
> /E data [ENTER]           # Exports as data.csv
> /E report [ENTER]         # Exports as report.csv
> /E output.csv [ENTER]     # Exports as output.csv

Notes:

.csv extension is added automatically if not provided
Text fields are quoted
Compatible with Excel, Google Sheets, etc.

/C - Clear Cell
Clears the current cell.
Syntax: /C
Example:
> /C [ENTER]                # Clears current cell

Note: Also recalculates dependent formulas.
/H - Help
Shows the help screen.
Syntax: /H
Example:
> /H [ENTER]                # Shows help
[Press ENTER to return]

/Q - Quit
Exits VisiCalc.
Syntax: /Q
Example:
> /Q [ENTER]                # Exits program

Warning: Does not prompt to save! Save your work first with /S.

Examples and Tutorials
Example 1: Simple Budget
Goal: Create a monthly budget tracker.
Steps:

Navigate to A1 and enter labels:
A1: "Item
B1: "Amount


Enter budget items:
A2: "Rent
B2: 1200
A3: "Food
B3: 400
A4: "Transport
B4: 150
A5: "Utilities
B5: 200


Add total in B6:
B6: +@SUM(B2...B5)


Add a separator in A7:
A7: --



Result:
     A         B    
    ───────── ─────
  1│Item      Amount
  2│Rent        1200
  3│Food         400
  4│Transport    150
  5│Utilities    200
  6│           1950
  7│─────────      

Example 2: Sales Tax Calculator
Goal: Calculate price with tax.
Setup:
A1: "Base Price
B1: 100

A2: "Tax Rate
B2: 0.08

A3: "Tax Amount
B3: +B1*B2

A4: "Total Price
B4: +B1+B3

Result:
     A              B    
    ────────────── ─────
  1│Base Price      100
  2│Tax Rate       0.08
  3│Tax Amount        8
  4│Total Price     108

Example 3: Grade Calculator
Goal: Calculate student grades with letter grades.
Setup:
A1: "Student
B1: "Score
C1: "Grade

A2: "Alice
B2: 92
C2: +@IF(B2>=90,"A",@IF(B2>=80,"B",@IF(B2>=70,"C","F")))

A3: "Bob
B3: 78
C3: +@IF(B3>=90,"A",@IF(B3>=80,"B",@IF(B3>=70,"C","F")))

Note: This uses nested IF statements. For text output, you'd need to use numeric codes (90=A, 80=B, etc.) since VisiCalc works with numbers.
Better approach:
C2: +@IF(B2>=90,90,@IF(B2>=80,80,@IF(B2>=70,70,0)))

Then interpret: 90=A, 80=B, 70=C, 0=F
Example 4: Loan Calculator
Goal: Calculate monthly payment.
Setup:
A1: "Loan Amount
B1: 10000

A2: "Annual Rate %
B2: 5

A3: "Years
B3: 3

A4: "Monthly Rate
B4: +B2/12/100

A5: "Num Payments
B5: +B3*12

A6: "Monthly Payment
B6: +B1*B4/(1-(1+B4)^(-B5))

Result: Shows monthly payment for a $10,000 loan at 5% for 3 years.
Example 5: Temperature Converter
Goal: Convert Celsius to Fahrenheit.
Setup:
A1: "Celsius
B1: "Fahrenheit

A2: 0
B2: +A2*9/5+32

A3: 10
B3: +A3*9/5+32

A4: 20
B4: +A4*9/5+32

A5: 30
B5: +A5*9/5+32

Result:
     A         B    
    ───────── ─────
  1│Celsius   Fahrenheit
  2│0              32
  3│10             50
  4│20             68
  5│30             86

Example 6: Circle Calculations
Goal: Calculate circle properties from radius.
Setup:
A1: "Radius
B1: 5

A2: "Diameter
B2: +B1*2

A3: "Circumference
B3: +2*@PI()*B1

A4: "Area
B4: +@PI()*B1^2

Result:
     A              B    
    ────────────── ─────────
  1│Radius              5
  2│Diameter            10
  3│Circumference   31.415927
  4│Area            78.539816

Example 7: Statistics Summary
Goal: Analyze a dataset.
Setup:
A1: "Data
A2: 23
A3: 45
A4: 12
A5: 67
A6: 34
A7: 89
A8: 56

B1: "Count
C1: +@COUNT(A2...A8)

B2: "Sum
C2: +@SUM(A2...A8)

B3: "Average
C3: +@AVG(A2...A8)

B4: "Min
C4: +@MIN(A2...A8)

B5: "Max
C5: +@MAX(A2...A8)

B6: "Range
C6: +C5-C4


Tips and Tricks
1. Quick Navigation

Use w and s to move vertically through data
Use a and d to move horizontally
The screen scrolls automatically

2. Formula Copying Pattern
To copy a formula down:

Enter formula in first cell
Navigate to next cell
Re-type formula with updated cell references

Example:
A1: 10
A2: 20
B1: +A1*2      # Result: 20
B2: +A2*2      # Result: 40 (manually update reference)

3. Using Ranges Efficiently
Ranges use ... notation:
+@SUM(A1...A100)    # More efficient than +A1+A2+A3+...

4. Avoiding Circular References
Bad:
A1: +A1+1           # ERROR: CIRCULAR

Good:
A1: 10
A2: +A1+1           # Result: 11

5. Percentage Calculations
A1: 100             # Original value
A2: 15              # Percentage
A3: +A1*A2/100      # Result: 15 (15% of 100)
A4: +A1+A3          # Result: 115 (add 15%)
A5: +A1*1.15        # Result: 115 (shortcut)

6. Compound Formulas
Build complex calculations step by step:
A1: 100             # Principal
A2: 0.05            # Rate
A3: 10              # Years
A4: +A1*(1+A2)^A3   # Compound interest

7. Data Validation with @IF
Prevent invalid entries:
A1: -5
A2: +@IF(A1<0,0,A1)     # Result: 0 (no negatives)

8. Creating Tables
Use repeating lines for visual separation:
A1: "Header
A2: --
A3: Data
A4: Data
A5: --
A6: "Total

9. Saving Regularly
Save your work frequently:
> /S mywork [ENTER]

No auto-save feature—you must save manually!
10. Exporting for Analysis
Export to CSV for use in other tools:
> /E analysis [ENTER]

Then open analysis.csv in Excel, Google Sheets, etc.

Troubleshooting
Error Messages



Error
Cause
Solution



CIRCULAR
Formula references itself
Check formula dependencies


DIV/0
Division by zero
Ensure divisor is not zero


SYNTAX
Invalid formula syntax
Check formula format (starts with +)


SQRT NEG
Square root of negative
Ensure value is positive


LN NEG
Log of negative/zero
Ensure value is positive


LOG NEG
Log of negative/zero
Ensure value is positive


TOO DEEP
Formula nesting too deep
Simplify formula


OVERFLOW
Number too large
Use smaller numbers


PAREN
Mismatched parentheses
Check ( and ) pairs


Common Problems
Problem: Navigation keys don't work
Solution: Make sure you're typing lowercase w, a, s, d followed by ENTER. Arrow keys are not supported.

Problem: Formula shows as text
Solution: Formulas must start with +. Check that you typed + before the formula.

Problem: Can't see my data
Solution: Use navigation keys to scroll. The display shows only 20 rows × 9 columns at a time.

Problem: File won't load
Solution: 

Check filename spelling
Ensure file has .vc extension
File must be in the same directory as the program


Problem: Formula returns wrong result
Solution:

Check operator precedence (use parentheses)
Verify cell references
Check for circular references


Problem: Lost my work
Solution:

VisiCalc has no auto-save
Always use /S filename to save before /Q to quit
Save frequently during work


Problem: Number displays as "ERROR"
Solution:

Number is too large (overflow)
Result is NaN (Not a Number)
Check formula for invalid operations


Performance Tips
Large Ranges:

Ranges over 10,000 cells may be slow
Break into smaller calculations if possible

Complex Formulas:

Deeply nested formulas (>100 levels) will error
Simplify by using intermediate cells

File Size:

Only cells with content are saved
Empty cells don't increase file size


Keyboard Reference Card
┌─────────────────────────────────────────────────┐
│           VISICALC QUICK REFERENCE              │
├─────────────────────────────────────────────────┤
│ NAVIGATION                                      │
│   w - Move UP        s - Move DOWN              │
│   a - Move LEFT      d - Move RIGHT             │
├─────────────────────────────────────────────────┤
│ DATA ENTRY                                      │
│   123        - Number                           │
│   "Text      - Label                            │
│   +A1+B1     - Formula                          │
│   --         - Line                             │
├─────────────────────────────────────────────────┤
│ COMMANDS                                        │
│   /S file    - Save                             │
│   /L file    - Load                             │
│   /E file    - Export CSV                       │
│   /C         - Clear cell                       │
│   /H         - Help                             │
│   /Q         - Quit                             │
├─────────────────────────────────────────────────┤
│ OPERATORS                                       │
│   + - * / ^  - Arithmetic                       │
│   ( )        - Parentheses                      │
├─────────────────────────────────────────────────┤
│ FUNCTIONS                                       │
│   @SUM @AVG @MIN @MAX @COUNT                    │
│   @SQRT @ABS @INT @ROUND                        │
│   @SIN @COS @TAN @PI                            │
│   @LN @LOG @EXP                                 │
│   @IF(cond,true,false)                          │
└─────────────────────────────────────────────────┘


File Format
VisiCalc files (.vc) are plain text:
# VisiCalc File v1.0
# Created: 2024-01-15 10:30:00
C:A1:"Sales
C:B1:1000
C:B2:2000
C:B3:+B1+B2

Format: C:CellRef:Content
You can edit these files manually if needed!

Conclusion
VisiCalc for Go brings the power of spreadsheet calculations to your terminal. With its comprehensive function library and simple interface, you can perform complex calculations, manage budgets, analyze data, and more—all without leaving the command line.
Remember:

Save frequently with /S
Use /H for quick help
Start formulas with +
Navigate with w, a, s, d

Happy calculating!
