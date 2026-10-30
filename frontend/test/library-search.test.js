import test from 'node:test';
import assert from 'node:assert/strict';
import {indexOriginalName, matchesOriginalName} from '../src/components/librarySearch.js';

test('original library search supports names, initials and full pinyin', () => {
    const book = indexOriginalName('我有一座冒险屋.epub');
    for (const query of ['冒险', 'wyyzmxw', 'WO YOU YI ZUO MAO XIAN WU', 'maoxian', 'EPUB', '']) {
        assert(matchesOriginalName(book, query), query);
    }
    assert(!matchesOriginalName(book, 'sanguo'));
    const mixed = indexOriginalName('凡人 修仙传 2.txt');
    assert(matchesOriginalName(mixed, 'frxxc2'));
    assert(matchesOriginalName(mixed, 'fanren xiuxian chuan 2'));
    assert(!matchesOriginalName(mixed, '??'));
});
