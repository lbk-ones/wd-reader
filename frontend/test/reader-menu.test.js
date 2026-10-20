import test from 'node:test';
import assert from 'node:assert/strict';
import {MINI_MENU_GROUPS, MINI_MENU_ACTION_IDS, miniMenuNeedsNormalWindow, runMiniMenuAction} from '../src/components/readerMenu.js';
import {translate} from '../src/i18n/core.js';
import {prepareReadingShortcut} from '../src/components/readingShortcuts.js';

test('mini window exposes every existing context action and an explicit exit from mini mode', () => {
    assert.deepEqual([...MINI_MENU_ACTION_IDS].sort(), ['title-bar', 'preferences', 'next-page', 'previous-page',
        'contents', 'next-chapter', 'previous-chapter', 'library', 'help', 'minimize', 'quit', 'exit-mini'].sort());
    assert.equal(new Set(MINI_MENU_ACTION_IDS).size, 12);
    for (const group of MINI_MENU_GROUPS) {
        for (const label of [group.label, ...group.actions.map(action => action.label)]) {
            assert.notEqual(translate('en', label), label, `missing English translation: ${label}`);
        }
    }
});

test('panels restore a full reader while page and chapter navigation stay in mini mode', () => {
    for (const action of ['contents', 'preferences', 'help', 'library', 'title-bar', 'exit-mini']) {
        assert.equal(miniMenuNeedsNormalWindow(action), true, action);
    }
    for (const action of ['next-page', 'previous-page', 'next-chapter', 'previous-chapter', 'minimize', 'quit']) {
        assert.equal(miniMenuNeedsNormalWindow(action), false, action);
    }
});

test('reader shortcuts pause while the independent menu is open', () => {
    for (const key of ['F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9']) {
        assert.equal(prepareReadingShortcut({key, preventDefault() {}}, {
            currentBookChapterName: 'Chapter 1', miniMenuOpen: true,
        }, true), false, key);
    }
});

test('title bar from mini mode stays visible regardless of the saved normal-window preference', async () => {
    for (const savedTitle of [true, false, undefined]) {
        let mini = true, title = false, savedPreference = savedTitle;
        const calls = [];
        await runMiniMenuAction('title-bar', {
            isMiniMode: mini,
            restoreWindow: async () => {
                await Promise.resolve();
                mini = false;
                title = savedTitle;
                calls.push('restore');
            },
            showTitle: () => { title = true; savedPreference = true; calls.push('show'); },
            dispatch: () => { title = !title; calls.push('toggle'); },
        });
        assert.equal(mini, false);
        assert.equal(title, true);
        assert.equal(savedPreference, true);
        assert.deepEqual(calls, ['restore', 'show']);
    }
});

test('ordinary title toggle still dispatches once and exiting mini mode only restores the saved state', async () => {
    const calls = [];
    const handlers = {isMiniMode: false, restoreWindow: async () => calls.push('restore'),
        showTitle: () => calls.push('show'), dispatch: id => calls.push(id)};
    await runMiniMenuAction('title-bar', handlers);
    assert.deepEqual(calls, ['title-bar']);
    calls.length = 0;
    await runMiniMenuAction('exit-mini', {...handlers, isMiniMode: true});
    assert.deepEqual(calls, ['restore']);
});

test('failed window restoration does not toggle the title or dispatch a second action', async () => {
    await assert.rejects(runMiniMenuAction('title-bar', {
        isMiniMode: true,
        restoreWindow: async () => { throw new Error('resize failed'); },
        showTitle: () => assert.fail('must not show before restoration succeeds'),
        dispatch: () => assert.fail('must not dispatch after a failed resize'),
    }), /resize failed/);
});
