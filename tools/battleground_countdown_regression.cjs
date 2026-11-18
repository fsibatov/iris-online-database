const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.join(__dirname, '..');
const html = fs.readFileSync(path.join(root, 'web', 'index.html'), 'utf8');
const js = fs.readFileSync(path.join(root, 'web', 'app.js'), 'utf8');
const css = fs.readFileSync(path.join(root, 'web', 'styles.css'), 'utf8');
const markup = html.match(/<div class="header-status-pill battleground-status" id="battlegroundStatus">([\s\S]*?)<\/div>/)?.[1];
assert.ok(markup, 'Battleground status must exist');
assert.match(markup, /id="battlegroundCountdown"/);
assert.equal((markup.match(/<(?:span|strong|time)\b/g) || []).length, 2, 'Show the battleground name and countdown');
assert.match(markup, /id="battlegroundName"/);
assert.doesNotMatch(markup, /battlegroundStart|<time\b|UTC/);
assert.match(css, /\.battleground-countdown\s*\{[^}]*color:\s*var\(--good\)/);
assert.match(css, /\.battleground-name\s*\{[^}]*white-space:\s*nowrap/);

const source = js.match(/  function renderBattlegroundStatus\(\) \{[\s\S]*?\n  \}/)?.[0];
assert.ok(source, 'Renderer must exist');
const attributes = new Map();
const countdown = { textContent: '--:--' };
const name = { textContent: 'БГ' };
const container = { setAttribute(k, v) { attributes.set(k, v); }, title: '' };
const sandbox = {
  battlegroundStatus: container,
  battlegroundCountdown: countdown,
  battlegroundName: name,
  battlegroundState: () => ({ name: 'Противостояние', start: '03:03', countdown: '07:24' }),
};
vm.runInNewContext(`${source}\nrenderBattlegroundStatus();`, sandbox);
assert.equal(name.textContent, 'Противостояние', 'The next battleground name must be shown');
assert.equal(countdown.textContent, '07:24', 'Countdown must update beside the battleground name');
assert.match(attributes.get('aria-label'), /Противостояние.*07:24/);
assert.equal(container.title, attributes.get('aria-label'));
console.log('Battleground name + countdown layout/accessibility: PASS');
