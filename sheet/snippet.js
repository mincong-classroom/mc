(async (config) => {
  const page = location.pathname.match(/\/spreadsheets\/d\/([^/]+)/);
  if (!page) {
    return "Not a spreadsheet: run it in the tab of the spreadsheet.";
  }
  if (config.sheet && page[1] !== config.sheet) {
    return `Not the spreadsheet of mc: open https://docs.google.com/spreadsheets/d/${config.sheet}/edit first.`;
  }
  const letters = Object.keys(config.columns);
  const url = `/spreadsheets/d/${page[1]}/gviz/tq?tqx=out:csv&headers=1` +
    `&sheet=${encodeURIComponent(config.tab)}&tq=${encodeURIComponent("select " + letters.join(", "))}`;
  const response = await fetch(url, { credentials: "include" });
  const body = await response.text();
  if (!response.ok || !(response.headers.get("content-type") || "").includes("text/csv")) {
    return `The tab ${config.tab} could not be read (HTTP ${response.status}): ${body.slice(0, 300)}`;
  }
  // The values of a CSV: within quotes when they hold a comma, a quote or a line break.
  const lines = [];
  let line = [], value = "", quoted = false;
  for (let i = 0; i < body.length; i++) {
    const c = body[i];
    if (quoted) {
      if (c === '"' && body[i + 1] === '"') { value += '"'; i++; }
      else if (c === '"') { quoted = false; }
      else { value += c; }
    } else if (c === '"') { quoted = true; }
    else if (c === ",") { line.push(value); value = ""; }
    else if (c === "\n") { line.push(value); lines.push(line); line = []; value = ""; }
    else if (c !== "\r") { value += c; }
  }
  if (value !== "" || line.length > 0) { line.push(value); lines.push(line); }
  const [headers = [], ...data] = lines;
  const columns = {};
  letters.forEach((letter, j) => { columns[letter] = (headers[j] || "").trim() || `(column ${letter})`; });
  // The first row holds the headers, and gviz keeps the empty rows above the last one: the row of
  // data i is the row i + 2 of the tab. A row without its keys, such as an empty row, is left out.
  const rows = [];
  data.forEach((values, i) => {
    const row = { _row: i + 2 };
    letters.forEach((letter, j) => {
      if ((values[j] || "") !== "") { row[columns[letter]] = values[j]; }
    });
    if (config.keys.every((key) => row[key])) { rows.push(row); }
  });
  const json = JSON.stringify({ tab: config.tab, keys: config.keys, columns, rows }, null, 2) + "\n";
  try {
    await navigator.clipboard.writeText(json);
  } catch (error) {
    if (typeof copy !== "function") {
      return `The rows could not be copied to the clipboard (${error}): click in the page, then run the script again.`;
    }
    copy(json); // The console of the DevTools has its own copy.
  }
  return `Copied ${rows.length} rows of the tab ${config.tab}, the columns ${letters.join(", ")} only, to the clipboard. ` +
    `Save them with: pbpaste > ${config.path}`;
})(CONFIG);
