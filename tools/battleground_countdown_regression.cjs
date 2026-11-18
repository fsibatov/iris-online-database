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
assert.equal((markup.match(/<(?:span|strong|time)\b/g) || []).length, 1, 'Countdown must be the only visible battleground text');
assert.doesNotMatch(markup, /БГ|battlegroundName|battlegroundStart|<time\b/);
assert.match(css, /\.battleground-countdown\s*\{[^}]*color:\s*var\(--good\)/);

const source = js.match(/  function renderBattlegroundStatus\(\) \{[\s\S]*?\n  \}/)?.[0];
assert.ok(source, 'Renderer must exist');
const attributes = new Map();
const countdown = { textContent: '--:--' };
const container = { setAttribute(k, v) { attributes.set(k, v); }, title: '' };
const sandbox = {
  battlegroundStatus: container,
  battlegroundCountdown: countdown,
  battlegroundState: () => ({ name: 'Противостояние', start: '03:03', countdown: '07:24' }),
};
vm.runInNewContext(`${source}\nrenderBattlegroundStatus();`, sandbox);
assert.equal(countdown.textContent, '07:24', 'Countdown must update without removed name/start elements');
assert.match(attributes.get('aria-label'), /Противостояние.*07:24/);
assert.equal(container.title, attributes.get('aria-label'));
console.log('Battleground countdown-only layout/accessibility: PASS');
