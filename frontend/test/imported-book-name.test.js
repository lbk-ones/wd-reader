import test from 'node:test';
import assert from 'node:assert/strict';
import {importedBookName} from '../src/components/importedBookName.js';

test('identifies the resulting book for local imports and conversions', () => {
    assert.equal(importedBookName('C:\\Books\\我的书.txt'), '我的书.txt');
    assert.equal(importedBookName('C:\\Books\\我的书.epub'), '我的书.txt');
    assert.equal(importedBookName('/books/story.v2.mobi'), 'story.v2.txt');
    assert.equal(importedBookName('/books/story.pdf'), 'story.txt');
});
