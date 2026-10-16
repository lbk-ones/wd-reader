import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {calculateReadingLayout} from '../src/components/readingLayout.js';
import {createReadingProgress} from '../src/components/readingProgress.js';
import {createAutoPageTimer} from '../src/components/autoPageTimer.js';
import {prepareReadingShortcut} from '../src/components/readingShortcuts.js';
import {calculatePageTurn, turnReaderPage} from '../src/components/readingPagination.js';

test('page turns correct wheel offsets in both directions without skipping the edge line', () => {
    const turn = (scrollTop, direction) => calculatePageTurn({scrollTop, direction,
        viewportHeight: 300, contentHeight: 3000, lineHeight: 30, origin: 90});
    assert.equal(turn(13, 1).top, 300);
    assert.equal(turn(313, -1).top, 30);
    assert.equal(turn(613, -1).top, 330);
    assert.equal(turn(300, 1).top, 600);
    assert.equal(turn(600, -1).top, 300);
    for (let top = 0; top < 2500; top += 7.3) {
        const down = turn(top, 1).top;
        const up = turn(top, -1).top;
        assert.equal(down % 30, 0);
        assert.equal(up % 30, 0);
        assert(down <= top + 300 + 1e-6 && down > top);
        assert(up >= top - 300 - 1e-6 && up <= top);
    }
});

test('page ends, short chapters, compact windows and fractional line heights stay aligned', () => {
    const last = calculatePageTurn({scrollTop: 553, direction: 1,
        viewportHeight: 300, contentHeight: 977, lineHeight: 30, origin: 90});
    assert.deepEqual(last, {top: 690, pageHeight: 300, paddingBottom: 13});
    assert.equal((977 + last.paddingBottom - last.pageHeight) % 30, 0);
    const short = calculatePageTurn({scrollTop: 0, direction: 1,
        viewportHeight: 300, contentHeight: 120, lineHeight: 30});
    assert.deepEqual(short, {top: 0, pageHeight: 300, paddingBottom: 0});
    assert.equal(calculatePageTurn({scrollTop: 13, direction: 1,
        viewportHeight: 30, contentHeight: 900, lineHeight: 30}).top, 30);
    const fractional = calculatePageTurn({scrollTop: 17.2, direction: 1,
        viewportHeight: 320, contentHeight: 3150, lineHeight: 31.5, origin: 94.5});
    assert.equal(fractional.top, 315);
    assert.equal(fractional.pageHeight, 315);
    assert.equal(calculatePageTurn({scrollTop: 0, direction: 1,
        viewportHeight: 0, contentHeight: 300, lineHeight: 30}), null);
});

test('DOM page turns use real paragraph offsets and do not accumulate chapter-end padding', () => {
    const content = {scrollTop: 553, clientHeight: 300, clientTop: 0,
        style: {paddingBottom: '0px'},
        get scrollHeight() {return 977 + parseFloat(this.style.paddingBottom);},
        getBoundingClientRect: () => ({top: 20}),
        querySelector: () => ({getBoundingClientRect: () => ({top: 20 + 90 - content.scrollTop})}),
        scrollTo({top}) {this.scrollTop = Math.min(top, this.scrollHeight - this.clientHeight);},
    };
    turnReaderPage(content, 30, 1);
    assert.equal(content.scrollTop, 690);
    assert.equal(content.style.paddingBottom, '13px');
    turnReaderPage(content, 30, 1);
    assert.equal(content.style.paddingBottom, '13px');
    turnReaderPage(content, 30, -1);
    assert.equal(content.scrollTop, 390);
});

function clock(repeat = false) {
    let now = 0, id = 0;
    const tasks = new Map();
    return {
        tasks,
        schedule(fn, delay) { tasks.set(++id, {fn, delay, at: now + delay}); return id; },
        cancel(key) { tasks.delete(key); },
        advance(ms) {
            const end = now + ms;
            for (;;) {
                const next = [...tasks].sort((a, b) => a[1].at - b[1].at)[0];
                if (!next || next[1].at > end) break;
                const [key, task] = next;
                now = task.at;
                if (repeat) task.at += task.delay;
                else tasks.delete(key);
                task.fn();
            }
            now = end;
        },
    };
}

test('layout returns safely for hidden windows and fits tiny and ordinary heights', () => {
    for (const height of [0, -30, undefined, NaN, Infinity]) {
        assert.equal(calculateReadingLayout(height, 20, 30), null);
    }
    for (const height of [1, 2, 10, 60, 120, 630]) {
        const layout = calculateReadingLayout(height, 20, 30);
        assert(layout.newContentHeight > 0 && layout.newContentHeight <= height);
        assert(layout.fontSize >= 1 && layout.fontSize <= layout.fontLineHeight);
    }
    assert.deepEqual(calculateReadingLayout(630, 20, 30), {
        fontSize: 20, fontLineHeight: 30, newContentHeight: 630, blankLineHeight: 0,
    });
    assert.equal(calculateReadingLayout(120, 20, 30, 3).fontLineHeight, 40);
});

test('manual activity grants a full new auto-page interval; reset cannot restart a paused timer', () => {
    const time = clock(true);
    let pages = 0;
    const timer = createAutoPageTimer(() => pages++, time);
    timer.start(30000);
    time.advance(29000);
    timer.reset();
    time.advance(1000);
    assert.equal(pages, 0);
    time.advance(28999);
    assert.equal(pages, 0);
    time.advance(1);
    assert.equal(pages, 1);
    assert.equal(time.tasks.size, 1);
    timer.stop();
    timer.reset();
    time.advance(60000);
    assert.equal(pages, 1);
    assert.equal(time.tasks.size, 0);
});

test('progress restoration is single, cancellable and does not overwrite a new book', () => {
    const time = clock();
    const positions = new Map([['a', '320'], ['b', '80']]);
    const content = {scrollTop: 0};
    let book = 'a', writes = 0;
    const progress = createReadingProgress({getBookName: () => book, getContent: () => content,
        read: name => positions.get(name), write: (name, top) => {writes++; positions.set(name, top);},
        schedule: time.schedule, cancel: time.cancel});
    for (let n = 0; n < 10; n++) progress.restore('a');
    assert.equal(time.tasks.size, 1);
    progress.save();
    assert.equal(positions.get('a'), '320');
    book = 'b';
    progress.restore('b');
    time.advance(100);
    assert.equal(content.scrollTop, 80);
    assert.equal(time.tasks.size, 0);
    content.scrollTop = 140;
    progress.save(); progress.save();
    assert.equal(positions.get('b'), '140');
    assert.equal(writes, 1);
    progress.restore('b');
    book = 'a';
    content.scrollTop = 10;
    time.advance(100);
    assert.equal(content.scrollTop, 10);
    progress.stop(false);
    positions.clear();
    progress.save(); // pagehide after clearing records must not recreate them.
    assert.equal(positions.size, 0);
    assert.equal(time.tasks.size, 0);
});

test('F5 cannot refresh reading while panels, loading, hidden content or inputs block shortcuts', () => {
    const makeEvent = patch => ({key: 'F5', preventDefault() { this.defaultPrevented = true; }, ...patch});
    const reading = {currentBookChapterName: '第一章'};
    for (const flag of ['settingVisible', 'muluVisible', 'helpVisible', 'sysSettingVisible',
        'downloadFromUrlVisible', 'gotoMuluIndexSearchVisible', 'loadingBook']) {
        const event = makeEvent();
        assert.equal(prepareReadingShortcut(event, {...reading, [flag]: true}, true), false);
        assert.equal(event.defaultPrevented, true);
    }
    for (const [display, target, modal] of [[false, undefined, false], [true, {closest: () => true}, false], [true, undefined, true]]) {
        const event = makeEvent({target});
        assert.equal(prepareReadingShortcut(event, reading, display, () => modal), false);
        assert.equal(event.defaultPrevented, true);
    }
    const repeat = makeEvent({repeat: true});
    assert.equal(prepareReadingShortcut(repeat, reading, true), false);
    assert.equal(repeat.defaultPrevented, true);
    const library = makeEvent();
    assert.equal(prepareReadingShortcut(library, {}, true), false);
    assert.equal(library.defaultPrevented, undefined);
    const modified = makeEvent({ctrlKey: true});
    assert.equal(prepareReadingShortcut(modified, reading, true), false);
    assert.equal(modified.defaultPrevented, undefined);
    assert.equal(prepareReadingShortcut(makeEvent(), reading, true), true);
    assert.equal(prepareReadingShortcut(makeEvent({key: 'F2'}), {...reading, muluVisible: true}, true), true);
});

test('clearing reading history removes all progress, retains preferences and notifies cache subscribers', async t => {
    // Utils.jsx contains plain JavaScript; a data URL lets Node load it without a JSX loader.
    const source = await readFile(new URL('../src/components/Utils.jsx', import.meta.url), 'utf8');
    const utils = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
    const storage = {'Wd-LastClickBook': 'a.txt,b.txt', 'Wd-a.txt': '第一章',
        'Wd-b.txt': '第二章', 'Wd-scroll-topa.txt': '320', 'Wd-scroll-topdeleted.txt': '90',
        'Wd-libraryLayout': 'grid', 'Wd-fontSize': '20', 'Wd-SplitMap': '{}', unrelated: 'keep'};
    Object.defineProperty(storage, 'removeItem', {value(key) {delete this[key];}});
    const events = new EventTarget();
    const previous = globalThis.window;
    globalThis.window = {localStorage: storage, addEventListener: events.addEventListener.bind(events),
        removeEventListener: events.removeEventListener.bind(events), dispatchEvent: events.dispatchEvent.bind(events)};
    t.after(() => { globalThis.window = previous; });
    let notifications = 0;
    const unsubscribe = utils.subscribeCacheChanges(() => notifications++);
    utils.clearReadingHistory();
    assert.deepEqual(Object.keys(storage).sort(), ['Wd-SplitMap', 'Wd-fontSize', 'Wd-libraryLayout', 'unrelated']);
    assert.equal(utils.getCacheCount(), 3);
    assert.equal(notifications, 1);
    unsubscribe();
});
