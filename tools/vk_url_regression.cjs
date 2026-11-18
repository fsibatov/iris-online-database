'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.resolve(__dirname, '../web/app.js'), 'utf8');
const start = source.indexOf('  function trustedVkPostURL(');
const end = source.indexOf('  function vkNewsCardHTML()', start);
assert(start >= 0 && end > start, 'trusted VK URL validation unavailable');
const context = {};
vm.runInNewContext(`${source.slice(start, end)}\nthis.trusted = trustedVkPostURL;`, context);
assert.equal(context.trusted(123, 'https://vk.ru/wall-59626511_123'), 'https://vk.ru/wall-59626511_123');
for (const url of ['javascript:alert(1)', 'https://evil.example/', 'https://vk.ru/wall-59626511_124', 'https://vk.ru/wall-59626511_123?x=1']) {
  assert.equal(context.trusted(123, url), '');
}
for (const id of [0, -1, 1.2, Number.MAX_SAFE_INTEGER + 1, NaN]) {
  assert.equal(context.trusted(id, `https://vk.ru/wall-59626511_${id}`), '');
}
console.log('VK outbound URL validation: PASS');
