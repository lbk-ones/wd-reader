import test from 'node:test';
import assert from 'node:assert/strict';
import {sortBooksByLastOpened} from '../src/components/bookSorting.js';

test('books sort by last opened time, falling back to import time', () => {
    const names = ['old', 'missing', 'new', 'same'];
    const metadata = {old: {importedAt: '2020-01-01T00:00:00Z', lastOpenedAt: '2026-01-01T00:00:00Z'},
        new: {importedAt: '2025-01-01T00:00:00Z'}, same: {importedAt: '2025-01-01T00:00:00Z'}};
    assert.deepEqual(sortBooksByLastOpened(names, metadata), ['old', 'new', 'same', 'missing']);
    assert.deepEqual(names, ['old', 'missing', 'new', 'same']);
});
