import test from 'node:test';
import assert from 'node:assert/strict';
import {getCurrentlyReadingBook, sortBooksByLastOpened} from '../src/components/bookSorting.js';

test('books sort by last opened time, falling back to import time', () => {
    const names = ['old', 'missing', 'new', 'same'];
    const metadata = {old: {importedAt: '2020-01-01T00:00:00Z', lastOpenedAt: '2026-01-01T00:00:00Z'},
        new: {importedAt: '2025-01-01T00:00:00Z'}, same: {importedAt: '2025-01-01T00:00:00Z'}};
    assert.deepEqual(sortBooksByLastOpened(names, metadata), ['old', 'new', 'same', 'missing']);
    assert.deepEqual(names, ['old', 'missing', 'new', 'same']);
});

test('reading badge tracks the last actual open and excludes unread imports', () => {
    const names = ['a', 'b', 'unread'];
    const metadata = {
        a: {importedAt: '2025-01-01', lastOpenedAt: '2025-02-01'},
        b: {importedAt: '2025-01-01', lastOpenedAt: '2025-03-01'},
        unread: {importedAt: '2026-01-01', lastOpenedAt: '2026-01-01'},
    };
    assert.equal(getCurrentlyReadingBook(names, metadata), 'b');
    metadata.a.lastOpenedAt = '2026-02-01';
    assert.equal(getCurrentlyReadingBook(names, metadata), 'a');
    assert.equal(getCurrentlyReadingBook(['unread'], metadata), null);
    assert.equal(getCurrentlyReadingBook(names, {}), null);
    assert.equal(getCurrentlyReadingBook(['b', 'unread'], metadata), 'b');
});
