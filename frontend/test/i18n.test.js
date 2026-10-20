import test from 'node:test';
import assert from 'node:assert/strict';
import {createLanguageStore, LANGUAGE_KEY, resolveLanguage, translate, translateBackendMessage} from '../src/i18n/core.js';
import en from '../src/i18n/en.js';
import enHelp from '../src/i18n/en-help.js';

test('locale selection respects saved preferences, detects English variants and falls back to Chinese', () => {
    assert.equal(resolveLanguage('zh-CN', 'en-US'), 'zh-CN');
    assert.equal(resolveLanguage('en', 'zh-CN'), 'en');
    for (const browser of ['en', 'en-US', 'en-GB', 'EN-au']) {
        assert.equal(resolveLanguage(null, browser), 'en');
    }
    for (const browser of ['zh-CN', 'zh-TW', 'fr-FR', '']) {
        assert.equal(resolveLanguage('invalid', browser), 'zh-CN');
    }
});

test('switching is immediate, persists across restarts and can sync another window', () => {
    const saved = new Map();
    const storage = {getItem: key => saved.get(key), setItem: (key, value) => saved.set(key, value)};
    let updates = 0;
    const store = createLanguageStore({storage, browserLanguage: 'zh-CN', onChange: () => updates++});
    store.setLanguage('en');
    assert.equal(store.getLanguage(), 'en');
    assert.equal(updates, 1);
    assert.equal(saved.get(LANGUAGE_KEY), 'en');
    assert.equal(createLanguageStore({storage}).getLanguage(), 'en');
    saved.set(LANGUAGE_KEY, 'zh-CN');
    store.sync();
    assert.equal(store.getLanguage(), 'zh-CN');
    store.setLanguage('invalid');
    assert.equal(store.getLanguage(), 'zh-CN');
    assert.equal(updates, 2);
    saved.clear();
    const englishSystem = createLanguageStore({storage, browserLanguage: 'en-GB'});
    assert.equal(englishSystem.getLanguage(), 'en');
});

test('restricted storage does not prevent switching languages', () => {
    const unavailable = () => { throw new Error('Storage blocked'); };
    const store = createLanguageStore({storage: {getItem: unavailable, setItem: unavailable}});
    store.setLanguage('en');
    assert.equal(store.getLanguage(), 'en');
});

test('translations interpolate values without translating user content or interpreting replacement syntax', () => {
    const name = '我的书 $& {1}';
    assert.equal(translate('en', '当前章节：{0}，打开目录', {0: name}), `Current chapter: ${name}, open contents`);
    assert.equal(translate('zh-CN', '第 {0} 章，共 {1} 章', {0: 2, 1: 10}), '第 2 章，共 10 章');
    assert.equal(translate('en', name), name);
    assert.equal(translate('en', 'constructor'), 'constructor');
    assert.equal(translate('en', '__proto__'), '__proto__');
    assert.equal(translate('en', '成功导入 {count} 份文件', {count: 0}), 'Imported 0 files');
});

test('all catalog translations preserve interpolation placeholders and have English text', () => {
    const placeholders = text => (text.match(/\{\w+\}/g) || []).sort();
    for (const [source, translation] of Object.entries({...en, ...enHelp})) {
        assert.equal(typeof translation, 'string', source);
        assert(translation.length > 0, source);
        assert.deepEqual(placeholders(translation), placeholders(source), source);
    }
});

test('known backend errors are localized for display while unknown diagnostics are retained', () => {
    const raw = '错误信息:章节不存在或没有正文，请重新选择章节';
    assert.equal(translateBackendMessage('en', raw), 'Error: This chapter is missing or empty. Select another chapter.');
    assert.equal(translateBackendMessage('zh-CN', raw), raw);
    assert.equal(translateBackendMessage('en', '错误信息:open C:/我的书.txt: access denied'), 'Error: open C:/我的书.txt: access denied');
    assert.equal(translateBackendMessage('en', '错误信息:断章正则无效: missing ]'), 'Error: Invalid chapter regex: missing ]');
    assert.equal(translateBackendMessage('en', '未找到.opf文件'), 'Could not find a .opf file');
});
