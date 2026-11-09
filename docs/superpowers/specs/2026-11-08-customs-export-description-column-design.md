# Customs Inventory Export: Item Description in the Comments Column: Design

**Date:** 2026-11-08 · **Status:** ready; the user's answers are in §4, calls 6–9 can still be changed
**Builds on:** [`2026-10-31-customs-inventory-export-design.md`](2026-10-31-customs-inventory-export-design.md) (the "first spec"), [`2026-11-02-customs-inventory-export-round-2-design.md`](2026-11-02-customs-inventory-export-round-2-design.md) ("round 2") and [`2026-11-08-customs-export-sold-price-design.md`](2026-11-08-customs-export-sold-price-design.md). Where they differ, this one wins.

**What this changes:**
- FR-14 and FR-17: the sheet has columns A–L, the title row is merged over A1:L1, and column K wraps its text;
- FR-7, FR-10 and round 2's contents rule: the texts the export writes itself (reasons, "Wert fehlt", "Inhalt von …", "inkl. Inhalt …") move from column K to the new column L;
- FR-17 "no colours, images or merged cells": column K may now hold bold, italic and underlined text;
- the first spec's call "The comments column holds the classification reasons, never item notes" is reversed by the user: column K now holds the item description.

## 1. What is wrong today

Column K, "Bemerkungen / comments", holds only the texts the export works out itself, such as "Kauf nach Stichtag (< 6 Monate)" or "Wert fehlt". The item's own description (the **Description** field in HomeBox, for example "Oak case, made 1890") never reaches the sheet, although it often says what the item is better than its name does.

The item page shows the description as Markdown (markdown-it with HTML allowed), so a description can contain markup such as `**Oak**` or `<u>1890</u>`.

Note: column C, "Bezeichnung / description", is the item **name**. It doesn't change.

## 2. Requirements

- **FR-47 Description in column K.** Column K, "Bemerkungen / comments", shows the description of each line member. Only the entity's `Description` field is used, never its notes or custom fields.
  - **Clean-up:** `\r\n` and `\r` become `\n`. In each line, every run of spaces and tabs becomes one space, and the line is trimmed. Empty lines are dropped. The remaining lines are joined by `\n`, so line breaks stay (Q3). A description that is empty after this adds nothing.
  - **Merged lines (FR-7):** the different cleaned-up descriptions of the members, each once, in member order, joined by "; ".
  - **Contents rolled into a container (round 2):** only the line members' own descriptions count. The descriptions of absorbed contents are not added to the container's line.
  - No description on any member: the cell stays empty.
- **FR-48 Process comments in column L.** New column L, header `Prozesshinweise / process comments`. It holds exactly what column K held before, built by the same rules (FR-7 "comments", FR-10, round 2 contents comments), joined by "; ".
- **FR-49 Sheet layout.** Columns A–L. Column L is 40 wide, like K. The title row is merged over A1:L1. Total and subtotal rows don't change.
  - Every non-empty K cell has wrap text on, so its line breaks show. L has no wrap.
  - K and L are text cells (FR-16): a description that starts with `=`, `+`, `-` or `@` never becomes a formula, with or without formatting.
  - **Cell limit:** Excel allows at most 32,767 characters (UTF-16 units) per cell. A K cell with formatting is cut by the export to at most 32,767 units, at a character boundary (a character outside the BMP is never split). A K cell without formatting is cut the same way by the spreadsheet library. The export never fails because of a long description.
- **FR-50 Formatting.** The description is written as its raw text, except bold, italic and underline, which become Excel formatting (Q4). Their markers are removed; everything else (lists, links, headings, other HTML) stays as it is written. Rules, applied to each line on its own (call 6):
  - `**x**` and `__x__` are bold; `*x*` and `_x_` are italic; `***x***` and `___x___` are bold and italic; `<u>x</u>` is underlined (the tags in any letter case). Formatting can nest, for example `**bold _and italic_**`.
  - A `*` or `_` marker is a run of 1–3 equal characters. A run of 4 or more is plain text.
  - An opening marker is followed by a non-space character and isn't preceded by a letter or digit. A closing marker is preceded by a non-space character and isn't followed by a letter or digit (call 7). So `snake_case_name`, `2*3*4`, `2 * 3` and a `* ` list marker stay plain text.
  - A marker preceded by a backslash is plain text, and the backslash stays (call 8).
  - A closing marker matches the nearest open marker of the same character and length. Open markers between the two that are still unmatched become plain text. A marker that is never matched is plain text.
  - A K cell with no formatted part is written as a plain string; otherwise as rich text (call 9). Adjacent parts with the same formatting are one run.
- **Unchanged:**
  - the header text of column K, `Bemerkungen / comments`;
  - the API request and response, so no codegen; the UI and its copy; the database schema;
  - classification, totals, merging rules (the description is not part of the merge key), issues and the receipt ZIP.

## 3. Exact copy

Sheet, column L header (Go): `Prozesshinweise / process comments`. No UI copy changes.

## 4. Decisions and calls

**Decided by the user:**
- The item description goes into "Bemerkungen / comments"; the texts the export writes itself move to a separate "process comments" column.
- Calls 1, 2 and 4 below are confirmed, and so is "merging ignores the description".
- **Q3 Line breaks** stay line breaks (this replaces the earlier call 3, which turned them into spaces).
- **Q4 Markdown:** raw text, but bold, italic and underline become Excel formatting.

**Calls:**
1. **Header of column L:** `Prozesshinweise / process comments`, German first like every other header. *(confirmed)*
2. **Column order:** the new column goes last (L), so columns A–K keep their letters and K keeps its header. *(confirmed)*
3. *(replaced by Q3)*
4. **Contents' descriptions are left out** of the container's line. The line already says "inkl. Inhalt (n Stk.)". *(confirmed)*
5. **Very long cells** *(replaced by FR-49 "Cell limit")*: the spreadsheet library fails on rich text over the limit, so the export cuts it itself.
6. **Formatting stays within one line.** A marker opened on one line and closed on the next is plain text. Lines are cleaned up separately anyway, and empty lines are dropped, so Markdown's paragraph rule couldn't be followed exactly.
7. **The word rule applies to `*` too.** Markdown allows `foo*bar*baz` to be italic inside a word; here it stays plain text, so numbers such as `2*3*4` are never formatted by mistake.
8. **A backslash keeps a marker plain, and stays visible**, because the rest of the description is raw text too.
9. **Plain string when nothing is formatted**, so most cells stay as simple as today.

## 5. QA strategy

| layer | what | where |
|---|---|---|
| Pure unit | clean-up (`\r\n`, empty lines, spaces, spaces only); merged line with equal and different descriptions; contents' descriptions not rolled in; process comments unchanged under their new name | `customsinventory/build_test.go`, `contents_test.go` |
| Pure unit | FR-50 table: each marker, nesting, `***`, `<U>`, look-alikes that stay plain (`snake_case_name`, `2*3*4`, `* item`, unclosed `**`, `\*x\*`, `****x****`), mixed lengths | `customsinventory/markup_test.go` |
| Sheet | 12 headers, L header text, A1:L1 merge, K wrap and L no wrap, K and L contents, rich-text runs, description starting with `=` stays text (plain and rich), plain and rich text over 32,767 units don't fail and are cut, no split surrogate pair | `customsinventory/xlsx_test.go` |
| Service | real test database: a description with a line break and bold reaches column K of the exported sheet, the process comment lands in L | `services/service_customs_inventory_test.go` |
