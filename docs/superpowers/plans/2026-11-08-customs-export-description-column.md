# Customs Export: Item Description in the Comments Column Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show each item's description in column K "Bemerkungen / comments" of the customs inventory sheet, with its line breaks and with bold, italic and underline as Excel formatting, and move the texts the export writes itself into a new column L "Prozesshinweise / process comments".

**Architecture:** The pure package gets `Item.Description`; `Build` cleans it up per entry (line breaks kept) and collects the members' descriptions into a new `Line.Descriptions`. The existing `Line.Comments` is renamed `Line.ProcessComments`, so the field name matches its new column. A small parser turns one line of description into formatted spans. `WriteXLSX` builds column K from those spans (plain string or rich text, wrapped, cut to Excel's cell limit) and writes `ProcessComments` in L. The service maps `Description` from the entity.

**Tech Stack:** Go (testify, excelize v2.11.0).

**Spec:** [`docs/superpowers/specs/2026-11-08-customs-export-description-column-design.md`](../specs/2026-11-08-customs-export-description-column-design.md)

## Global Constraints

- No schema, migration, API, swagger, TS type or locale change, and no new dependency. Don't run `task generate`; if anything under `backend/app/api/static/docs/`, `docs/public/api/`, `frontend/` or `go.mod` changes, stop, the plan is wrong.
- Column L header, exactly: `Prozesshinweise / process comments`. Column K keeps `Bemerkungen / comments`.
- Only the entity's `Description` is exported, never `Notes` or custom fields.
- Both K and L join their parts with `"; "`. Inside K, the lines of one description are joined with `"\n"`.
- The cell limit is `excelize.TotalCellChars` (32,767 UTF-16 units). Count with `unicode/utf16.RuneLen`; never split a rune.
- Underline is `excelize.Font{Underline: "single"}`.
- `golden_report.json` changes only in Task 1, and only by the key rename `"Comments"` → `"ProcessComments"` and one new `"Descriptions": null` per line (the golden items have no description). Regenerate it with `-update` and check the diff says exactly that.
- For a scoped Go run use `cd backend && go test <pkg>`; `task go:test` always runs every package.
- `task go:lint` clean before the PR. gocyclo: keep the parser's marker checks in small helpers rather than one long function; don't add branches to `lineFrom` or `Build`.
- Tests never use the local dev database.
- **PR slicing.** One PR with Tasks 1–4, against `main`. The body follows the repo's PR conventions (persona first line, signature).

## Review Focus

1. **A description that starts with `=`, `+`, `-` or `@`, plain or formatted.** Expected: inline text, never a formula (FR-16, FR-49). Test: Task 3, `TestWriteXLSX_TextNeverFormula` (Dining chair plain, Shelf rich).
2. **Markup look-alikes in ordinary text** (`snake_case_name`, `2*3*4`, `2 * 3`, a `* ` list line, an unclosed `**`, `\*x\*`, `****x****`). Expected: stay exactly as written, no formatting (FR-50). Test: Task 2, `TestParseMarkup`.
3. **Windows line breaks, blank lines and spaces-only descriptions.** Expected: one `\n` between non-empty lines, nothing for spaces only. Tests: Task 1, `TestBuild_Descriptions`; Task 4, `TestCustomsInventory_ExportDescriptionColumns` (through the real database).
4. **A long merged line of formatted descriptions, including characters outside the BMP.** Expected: the export succeeds, K is at most 32,767 UTF-16 units and no surrogate pair is split (FR-49). Tests: Task 3, `TestWriteXLSX_LongDescriptionsCut` and `TestWriteXLSX_LongRichDescriptionsCut`.
5. **A container with rolled-in contents that have descriptions.** Expected: only the container's own description in K; L still ends with "inkl. Inhalt (7 Stk.)". Test: Task 1, `TestBuild_RollUp_ContentDescriptionsNotRolledIn`.

---

### Task 1: Descriptions on the line, process comments renamed (FR-47, FR-48)

**Files:**
- Modify: `backend/internal/core/services/reporting/customsinventory/model.go` (`Item`, `Line`)
- Modify: `backend/internal/core/services/reporting/customsinventory/build.go` (`entry`, `newEntry`, `lineFrom`, new `cleanDescription`)
- Modify: `backend/internal/core/services/reporting/customsinventory/xlsx.go` (rename only: `l.Comments` → `l.ProcessComments`, still column K in this task)
- Modify: `backend/internal/core/services/reporting/customsinventory/testdata/golden_report.json` (regenerated)
- Test: `build_test.go`, `contents_test.go`, `xlsx_test.go` (rename only) in the same package

**Interfaces:**
- Produces: field `Item.Description string` (right after `Name`); field `Line.Descriptions []string` (right after `ReceiptNames`); `Line.Comments` renamed `Line.ProcessComments []string` (right after `Descriptions`, same content as before). Unexported: `cleanDescription(s string) string` in `build.go` (FR-47 clean-up); `entry.description string`, set in `newEntry` to `cleanDescription(it.Description)`.

- [ ] **Step 1: Rename and write the failing tests**

Rename every `Comments` use on `Line` to `ProcessComments` in `build_test.go`, `contents_test.go` (including `hasComment`) and `xlsx_test.go`'s `testReport()`. Then add:

`TestBuild_Descriptions` in `build_test.go`: `in := goldenInput()`; `setItem(&in, id(101), …Description = "  Oak case,\r\n\n\t made   1890 ")` (Grandfather clock); `setItem(&in, id(102), …Description = " \r\n\t ")` (Sofa). `r := mustBuild(t, in)`. Assert:
- `lineByName(t, r, "Grandfather clock").Descriptions` equals `[]string{"Oak case,\nmade 1890"}`;
- `lineByName(t, r, "Grandfather clock").ProcessComments` is empty (duty-free by date, has a price);
- `lineByName(t, r, "Sofa").Descriptions` is empty.

`TestBuild_MergeIdentical_Descriptions` in `build_test.go`, next to `TestBuild_MergeIdentical`: `in := goldenInput()`; descriptions `id(131)` `"Tax 2019"`, `id(133)` `"Tax  2019"`, `id(135)` `"Tax 2020\n**box 2**"`. Take the merged Files line the way `TestBuild_MergeIdentical` does (`files[0]`, 14 members, sorted by asset ID). Assert `Descriptions` equals `[]string{"Tax 2019", "Tax 2020\n**box 2**"}` (markup is kept here; the sheet writer formats it) and `ProcessComments` still equals `[]string{"zollfrei lt. Tag „owned > 2 years“; Wert fehlt"}`.

`TestBuild_RollUp_ContentDescriptionsNotRolledIn` in `contents_test.go`: `in := containerInput()`; descriptions `boxID` `"Moving box 3"`, `bookID` `"Paperbacks"`, `penID` `"Blue ink"`. `l := allLines(mustBuild(t, in))[0]`. Assert `l.Descriptions` equals `[]string{"Moving box 3"}` and `l.ProcessComments` contains `"inkl. Inhalt (7 Stk.)"`.

- [ ] **Step 2: Run them and see them fail**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: build failure, `Line` has no field `ProcessComments` / `Descriptions`, `Item` has no field `Description`.

- [ ] **Step 3: Implement**

- `model.go`: add the fields as in Interfaces; rename `Comments`. Doc comments: `Descriptions // column K (FR-47), markup kept`, `ProcessComments // column L (FR-48)`.
- `build.go`: `cleanDescription` replaces `\r\n` then `\r` with `\n`, applies `strings.Fields` + one space to each line, drops empty lines, joins with `\n`. In `lineFrom`'s member loop, `l.Descriptions = appendUnique(l.Descriptions, m.description)`; the contents loop doesn't touch `Descriptions`. Rename the three `l.Comments` uses.
- `xlsx.go`: `l.Comments` → `l.ProcessComments` (still column K until Task 3).

- [ ] **Step 4: Regenerate the golden file and run the package tests**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/ -run TestBuild_Golden -update`, then `git diff` on `golden_report.json`.
Expected: the diff only renames `"Comments"` to `"ProcessComments"` and adds `"Descriptions": null`, once per line. Anything else: stop.
Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/core/services/reporting/customsinventory/
git commit -m "feat(customs): collect item descriptions per line, rename comments to process comments"
```

### Task 2: Bold, italic and underline markers (FR-50)

**Files:**
- Create: `backend/internal/core/services/reporting/customsinventory/markup.go`
- Test: `backend/internal/core/services/reporting/customsinventory/markup_test.go`

**Interfaces:**
- Produces (unexported): `type span struct { text string; bold, italic, underline bool }` and `func parseMarkup(line string) []span`. It takes one line (no `\n`), returns nil for `""`, removes matched markers, keeps everything else verbatim, and merges adjacent spans with equal formatting.

- [ ] **Step 1: Write the failing test**

`TestParseMarkup`, table-driven; `p(s)` is a plain span, `b`, `i`, `u`, `bi` set the flags:

| input | want |
|---|---|
| `""` | nil |
| `"plain text"` | `p("plain text")` |
| `"**Oak** case"` | `b("Oak"), p(" case")` |
| `"__Oak__, made"` | `b("Oak"), p(", made")` |
| `"an *old* clock"` | `p("an "), i("old"), p(" clock")` |
| `"an _old_ clock"` | `p("an "), i("old"), p(" clock")` |
| `"***both***"` | `bi("both")` |
| `"___both___"` | `bi("both")` |
| `"<u>1890</u> model"` | `u("1890"), p(" model")` |
| `"<U>1890</U>"` | `u("1890")` |
| `"**bold _and italic_**"` | `b("bold "), bi("and italic")` |
| `"**a** and **b**"` | `b("a"), p(" and "), b("b")` |
| `"[link](http://x) **b**"` | `p("[link](http://x) "), b("b")` |
| `"snake_case_name"` | `p("snake_case_name")` |
| `"2*3*4"` | `p("2*3*4")` |
| `"2 * 3 * 4"` | `p("2 * 3 * 4")` |
| `"* list item"` | `p("* list item")` |
| `"**unclosed"` | `p("**unclosed")` |
| `"**x*"` | `p("**x*")` |
| `"\\*not italic\\*"` (Go string) | `p("\\*not italic\\*")` |
| `"****x****"` | `p("****x****")` |
| `"<u>open only"` | `p("<u>open only")` |
| `"**a _b** c_"` | `b("a _b"), p(" c_")` |

- [ ] **Step 2: Run it and see it fail**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/ -run TestParseMarkup`
Expected: build failure, `parseMarkup` undefined.

- [ ] **Step 3: Implement `parseMarkup` in `markup.go`**

The spec's rules (FR-50) are the contract. One scan that satisfies them:
1. Tokenize left to right into text, `<u>`/`</u>` (case-insensitive) and `*`/`_` runs. A run of 4+ equal characters, or one preceded by `\`, is text.
2. For each run, decide `canOpen` (next character exists, isn't a space, and the previous one isn't a letter or digit) and `canClose` (previous character exists, isn't a space, and the next one isn't a letter or digit). Use `unicode.IsLetter`/`IsDigit`/`IsSpace`.
3. Keep a stack of open markers. A closer first tries to match the nearest open marker with the same character and length; on a match, open markers above it turn into text. Otherwise, if it can open, push it; else it's text. `</u>` matches the nearest `<u>` the same way.
4. Markers left on the stack at the end are text. Each matched pair sets bold (length 2), italic (length 1) or both (length 3), or underline, on the text between them.

- [ ] **Step 4: Run the package tests and see them pass**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/core/services/reporting/customsinventory/markup.go backend/internal/core/services/reporting/customsinventory/markup_test.go
git commit -m "feat(customs): read bold, italic and underline markers in descriptions"
```

### Task 3: Columns K and L on the sheet (FR-49, FR-50)

**Files:**
- Modify: `backend/internal/core/services/reporting/customsinventory/xlsx.go` (`ColumnHeaders`, `columnWidths`, `sheetStyles`, `newStyles`, `sheetWriter.line`, `WriteXLSX`'s `MergeCell`)
- Test: `backend/internal/core/services/reporting/customsinventory/xlsx_test.go`

**Interfaces:**
- Consumes: `Line.Descriptions`, `Line.ProcessComments` (Task 1); `parseMarkup`, `span` (Task 2).
- Produces: `ColumnHeaders(currency string) []string` returns 12 headers (A–L), the last `"Prozesshinweise / process comments"`. Unexported: `sheetStyles.wrap` (`Alignment{WrapText: true}`); `func (w *sheetWriter) descriptionCell(descriptions []string) any`; `func cutRuns(runs []excelize.RichTextRun, limit int) []excelize.RichTextRun`.

- [ ] **Step 1: Write the failing tests**

In `testReport()`: Dining chair gets `Descriptions: []string{"=SUM(A1:A9)"}`; the Files line gets `Descriptions: []string{"**Oak** case\nmade <u>1890</u>", "_old_"}`; Shelf gets `Descriptions: []string{"**=1+1**"}`; Espresso machine gets `Descriptions: []string{"Dual boiler", "Black"}`.

`TestWriteXLSX_LayoutAndLabels`: add `assert.Len(t, ColumnHeaders("EUR"), 12)`, `ColumnHeaders("EUR")[10]` is `"Bemerkungen / comments"`, `[11]` is `"Prozesshinweise / process comments"`; `f.GetColWidth(SheetName, "L")` is `40`; the merge end axis becomes `"L1"`.

`TestWriteXLSX_CellTypes`: espresso `K` is `"Dual boiler; Black"`, `L` is `"Kauf nach Stichtag (< 6 Monate)"` (was asserted on K). The espresso K style (`GetCellStyle` → `GetStyle`) has `Alignment.WrapText == true`; its L style has no wrap (nil `Alignment` counts as no wrap).

`TestWriteXLSX_DescriptionRichText`: the Files row's K text is `"Oak case\nmade 1890; old"`. `f.GetCellRichText` returns exactly five runs: `"Oak"` bold; `" case\nmade "` no font; `"1890"` underline `"single"`; `"; "` no font; `"old"` italic.

`TestWriteXLSX_TextNeverFormula`: add `K{chair}` → `"=SUM(A1:A9)"` and `K{shelf}` → `"=1+1"` to the map (no formula, inline string), with `chair := findRow(t, f, "C", "Dining chair")`.

`TestWriteXLSX_LongDescriptionsCut`: `r := testReport()`; Shelf (`r.Groups[1].Lines[0]`) gets 40 descriptions of `strings.Repeat("x", 1000)` and `ProcessComments: []string{"Wert fehlt"}`. `WriteXLSX(r)` returns no error; for the Shelf row, `len(cell K) == 32767` and `cell L == "Wert fehlt"`.

`TestWriteXLSX_LongRichDescriptionsCut`, two cases on Shelf, each `WriteXLSX` without error:
- 40 descriptions of `"**" + strings.Repeat("x", 1000) + "**"`: K text is 32,767 long; the last rich-text run is bold;
- one description `"**" + strings.Repeat("😀", 20000) + "**"`: K text is valid UTF-8 and its UTF-16 length is 32,766 (16,383 emoji; the 16,384th would split).

- [ ] **Step 2: Run them and see them fail**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/ -run TestWriteXLSX`
Expected: FAIL: 11 headers, merge ends at K1, espresso's K holds the process comment, no rich text in K, L empty.

- [ ] **Step 3: Implement**

- `ColumnHeaders` appends the L header; its doc comment says A–L. `columnWidths` gets a 12th entry, `40`. `newStyles` adds `wrap`. `MergeCell("A1", "L1")`.
- `line` writes `w.descriptionCell(l.Descriptions)` in K and `text(strings.Join(l.ProcessComments, "; "))` in L.
- `descriptionCell`: nil when there are no descriptions. Otherwise split each description on `\n`, run `parseMarkup` on each line, and build one list of runs with a plain `"\n"` between lines and a plain `"; "` between descriptions; merge adjacent runs with the same formatting. If no run is formatted, return `excelize.Cell{StyleID: w.st.wrap, Value: <the runs' text joined>}`; otherwise `excelize.Cell{StyleID: w.st.wrap, Value: cutRuns(runs, excelize.TotalCellChars)}`. Formatted runs get `Font{Bold, Italic, Underline: "single"}` as set; plain runs no font.
- `cutRuns` keeps runs while the UTF-16 total fits, shortens the first run that doesn't at a rune boundary, drops it if it ends up empty, and drops everything after it.

- [ ] **Step 4: Run the package tests and see them pass**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: PASS, `golden_report.json` unchanged by this task.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/core/services/reporting/customsinventory/
git commit -m "feat(customs): description in the comments column, process comments in a new column"
```

### Task 4: The service passes the description on (FR-47)

**Files:**
- Modify: `backend/internal/core/services/service_customs_inventory.go` (`toCustomsItem`)
- Test: `backend/internal/core/services/service_customs_inventory_test.go`

**Interfaces:**
- Consumes: `Item.Description` (Task 1); the K/L layout (Task 3).

- [ ] **Step 1: Write the failing test**

`TestCustomsInventory_ExportDescriptionColumns`: `f := newCustomsFixture(t)`; `tClient.Entity.UpdateOneID(f.lamp).SetDescription("**Brass**,\r\nwith shade").Save(f.ctx)`. `req := f.request(f.clock, f.lamp)`, `req.Options.IncludeReceipts = false`; export and open the workbook as in `TestCustomsInventory_ExportXLSXOnly`. Find each row by its C cell. Assert:
- "Old lamp": K text `"Brass,\nwith shade"`, its first rich-text run `"Brass"` bold; L `"zollfrei lt. Tag „owned > 2 years“"`;
- "Grandfather clock": K and L empty.

- [ ] **Step 2: Run it and see it fail**

Run: `cd backend && go test ./internal/core/services/ -run TestCustomsInventory_ExportDescriptionColumns`
Expected: FAIL, the lamp's K cell is empty.

- [ ] **Step 3: Implement**

`toCustomsItem` maps `Description: e.Description`.

- [ ] **Step 4: Run the service tests and lint**

Run: `cd backend && go test ./internal/core/services/`, then `task go:lint`.
Expected: PASS, including every existing `TestCustomsInventory_*`; lint clean.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/core/services/service_customs_inventory.go backend/internal/core/services/service_customs_inventory_test.go
git commit -m "feat(customs): pass the item description to the inventory list"
```
