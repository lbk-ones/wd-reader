import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeCacheEntries, mergeFrontendCache, snapshotFrontendCache } from '../src/components/resourceCache.js';

function storage(values = {}) {
    const map = new Map(Object.entries(values));
    return {
        get length() {
            return map.size;
        },
        key: (index) => [...map.keys()][index] ?? null,
        getItem: (key) => map.get(key) ?? null,
        setItem: (key, value) => map.set(key, String(value)),
        removeItem: (key) => map.delete(key),
    };
}

test('exports every storage key, including keys outside the application prefix and session values', () => {
    const caches = {
        localStorage: storage({ 'Wd-fontSize': '20', custom: 'value' }),
        sessionStorage: storage({ session: '1' }),
    };
    const snapshot = snapshotFrontendCache(caches);
    assert.equal(snapshot.localStorage.custom, 'value');
    assert.equal(snapshot.sessionStorage.session, '1');
});

test('merge keeps current preferences and remaps chapters, scrolling, history and split settings to renamed books', () => {
    const current = {
        'Wd-fontSize': '20',
        'Wd-same.txt': 'current chapter',
        'Wd-scroll-topsame.txt': '25',
        'Wd-LastClickBook': 'same.txt,existing.txt',
        'Wd-SplitMap': JSON.stringify({ 'same.txt': '1', 'same.txt-Value': 'local' }),
    };
    const incoming = {
        'Wd-fontSize': '30',
        'Wd-same.txt': 'imported chapter',
        'Wd-scroll-topsame.txt': '100',
        'Wd-LastClickBook': 'same.txt,new.txt',
        'Wd-SplitMap': JSON.stringify({ 'same.txt': '2', 'same.txt-Value': 'remote', 'new.txt': '3' }),
        'outside-prefix': 'exported',
    };
    const result = mergeCacheEntries(current, incoming, { 'same.txt': 'same (import 2).txt' });
    assert.equal(result['Wd-fontSize'], '20');
    assert.equal(result['Wd-same.txt'], 'current chapter');
    assert.equal(result['Wd-same (import 2).txt'], 'imported chapter');
    assert.equal(result['Wd-scroll-topsame (import 2).txt'], '100');
    assert.equal(result['Wd-LastClickBook'], 'same.txt,existing.txt,same (import 2).txt,new.txt');
    assert.deepEqual(JSON.parse(result['Wd-SplitMap']), {
        'same.txt': '1',
        'same.txt-Value': 'local',
        'same (import 2).txt': '2',
        'same (import 2).txt-Value': 'remote',
        'new.txt': '3',
    });
    assert.equal(result['outside-prefix'], 'exported');
    const repeated = mergeCacheEntries(result, incoming, { 'same.txt': 'same (import 2).txt' });
    assert.deepEqual(repeated, result);
});

test('existing compound settings take priority and special property names remain plain data', () => {
    const result = mergeCacheEntries(
        { 'Wd-SplitMap': '{"a.txt":1}' },
        JSON.parse('{"Wd-SplitMap":"{\\"a.txt\\":2,\\"b.txt\\":3}","__proto__":"value","constructor":"saved"}'),
    );
    assert.deepEqual(JSON.parse(result['Wd-SplitMap']), { 'a.txt': 1, 'b.txt': 3 });
    assert.equal(result.__proto__, 'value');
    assert.equal(result.constructor, 'saved');
    assert.equal(Object.getPrototypeOf(result), null);
});

test('a quota failure rolls back both local and session storage before disk import starts', () => {
    const caches = { localStorage: storage({ current: 'keep' }), sessionStorage: storage({ session: 'keep' }) };
    const before = snapshotFrontendCache(caches);
    const write = caches.sessionStorage.setItem;
    caches.sessionStorage.setItem = (key, value) => {
        if (key === 'too-large') throw new Error('QuotaExceededError');
        return write(key, value);
    };
    assert.throws(
        () => mergeFrontendCache({ localStorage: { added: '1' }, sessionStorage: { 'too-large': '2' } }, {}, caches),
        /Quota/,
    );
    assert.deepEqual(snapshotFrontendCache(caches), before);
});

test('cache changes can be rolled back when the library commit fails', () => {
    const caches = { localStorage: storage({ 'Wd-LastClickBook': 'local.txt' }), sessionStorage: storage() };
    const before = snapshotFrontendCache(caches);
    const change = mergeFrontendCache(
        { localStorage: { 'Wd-LastClickBook': 'remote.txt', 'Wd-new.txt': 'chapter' }, sessionStorage: { added: '1' } },
        {},
        caches,
    );
    assert.equal(change.count, 3);
    change.rollback();
    assert.deepEqual(snapshotFrontendCache(caches), before);
});
