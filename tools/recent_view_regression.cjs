'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.resolve(__dirname, '../web/app.js'), 'utf8');
const start = source.indexOf('  function normalizedRecentViewedEntries() {');
const end = source.indexOf('  function recentViewedEntries() {', start);
assert(start >= 0 && end > start, 'recently-viewed normalization unavailable');

const state = { recentlyViewed: [
  { type: 'item', id: '4', name: 'A' },
  { type: 'item', id: 4, name: 'Duplicate' },
  { type: 'item', id: '4" onclick="alert(1)', name: 'Bad' },
  { type: 'item', id: 5, name: { text: 'Unexpected' } },
  { type: 'item', id: true, name: 'Boolean identifier' },
  { type: 'item', id: Number.MAX_SAFE_INTEGER + 1, name: 'Unsafe' },
  { type: 'monster', id: 6, name: 'Server A', server: 'kiss' },
  { type: 'monster', id: 6, name: 'Server B', server: 'original' },
  { type: 'monster', id: 7, name: 'Invalid server', server: 'unknown' },
] };
const context = { state, RECENT_VIEWED_LIMIT: 8,
  hasMeaningfulText: value => typeof value === 'string' && Boolean(value.trim()),
  normalizeServerKey: key => key === 'or' ? 'original' : key,
};
vm.runInNewContext(`${source.slice(start, end)}\nthis.normalize = normalizedRecentViewedEntries;`, context);
const result = context.normalize();
assert.deepEqual(JSON.parse(JSON.stringify(result.map(({type,id,server}) => ({type,id,server:server || ''})))), [
  {type:'item',id:4,server:''},
  {type:'monster',id:6,server:'kiss'},
  {type:'monster',id:6,server:'original'},
]);
console.log('Recently-viewed safety/dedup regression: PASS');
