/**
 * mc sheet script: imports the export of mc eval export in this spreadsheet.
 *
 * Paste this file in "Extensions", then "Apps Script", in place of the code there, save it, then
 * reload the spreadsheet: the menu "mc", then "Import", opens a dialog, where you paste the export.
 * The first import asks to authorize the script, once.
 *
 * Each tab of the export has a range of mc: its columns, which the import rewrites on every row.
 * A row is found by its keys, such as the team, the lab and the rule, and a new key gets a new
 * row at the end. The other columns, such as those of the teacher, are never written, and never
 * read but for the keys and the confirmed column: a row whose confirmed cell is filled, such as
 * the completeness of the teacher, is never written, and the summary lists its cells that differ
 * from the export.
 *
 * @OnlyCurrentDoc
 */

/** Adds the menu "mc" to the spreadsheet. */
function onOpen() {
  SpreadsheetApp.getUi().createMenu("mc").addItem("Import", "mcShowImport").addToUi();
}

/** Opens the dialog of the import. */
function mcShowImport() {
  const html = HtmlService.createHtmlOutput(MC_DIALOG).setWidth(760).setHeight(560);
  SpreadsheetApp.getUi().showModalDialog(html, "Import the export of mc");
}

/**
 * Imports an export, the JSON of mc eval export, and returns what it did. It checks every tab
 * before it writes any, then writes each tab, then reads the cells back and compares them.
 */
function mcImport(text) {
  const started = Date.now();
  const data = mcParse(text);
  const spreadsheet = SpreadsheetApp.getActive();
  if (data.spreadsheet && data.spreadsheet !== spreadsheet.getId()) {
    throw new Error(`This export is for the spreadsheet ${data.spreadsheet}, not this one.`);
  }
  const lock = LockService.getDocumentLock();
  lock.waitLock(30000);
  try {
    const plans = data.tabs.map((tab) => {
      const sheet = spreadsheet.getSheetByName(tab.name);
      if (!sheet) {
        throw new Error(`No tab "${tab.name}" in this spreadsheet.`);
      }
      const width = Math.max(sheet.getLastColumn(), ...Object.keys(tab.columns).map(mcColumnIndex)) + 1;
      const headers = sheet.getRange(1, 1, 1, width).getValues()[0];
      const height = sheet.getLastRow() - 1;
      const read = (letter) => (height > 0 ? sheet.getRange(2, mcColumnIndex(letter) + 1, height, 1).getValues().map((r) => r[0]) : []);
      const keys = {};
      for (const key of tab.keys) {
        keys[key] = read(mcLetterOf(tab, key));
      }
      const letter = Object.keys(tab.confirmed || {})[0];
      return { sheet, tab, plan: mcPlan(tab, headers, keys, letter ? read(letter) : []) };
    });
    for (const { sheet, tab, plan } of plans) {
      mcWrite(sheet, tab, plan);
    }
    SpreadsheetApp.flush();
    const results = plans.map(({ sheet, tab, plan }) => {
      const differences = [];
      const held = [];
      for (const group of plan.lastRow < 2 ? [] : plan.groups) {
        const range = sheet.getRange(2, group.first + 1, plan.lastRow - 1, group.letters.length);
        const compared = mcCompare(tab, plan, group, range.getValues(), range.getFormulas());
        differences.push(...compared.differences);
        held.push(...compared.held);
      }
      return { tab, plan, differences, held };
    });
    const summary = mcSummary(results, (Date.now() - started) / 1000);
    spreadsheet.toast(summary.split("\n")[0], "mc", 30);
    return summary;
  } finally {
    lock.releaseLock();
  }
}

/**
 * Writes the plan of a tab: the new rows first, then the range of mc, group after group, run after
 * run of the rows that are not confirmed.
 */
function mcWrite(sheet, tab, plan) {
  if (plan.added.length > 0 && plan.added[0] > 2) {
    const first = plan.added[0];
    for (const letter of tab.fill || []) {
      const column = mcColumnIndex(letter) + 1;
      sheet.getRange(first - 1, column).copyTo(sheet.getRange(first, column, plan.added.length, 1));
    }
  }
  if (plan.lastRow < 2) {
    return; // An empty tab, and an export without rows.
  }
  const runs = mcRuns(plan);
  for (const group of plan.groups) {
    const grid = mcGrid(tab, plan, group);
    for (const [first, last] of runs) {
      sheet.getRange(first, group.first + 1, last - first + 1, group.letters.length).setValues(grid.slice(first - 2, last - 1));
    }
  }
}

// The functions below use no service of Apps Script, so that node can test them.

/** Parses an export, and checks what the import needs. */
function mcParse(text) {
  let data;
  try {
    data = JSON.parse(text);
  } catch (error) {
    throw new Error(`Not the JSON of mc eval export: ${error.message}`);
  }
  if (!data || !Array.isArray(data.tabs) || data.tabs.length === 0) {
    throw new Error("Not an export of mc: no tabs.");
  }
  for (const tab of data.tabs) {
    if (!tab.name || !Array.isArray(tab.keys) || tab.keys.length === 0 || !tab.columns || !Array.isArray(tab.rows)) {
      throw new Error(`The tab ${JSON.stringify(tab.name)} of the export has no name, keys, columns or rows.`);
    }
    for (const key of tab.keys) {
      if (!mcLetterOf(tab, key)) {
        throw new Error(`The key "${key}" of the tab ${tab.name} has no column.`);
      }
    }
    const confirmed = Object.keys(tab.confirmed || {});
    if (confirmed.length > 1 || confirmed.some((letter) => tab.columns[letter] !== undefined)) {
      throw new Error(`The tab ${tab.name} of the export has a confirmed column that is not one column out of the range of mc.`);
    }
  }
  return data;
}

/** Returns the index of a column from its letters, from 0 for A. */
function mcColumnIndex(letters) {
  let index = 0;
  for (const c of letters) {
    index = index * 26 + c.charCodeAt(0) - 64;
  }
  return index - 1;
}

/** Returns the letter of the column of a header in the tab of an export, or undefined. */
function mcLetterOf(tab, header) {
  return Object.keys(tab.columns).find((letter) => tab.columns[letter] === header);
}

/** Returns the keys of a row, joined: the same for the export and the tab. */
function mcKey(values) {
  return values.map((v) => String(v).trim()).join("\u001f");
}

/**
 * Plans the import of a tab: headers are the cells of its first row, keys the values of each key
 * on the rows 2 and on, and confirmed the values of the confirmed column on the same rows, if any.
 * It checks the headers, finds the row of each row of the export and the confirmed rows, and groups
 * the columns of mc that touch each other.
 */
function mcPlan(tab, headers, keys, confirmed) {
  const wrong = [];
  const expected = Object.assign({}, tab.columns, tab.confirmed || {});
  for (const letter of Object.keys(expected)) {
    const got = String(headers[mcColumnIndex(letter)] === undefined ? "" : headers[mcColumnIndex(letter)]).trim();
    if (got !== expected[letter]) {
      wrong.push(`${letter}1 is ${JSON.stringify(got)}, not ${JSON.stringify(expected[letter])}`);
    }
  }
  if (wrong.length > 0) {
    throw new Error(`The headers of the tab ${tab.name} changed, nothing was written: ${wrong.join("; ")}.`);
  }
  const height = keys[tab.keys[0]].length;
  const rowOf = new Map(); // The row of each key of the tab.
  const keysAt = []; // The keys of each row of the tab, from the row 2.
  const confirmedRows = []; // The rows with keys whose confirmed cell is filled.
  let lastKeyed = 1;
  for (let i = 0; i < height; i++) {
    const values = tab.keys.map((k) => keys[k][i]);
    keysAt.push(values);
    if (values.every((v) => String(v).trim() === "")) {
      continue;
    }
    const key = mcKey(values);
    if (rowOf.has(key)) {
      throw new Error(`The rows ${rowOf.get(key)} and ${i + 2} of the tab ${tab.name} have the same keys, ${values.join(" ")}: nothing was written.`);
    }
    rowOf.set(key, i + 2);
    lastKeyed = i + 2;
    if (confirmed && confirmed[i] !== undefined && confirmed[i] !== null && String(confirmed[i]).trim() !== "") {
      confirmedRows.push(i + 2);
    }
  }
  const rows = []; // The row of each row of the export.
  const added = [];
  const exported = new Set();
  let next = lastKeyed + 1;
  for (const row of tab.rows) {
    const key = mcKey(tab.keys.map((k) => row[k]));
    exported.add(key);
    if (rowOf.has(key)) {
      rows.push(rowOf.get(key));
    } else {
      rows.push(next);
      added.push(next);
      next++;
    }
  }
  const unknown = [];
  for (const [key, row] of rowOf) {
    if (!exported.has(key)) {
      unknown.push({ row, keys: keysAt[row - 2] });
    }
  }
  unknown.sort((a, b) => a.row - b.row);
  const indexes = Object.keys(tab.columns).map(mcColumnIndex).sort((a, b) => a - b);
  const groups = [];
  for (const index of indexes) {
    const last = groups[groups.length - 1];
    if (last && last.first + last.letters.length === index) {
      last.letters.push(mcLetters(index));
    } else {
      groups.push({ first: index, letters: [mcLetters(index)] });
    }
  }
  return { rows, added, unknown, keysAt, groups, confirmed: confirmedRows, lastRow: Math.max(next - 1, lastKeyed) };
}

/**
 * Returns the runs of rows that the import writes, from the row 2 to the last row of the plan, as
 * [first, last]: every row but the confirmed ones.
 */
function mcRuns(plan) {
  const confirmed = new Set(plan.confirmed || []);
  const runs = [];
  for (let row = 2; row <= plan.lastRow; row++) {
    if (confirmed.has(row)) {
      continue;
    }
    const last = runs[runs.length - 1];
    if (last && last[1] === row - 1) {
      last[1] = row;
    } else {
      runs.push([row, row]);
    }
  }
  return runs;
}

/** Returns the letters of the column of an index, from 0 for A. */
function mcLetters(index) {
  let letters = "";
  for (index++; index > 0; index = Math.floor((index - 1) / 26)) {
    letters = String.fromCharCode(65 + ((index - 1) % 26)) + letters;
  }
  return letters;
}

/**
 * Returns what a cell of the range of mc holds after the import: a formula, a value, or "" for an
 * empty cell. A row of the tab that the export doesn't have keeps its keys, and nothing else.
 */
function mcCell(tab, plan, rowOf, header, row) {
  const i = rowOf.get(row);
  if (i === undefined) {
    const k = tab.keys.indexOf(header);
    const keys = plan.keysAt[row - 2];
    return k >= 0 && keys ? keys[k] : "";
  }
  if (tab.formulas && tab.formulas[header]) {
    return { formula: tab.formulas[header].split("{row}").join(String(row)) };
  }
  const value = tab.rows[i][header];
  if (value === undefined || value === null) {
    return "";
  }
  if (typeof value === "string" && value.startsWith("=")) {
    return { formula: value.split("{row}").join(String(row)) };
  }
  return value;
}

/** Returns the rows of the export by the row of the tab where they go. */
function mcRowsOf(plan) {
  const rowOf = new Map();
  plan.rows.forEach((row, i) => rowOf.set(row, i));
  return rowOf;
}

/**
 * Returns the cells to write in a group of columns, from the row 2 to the last row. A text starts
 * with an apostrophe, so that Sheets keeps it a text, even when it looks like a number or a date.
 */
function mcGrid(tab, plan, group) {
  const rowOf = mcRowsOf(plan);
  const grid = [];
  for (let row = 2; row <= plan.lastRow; row++) {
    grid.push(group.letters.map((letter) => {
      const cell = mcCell(tab, plan, rowOf, tab.columns[letter], row);
      if (cell && typeof cell === "object") {
        return cell.formula;
      }
      if (typeof cell === "string" && cell !== "") {
        return "'" + cell;
      }
      return cell;
    }));
  }
  return grid;
}

/** Returns a formula without its spaces and in uppercase, but in its texts: how Sheets may store it. */
function mcNormalizeFormula(formula) {
  let out = "";
  let quoted = false;
  for (const c of String(formula)) {
    if (c === '"') {
      quoted = !quoted;
    }
    out += quoted || c === '"' ? c : c === " " ? "" : c.toUpperCase();
  }
  return out;
}

/**
 * Compares the cells of a group of columns, as read after the import, with the export. It returns
 * the differences on the rows that the import wrote, such as "Evaluation!G5: 0.8, not 0.9", and,
 * as held, the cells of the confirmed rows of the export that differ from it, which the import left
 * as they are, such as "Evaluation!G5".
 */
function mcCompare(tab, plan, group, values, formulas) {
  const rowOf = mcRowsOf(plan);
  const confirmed = new Set(plan.confirmed || []);
  const differences = [];
  const held = [];
  for (let row = 2; row <= plan.lastRow; row++) {
    if (confirmed.has(row) && !rowOf.has(row)) {
      continue; // A confirmed row that the export doesn't have: listed as not in the export.
    }
    group.letters.forEach((letter, j) => {
      const want = mcCell(tab, plan, rowOf, tab.columns[letter], row);
      const value = values[row - 2][j];
      const formula = formulas[row - 2][j];
      let same;
      let got = value;
      if (want && typeof want === "object") {
        same = mcNormalizeFormula(formula) === mcNormalizeFormula(want.formula);
        got = formula ? { formula } : value;
      } else if (typeof want === "number") {
        same = typeof value === "number" && Math.abs(value - want) < 1e-9;
      } else if (typeof want === "boolean") {
        same = value === want;
      } else {
        same = String(value) === String(want);
      }
      if (!same && confirmed.has(row)) {
        held.push(`${tab.name}!${letter}${row}`);
      } else if (!same) {
        const show = (v) => (v && typeof v === "object" ? v.formula : JSON.stringify(v));
        differences.push(`${tab.name}!${letter}${row}: ${show(got)}, not ${show(want)}`);
      }
    });
  }
  return { differences, held };
}

/** Returns the summary of an import: its first line for a toast, then a line per tab. */
function mcSummary(results, seconds) {
  const rows = results.reduce((n, r) => n + r.tab.rows.length, 0);
  const added = results.reduce((n, r) => n + r.plan.added.length, 0);
  const differences = results.reduce((n, r) => n + r.differences.length, 0);
  const confirmed = results.reduce((n, r) => n + (r.plan.confirmed || []).length, 0);
  const lines = [
    `Imported ${rows} rows in ${seconds.toFixed(1)} s, ${added} new, ` +
      (confirmed > 0 ? `${confirmed} confirmed left as they are, ` : "") +
      (differences === 0 ? "and every cell read back as imported." : `but ${differences} cells read back differently.`),
  ];
  for (const { tab, plan, differences, held } of results) {
    const written = mcRuns(plan).reduce((n, [first, last]) => n + last - first + 1, 0);
    const cells = plan.groups.reduce((n, g) => n + g.letters.length, 0) * written;
    let line = `- ${tab.name}: ${tab.rows.length} rows, ${plan.added.length} new, ${cells} cells of the columns ` +
      plan.groups.map((g) => (g.letters.length > 1 ? `${g.letters[0]}:${g.letters[g.letters.length - 1]}` : g.letters[0])).join(", ") + ".";
    if (plan.added.length > 0) {
      line += ` New rows: ${plan.added[0]} to ${plan.added[plan.added.length - 1]}.`;
    }
    if (plan.unknown.length > 0) {
      const confirmedRows = new Set(plan.confirmed || []);
      line += ` Not in the export, so cleared but for their keys, to delete by hand: ` +
        plan.unknown.map((u) => `row ${u.row} (${u.keys.join(" ")}${confirmedRows.has(u.row) ? ", confirmed: left as it is" : ""})`).join(", ") + ".";
    }
    if ((plan.confirmed || []).length > 0) {
      const letter = Object.keys(tab.confirmed || {})[0];
      line += ` ${plan.confirmed.length} rows confirmed in ${letter}, left as they are.`;
      const byRow = new Map();
      for (const cell of held || []) {
        const [, letters, row] = cell.match(/!([A-Z]+)(\d+)$/);
        byRow.set(Number(row), (byRow.get(Number(row)) || []).concat(letters));
      }
      if (byRow.size > 0) {
        const rowsHeld = [...byRow.keys()].sort((a, b) => a - b);
        line += " Their cells that differ from the export: " +
          rowsHeld.slice(0, 20).map((row) => `row ${row} (${plan.keysAt[row - 2].join(" ")}): ${byRow.get(row).join(", ")}`).join("; ") +
          (rowsHeld.length > 20 ? `; and ${rowsHeld.length - 20} more rows` : "") + ".";
      }
    }
    lines.push(line);
    for (const d of differences.slice(0, 20)) {
      lines.push(`  ${d}`);
    }
    if (differences.length > 20) {
      lines.push(`  and ${differences.length - 20} more.`);
    }
  }
  return lines.join("\n");
}

const MC_DIALOG = `<!DOCTYPE html>
<html>
<head>
<base target="_top">
<style>
  body { font: 14px Arial, sans-serif; margin: 0; }
  textarea { box-sizing: border-box; width: 100%; height: 300px; font: 12px monospace; }
  pre { white-space: pre-wrap; font: 12px monospace; max-height: 150px; overflow: auto; }
  button { margin: 8px 8px 8px 0; padding: 6px 16px; }
</style>
</head>
<body>
<p>Paste the export of <code>mc eval export</code>, then click Import, or press Cmd+Enter.</p>
<textarea id="export" spellcheck="false" autofocus></textarea>
<div><button id="import">Import</button><span id="status"></span></div>
<pre id="result"></pre>
<script>
  const area = document.getElementById("export");
  const button = document.getElementById("import");
  const status = document.getElementById("status");
  const result = document.getElementById("result");
  area.focus();
  area.addEventListener("input", () => {
    status.textContent = area.value ? Math.round(area.value.length / 1024) + " KB" : "";
  });
  function run() {
    if (!area.value.trim()) {
      status.textContent = "Paste the export first.";
      return;
    }
    button.disabled = true;
    status.textContent = "Importing...";
    result.textContent = "";
    google.script.run
      .withSuccessHandler((summary) => {
        button.disabled = false;
        status.textContent = "Done.";
        result.textContent = summary;
      })
      .withFailureHandler((error) => {
        button.disabled = false;
        status.textContent = "Failed.";
        result.textContent = String((error && error.message) || error);
      })
      .mcImport(area.value);
  }
  button.addEventListener("click", run);
  document.addEventListener("keydown", (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      run();
    }
  });
</script>
</body>
</html>`;
