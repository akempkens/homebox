# Customs Export: Item Description in the Comments Column Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show each item's description in column K "Bemerkungen / comments" of the customs inventory sheet, and move the texts the export writes itself into a new column L "Prozesshinweise / process comments".

**Architecture:** The pure package gets `Item.Description`; `Build` cleans it up per entry and collects the members' descriptions into a new `Line.Descriptions`. The existing `Line.Comments` is renamed `Line.ProcessComments`, so the field name matches its new column. `WriteXLSX` writes `Descriptions` in K and `ProcessComments` in L. The service maps `Description` from the entity.

**Tech Stack:** Go (testify, excelize v2.11.0).

**Spec:** [`docs/superpowers/specs/2026-11-08-customs-export-description-column-design.md`](../specs/2026-11-08-customs-export-description-column-design.md)

## Global Constraints

- No schema, migration, API, swagger, TS type or locale change. Don't run `task generate`; if anything under `backend/app/api/static/docs/`, `docs/public/api/` or `frontend/` changes, stop, the plan is wrong.
- Column L header, exactly: `Prozesshinweise / process comments`. Column K keeps `Bemerkungen / comments`.
- Only the entity's `Description` is exported, never `Notes` or custom fields.
- Clean-up reuses `normalizeName` (`strings.Fields` joined by one space). Both K and L join their parts with `"; "`.
- `golden_report.json` changes only by the key rename `"Comments"` → `"ProcessComments"` and one new `"Descriptions": null` per line (the golden items have no description). Regenerate it with `-update` and check the diff says exactly that.
- For a scoped Go run use `cd backend && go test <pkg>`; `task go:test` always runs every package.
- `task go:lint` clean before the PR (gocyclo: don't let `lineFrom` or `Build` grow branches; one `appendUnique` call per member is fine).
- Tests never use the local dev database.
- **PR slicing.** One PR with Tasks 1–3, against `main`. The body follows the repo's PR conventions (persona first line, signature).

## Review Focus

1. **A description that starts with `=`, `+`, `-` or `@`.** Expected: written as inline text, never a formula (FR-16, FR-49). Test: Task 2, `TestWriteXLSX_TextNeverFormula` (Dining chair, K).
2. **A multi-line description, or one of spaces only.** Expected: one line with single spaces; spaces only gives an empty cell. Tests: Task 1, `TestBuild_Descriptions`; Task 3, `TestCustomsInventory_ExportDescriptionColumns` (line break through the real database).
3. **Merged members with the same description written slightly differently ("Tax 2019" vs "Tax  2019") and with different ones.** Expected: each cleaned-up description once, in member order. Test: Task 1, `TestBuild_MergeIdentical_Descriptions`.
4. **A container with rolled-in contents that have descriptions.** Expected: only the container's own description in K; L still ends with "inkl. Inhalt (7 Stk.)". Test: Task 1, `TestBuild_RollUp_ContentDescriptionsNotRolledIn`.
5. **Joined descriptions longer than Excel's 32,767-character cell limit.** Expected: the export succeeds and the K cell is cut at 32,767 characters; L is untouched (spec call 5). Test: Task 2, `TestWriteXLSX_LongDescriptionsCut`.

---

### Task 1: Descriptions on the line, process comments renamed (FR-47, FR-48)

**Files:**
- Modify: `backend/internal/core/services/reporting/customsinventory/model.go` (`Item`, `Line`)
- Modify: `backend/internal/core/services/reporting/customsinventory/build.go` (`entry`, `newEntry`, `lineFrom`)
- Modify: `backend/internal/core/services/reporting/customsinventory/xlsx.go` (rename only: `l.Comments` → `l.ProcessComments`, still column K in this task)
- Modify: `backend/internal/core/services/reporting/customsinventory/testdata/golden_report.json` (regenerated)
- Test: `build_test.go`, `contents_test.go`, `xlsx_test.go` (rename only) in the same package

**Interfaces:**
- Produces: field `Item.Description string` (right after `Name`); field `Line.Descriptions []string` (right after `ReceiptNames`); `Line.Comments` renamed `Line.ProcessComments []string` (right after `Descriptions`, same content as before). Unexported: `entry.description string` (`normalizeName(it.Description)`, set in `newEntry`).

- [ ] **Step 1: Rename and write the failing tests**

Rename every `Comments` use on `Line` to `ProcessComments` in `build_test.go`, `contents_test.go` (including `hasComment`) and `xlsx_test.go`'s `testReport()`. Then add:

`TestBuild_Descriptions` in `build_test.go`: `in := goldenInput()`; `setItem(&in, id(101), …Description = "  Oak case,\n\tmade 1890 ")` (Grandfather clock); `setItem(&in, id(102), …Description = " \n ")` (Sofa). `r := mustBuild(t, in)`. Assert:
- `lineByName(t, r, "Grandfather clock").Descriptions` equals `[]string{"Oak case, made 1890"}`;
- `lineByName(t, r, "Grandfather clock").ProcessComments` is empty (duty-free by date, has a price);
- `lineByName(t, r, "Sofa").Descriptions` is empty.

`TestBuild_MergeIdentical_Descriptions` in `build_test.go`, next to `TestBuild_MergeIdentical`: `in := goldenInput()`; descriptions `id(131)` `"Tax 2019"`, `id(133)` `"Tax  2019"`, `id(135)` `"Tax 2020"`. Take the merged Files line the way `TestBuild_MergeIdentical` does (`files[0]`, 14 members). Assert `Descriptions` equals `[]string{"Tax 2019", "Tax 2020"}` and `ProcessComments` still equals `[]string{"zollfrei lt. Tag „owned > 2 years“; Wert fehlt"}`.

`TestBuild_RollUp_ContentDescriptionsNotRolledIn` in `contents_test.go`: `in := containerInput()`; descriptions `boxID` `"Moving box 3"`, `bookID` `"Paperbacks"`, `penID` `"Blue ink"`. `l := allLines(mustBuild(t, in))[0]`. Assert `l.Descriptions` equals `[]string{"Moving box 3"}` and `l.ProcessComments` contains `"inkl. Inhalt (7 Stk.)"`.

- [ ] **Step 2: Run them and see them fail**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: build failure, `Line` has no field `ProcessComments` / `Descriptions`, `Item` has no field `Description`.

- [ ] **Step 3: Implement**

- `model.go`: add the fields as in Interfaces; rename `Comments`. Doc comments: `Descriptions // column K (FR-47)`, `ProcessComments // column L (FR-48)`.
- `build.go`: `newEntry` sets `description`. In `lineFrom`'s member loop, `l.Descriptions = appendUnique(l.Descriptions, m.description)`; the contents loop doesn't touch `Descriptions`. Rename the three `l.Comments` uses.
- `xlsx.go`: `l.Comments` → `l.ProcessComments` (still column K until Task 2).

- [ ] **Step 4: Regenerate the golden file and run the package tests**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/ -run TestBuild_Golden -update`, then `git diff --stat` and `git diff` on `golden_report.json`.
Expected: the diff only renames `"Comments"` to `"ProcessComments"` and adds `"Descriptions": null`, once per line. Anything else: stop.
Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/core/services/reporting/customsinventory/
git commit -m "feat(customs): collect item descriptions per line, rename comments to process comments"
```

### Task 2: Columns K and L on the sheet (FR-49)

**Files:**
- Modify: `backend/internal/core/services/reporting/customsinventory/xlsx.go` (`ColumnHeaders`, `columnWidths`, `sheetWriter.line`, `WriteXLSX`'s `MergeCell`)
- Test: `backend/internal/core/services/reporting/customsinventory/xlsx_test.go`

**Interfaces:**
- Consumes: `Line.Descriptions`, `Line.ProcessComments` from Task 1.
- Produces: `ColumnHeaders(currency string) []string` returns 12 headers (A–L), the last `"Prozesshinweise / process comments"`.

- [ ] **Step 1: Write the failing tests**

In `testReport()`: Dining chair gets `Descriptions: []string{"=SUM(A1:A9)"}`; Espresso machine gets `Descriptions: []string{"Dual boiler", "Black"}`.

`TestWriteXLSX_LayoutAndLabels`: add `assert.Len(t, ColumnHeaders("EUR"), 12)`, `ColumnHeaders("EUR")[10]` is `"Bemerkungen / comments"`, `[11]` is `"Prozesshinweise / process comments"`; `f.GetColWidth(SheetName, "L")` is `40`; the merge end axis becomes `"L1"`.

`TestWriteXLSX_CellTypes`: espresso `K` is `"Dual boiler; Black"`, `L` is `"Kauf nach Stichtag (< 6 Monate)"` (was asserted on K).

`TestWriteXLSX_TextNeverFormula`: add `K{chair}` → `"=SUM(A1:A9)"` to the map (no formula, inline string), with `chair := findRow(t, f, "C", "Dining chair")`.

`TestWriteXLSX_LongDescriptionsCut`: `r := testReport()`; Shelf (`r.Groups[1].Lines[0]`) gets 40 descriptions of `strings.Repeat("x", 1000)` and `ProcessComments: []string{"Wert fehlt"}`. `WriteXLSX(r)` returns no error; for the Shelf row, `len(cell K) == 32767` and `cell L == "Wert fehlt"`.

- [ ] **Step 2: Run them and see them fail**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/ -run TestWriteXLSX`
Expected: FAIL: 11 headers, merge ends at K1, espresso's K holds the process comment, L is empty.

- [ ] **Step 3: Implement**

`ColumnHeaders` appends the L header; its doc comment says A–L. `columnWidths` gets a 12th entry, `40`. `line` writes `text(strings.Join(l.Descriptions, "; "))` in K and `text(strings.Join(l.ProcessComments, "; "))` in L. `MergeCell("A1", "L1")`. No new style: K and L stay plain text cells.

- [ ] **Step 4: Run the package tests and see them pass**

Run: `cd backend && go test ./internal/core/services/reporting/customsinventory/`
Expected: PASS, `golden_report.json` unchanged by this task.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/core/services/reporting/customsinventory/
git commit -m "feat(customs): description in the comments column, process comments in a new column"
```

### Task 3: The service passes the description on (FR-47)

**Files:**
- Modify: `backend/internal/core/services/service_customs_inventory.go` (`toCustomsItem`)
- Test: `backend/internal/core/services/service_customs_inventory_test.go`

**Interfaces:**
- Consumes: `Item.Description` from Task 1; the K/L layout from Task 2.

- [ ] **Step 1: Write the failing test**

`TestCustomsInventory_ExportDescriptionColumns`: `f := newCustomsFixture(t)`; `tClient.Entity.UpdateOneID(f.lamp).SetDescription("Brass,\nwith shade").Save(f.ctx)`. `req := f.request(f.clock, f.lamp)`, `req.Options.IncludeReceipts = false`; export and open the workbook as in `TestCustomsInventory_ExportXLSXOnly`. Find each row by its C cell. Assert:
- "Old lamp": K `"Brass, with shade"`, L `"zollfrei lt. Tag „owned > 2 years“"`;
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
