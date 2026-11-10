import test from 'node:test';
import assert from 'node:assert/strict';
import {
    defaultSourceGroup,
    formatResourceSize,
    isDownloadActive,
    mergeResourceResults,
    normalizeSourceGroup,
    resourceDownloadProgress,
    sourceGroupName,
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

test('download sources rank first, with title relevance used within each priority', () => {
    const files = [{ url: 'https://example.org/book.txt', format: 'TXT' }];
    const books = [
        { sourceId: 'crawler', id: 'crawler-exact', matchType: 'exact', files },
        { sourceId: 'archive', id: 'download-related', matchType: 'related', files },
        { sourceId: 'archive', id: 'download-exact', matchType: 'exact', files },
    ];
    assert.deepEqual(
        mergeResourceResults(books, 'txt', new Set(['archive'])).map((book) => book.id),
        ['download-exact', 'download-related', 'crawler-exact'],
    );
});

test('legacy download group names resolve to the download key and label', () => {
    assert.equal(defaultSourceGroup, 'download');
    assert.equal(normalizeSourceGroup(' 完本下载 '), defaultSourceGroup);
    assert.equal(sourceGroupName(defaultSourceGroup), '完本下载');
    assert.equal(sourceGroupName('自定义书源'), '自定义书源');
});

test('resource types filter individual formats and restore all files without mutating results', () => {
    const txt = { url: 'https://example.org/a.zip', format: 'TXT (ZIP)', archiveFormat: 'txt' };
    const epub = { url: 'https://example.org/a.epub', format: 'EPUB' };
    const books = [
        { sourceId: 's', id: '1', files: [txt, epub] },
        { sourceId: 's', id: '2', files: [epub] },
    ];
    assert.deepEqual(
        mergeResourceResults(books, 'txt').map((book) => book.files),
        [[txt]],
    );
    assert.equal(mergeResourceResults(books, 'epub').length, 2);
    assert.equal(mergeResourceResults(books, 'kindle').length, 0);
    assert.deepEqual(
        mergeResourceResults(books).map((book) => book.files),
        [[txt, epub], [epub]],
    );
    assert.equal(books[0].files.length, 2);
});

test('Kindle resource type groups MOBI, AZW and AZW3 including archives and extensionless download URLs', () => {
    const files = [
        { url: 'https://example.org/download?id=1', format: 'mobi' },
        { url: 'https://example.org/archive.zip', format: 'AZW (ZIP)', archiveFormat: 'azw' },
        { url: 'https://example.org/download?id=3', name: 'book.AZW3' },
        { url: 'https://example.org/a.epub', format: 'EPUB' },
    ];
    assert.deepEqual(mergeResourceResults([{ sourceId: 's', id: '1', files }], 'kindle')[0].files, files.slice(0, 3));
});
