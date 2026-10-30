import test from 'node:test';
import assert from 'node:assert/strict';
import {sortBooksByImportTime} from '../src/components/bookSorting.js';

test('books sort by import time descending with a stable fallback', () => {
    const names = ['old', 'missing', 'new', 'same'];
    const metadata = {old: {importedAt: '2020-01-01T00:00:00Z'},
        new: {importedAt: '2025-01-01T00:00:00Z'}, same: {importedAt: '2025-01-01T00:00:00Z'}};
    assert.deepEqual(sortBooksByImportTime(names, metadata), ['new', 'same', 'old', 'missing']);
    assert.deepEqual(names, ['old', 'missing', 'new', 'same']);
});
