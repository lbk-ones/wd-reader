import test from 'node:test';
import assert from 'node:assert/strict';
import {
    formatResourceSize,
    isDownloadActive,
    mergeResourceResults,
    resourceDownloadProgress,
} from '../src/components/resourceDownloads.js';

test('download progress handles unknown sizes and reserves completion for actual import', () => {
    assert.equal(formatResourceSize(0), '');
    assert.equal(formatResourceSize(1024), '1 KB');
    assert.equal(formatResourceSize(1024 * 1024), '1.0 MB');
    assert.equal(resourceDownloadProgress({ status: 'downloading', received: 100, total: 0 }), 0);
    assert.equal(resourceDownloadProgress({ status: 'downloading', received: 110, total: 100 }), 99);
    assert.equal(resourceDownloadProgress({ status: 'done' }), 100);
    assert.equal(isDownloadActive({ status: 'importing' }), true);
    assert.equal(isDownloadActive({ status: 'error' }), false);
});

test('resource list excludes catalog entries without actual files and keeps different source editions', () => {
    const book = { sourceId: 'archive', id: '1', files: [{ url: 'https://example.org/book.epub' }] };
    assert.equal(
        mergeResourceResults([book, book, { ...book, files: [] }, { ...book, sourceId: 'gutenberg' }]).length,
        2,
    );
});

test('exact titles rank before related titles without presenting sequels as originals', () => {
    const book = { sourceId: 's', id: '1', files: [{ url: 'https://example.org/1' }], matchType: 'related' };
    const exact = { ...book, id: '2', matchType: 'exact' };
    assert.equal(mergeResourceResults([book, exact])[0].id, '2');
});
