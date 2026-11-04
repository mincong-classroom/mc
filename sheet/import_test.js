// The tests of import.gs, the script of mc sheet script, with node: node --test sheet/import_test.js.
// The script runs in Apps Script, so the tests load it in a context of its own, and test the
// functions that use no service of Apps Script. The teams are made up.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const context = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(__dirname, "import.gs"), "utf8"), context);
const { mcParse, mcPlan, mcRuns, mcGrid, mcCompare, mcSummary, mcColumnIndex, mcLetters, mcNormalizeFormula } = context;

// An export of one tab: the keys in A and B, the students in C (filled down), the teacher in E.
function tab(rows) {
  return {
    name: "Evaluation",
    keys: ["Team", "Rule"],
    columns: { A: "Team", B: "Rule", D: "TA Completeness", F: "Score", G: "TA Comment" },
    formulas: { Score: "=D{row}*2" },
    fill: ["C"],
    rows,
  };
}

// plain returns a value of the context of the script as a value of this one, to compare it.
const plain = (value) => JSON.parse(JSON.stringify(value));

const headers = ["Team", "Rule", "Students", "TA Completeness", "Completeness", "Score", "TA Comment"];

test("the letters of a column, and back", () => {
  for (const [letters, index] of [["A", 0], ["Z", 25], ["AA", 26], ["AZ", 51], ["BA", 52]]) {
    assert.equal(mcColumnIndex(letters), index);
    assert.equal(mcLetters(index), letters);
  }
});

test("a row is found by its keys, and a new key gets a row at the end", () => {
  const t = tab([
    { Team: "blue", Rule: "L1_BLD", "TA Completeness": 1 },
    { Team: "gold", Rule: "L1_BLD", "TA Completeness": 0.5 },
    { Team: "blue", Rule: "L1_RUN", "TA Completeness": 0.8, "TA Comment": "Done well: it runs." },
  ]);
  const plan = mcPlan(t, headers, { Team: ["gold", "blue", "pink"], Rule: ["L1_BLD", "L1_BLD", "L1_BLD"] });
  assert.deepEqual(plain(plan.rows), [3, 2, 5]);
  assert.deepEqual(plain(plan.added), [5]);
  assert.equal(plan.lastRow, 5);
  assert.deepEqual(plain(plan.unknown), [{ row: 4, keys: ["pink", "L1_BLD"] }]);
  assert.deepEqual(plain(plan.groups.map((g) => g.letters)), [["A", "B"], ["D"], ["F", "G"]]);
});

test("the grid rewrites the range of mc, and clears what the export doesn't give", () => {
  const t = tab([
    { Team: "blue", Rule: "L1_BLD", "TA Completeness": 1 },
    { Team: "gold", Rule: "L1_BLD", "TA Completeness": 0.5, "TA Comment": "1.0.0" },
  ]);
  const plan = mcPlan(t, headers, { Team: ["gold", "blue", "pink"], Rule: ["L1_BLD", "L1_BLD", "L1_BLD"] });
  const [ab, d, fg] = plan.groups;
  assert.deepEqual(plain(mcGrid(t, plan, ab)), [["'gold", "'L1_BLD"], ["'blue", "'L1_BLD"], ["'pink", "'L1_BLD"]]);
  assert.deepEqual(plain(mcGrid(t, plan, d)), [[0.5], [1], [""]]);
  assert.deepEqual(plain(mcGrid(t, plan, fg)), [["=D2*2", "'1.0.0"], ["=D3*2", ""], ["", ""]]);
});

test("a formula of a row gives its row as {row}", () => {
  const t = tab([{ Team: "blue", Rule: "L1_BLD", "TA Comment": "=-0.1*D{row}" }]);
  const plan = mcPlan(t, headers, { Team: [], Rule: [] });
  assert.deepEqual(plain(plan.added), [2]);
  assert.deepEqual(plain(mcGrid(t, plan, plan.groups[2])), [["=D2*2", "=-0.1*D2"]]);
});

test("the import stops before any write when a header changed", () => {
  const t = tab([]);
  const moved = ["Team", "Rule", "Students", "Completeness", "TA Completeness", "Score", "TA Comment"];
  assert.throws(() => mcPlan(t, moved, { Team: [], Rule: [] }), /D1 is "Completeness", not "TA Completeness"/);
});

test("the import stops when two rows of the tab have the same keys", () => {
  assert.throws(() => mcPlan(tab([]), headers, { Team: ["blue", "blue"], Rule: ["L1_BLD", "L1_BLD"] }), /rows 2 and 3/);
});

test("the cells read back are compared with what was written", () => {
  const t = tab([{ Team: "blue", Rule: "L1_BLD", "TA Completeness": 0.8, "TA Comment": "1.0.0" }]);
  const plan = mcPlan(t, headers, { Team: ["blue"], Rule: ["L1_BLD"] });
  const fg = plan.groups[2];
  assert.deepEqual(plain(mcCompare(t, plan, fg, [[1.6, "1.0.0"]], [["=d2 * 2", ""]]).differences), []);
  assert.deepEqual(plain(mcCompare(t, plan, fg, [[1.6, "1.0"]], [["=D2*3", ""]]).differences), [
    'Evaluation!F2: =D2*3, not =D2*2',
    'Evaluation!G2: "1.0", not "1.0.0"',
  ]);
  assert.deepEqual(plain(mcCompare(t, plan, plan.groups[1], [[0.8]], [[""]]).differences), []);
  assert.deepEqual(plain(mcCompare(t, plan, plan.groups[1], [["0.8"]], [[""]]).differences), ['Evaluation!D2: "0.8", not 0.8']);
});

// confirmedTab is the tab of tab(), whose rows are confirmed by the completeness of the teacher, E.
function confirmedTab(rows) {
  return Object.assign(tab(rows), { confirmed: { E: "Completeness" } });
}

test("a confirmed row is never written, and its cells that differ are held", () => {
  const t = confirmedTab([
    { Team: "blue", Rule: "L1_BLD", "TA Completeness": 0.7 },
    { Team: "gold", Rule: "L1_BLD", "TA Completeness": 0.5 },
    { Team: "pink", Rule: "L1_BLD", "TA Completeness": 1 },
    { Team: "teal", Rule: "L1_BLD", "TA Completeness": 0.9 },
  ]);
  const keys = { Team: ["blue", "gold", "pink", "red"], Rule: ["L1_BLD", "L1_BLD", "L1_BLD", "L1_BLD"] };
  const plan = mcPlan(t, headers, keys, [1, "", " 0.5 ", 0.8]);
  assert.deepEqual(plain(plan.confirmed), [2, 4, 5]);
  assert.deepEqual(plain(plan.added), [6]);
  // The rows 3 (gold) and 6 (teal, new) are written; 2, 4 and 5 are left as they are.
  assert.deepEqual(plain(mcRuns(plan)), [[3, 3], [6, 6]]);
  const d = plan.groups[1];
  const compared = mcCompare(t, plan, d, [[1], [0.5], [1], ["x"], [0.9]], [[""], [""], [""], [""], [""]]);
  // blue keeps 1 against 0.7 of the export: held. pink holds 1 as the export: nothing. The row 5,
  // red, is not in the export: neither held nor a difference.
  assert.deepEqual(plain(compared), { differences: [], held: ["Evaluation!D2"] });
  const summary = mcSummary([{ tab: t, plan, differences: [], held: compared.held }], 0.5);
  assert.equal(summary, [
    "Imported 4 rows in 0.5 s, 1 new, 3 confirmed left as they are, and every cell read back as imported.",
    "- Evaluation: 4 rows, 1 new, 10 cells of the columns A:B, D, F:G. New rows: 6 to 6." +
      " Not in the export, so cleared but for their keys, to delete by hand: row 5 (red L1_BLD, confirmed: left as it is)." +
      " 3 rows confirmed in E, left as they are. Their cells that differ from the export: row 2 (blue L1_BLD): D.",
  ].join("\n"));
});

test("without a confirmed column, every row is written", () => {
  const t = tab([{ Team: "blue", Rule: "L1_BLD", "TA Completeness": 1 }]);
  const plan = mcPlan(t, headers, { Team: ["blue", "gold"], Rule: ["L1_BLD", "L1_BLD"] });
  assert.deepEqual(plain(plan.confirmed), []);
  assert.deepEqual(plain(mcRuns(plan)), [[2, 3]]);
});

test("the import stops when the confirmed column moved", () => {
  const moved = ["Team", "Rule", "Students", "TA Completeness", "Comment", "Score", "TA Comment"];
  assert.throws(() => mcPlan(confirmedTab([]), moved, { Team: [], Rule: [] }, []), /E1 is "Comment", not "Completeness"/);
});

test("a formula is compared without its spaces and its case, but in its texts", () => {
  assert.equal(mcNormalizeFormula('=concat(A2, "_ x")'), '=CONCAT(A2,"_ x")');
});

test("an export without tabs, or a key without a column, is refused", () => {
  assert.throws(() => mcParse("{"), /Not the JSON of mc eval export/);
  assert.throws(() => mcParse('{"tabs": []}'), /no tabs/);
  assert.throws(() => mcParse('{"tabs": [{"name": "Rules", "keys": ["Rule"], "columns": {"A": "Lab"}, "rows": []}]}'), /key "Rule"/);
  assert.throws(
    () => mcParse('{"tabs": [{"name": "Evaluation", "keys": ["Team"], "columns": {"A": "Team"}, "confirmed": {"A": "Team"}, "rows": []}]}'),
    /confirmed column/,
  );
});

test("the summary says what the import did", () => {
  const t = tab([{ Team: "blue", Rule: "L1_BLD", "TA Completeness": 1 }]);
  const plan = mcPlan(t, headers, { Team: ["pink"], Rule: ["L1_BLD"] });
  const summary = mcSummary([{ tab: t, plan, differences: [] }], 1.234);
  assert.equal(summary, [
    "Imported 1 rows in 1.2 s, 1 new, and every cell read back as imported.",
    "- Evaluation: 1 rows, 1 new, 10 cells of the columns A:B, D, F:G. New rows: 3 to 3." +
      " Not in the export, so cleared but for their keys, to delete by hand: row 2 (pink L1_BLD).",
  ].join("\n"));
});
