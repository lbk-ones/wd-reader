export function createReadingProgress({getBookName, getContent, read, write,
    schedule = setTimeout, cancel = clearTimeout}) {
    let restoreTimer = null;
    let active = true;
    const save = () => {
        if (!active || restoreTimer !== null) return;
        const book = getBookName();
        const content = getContent();
        if (book && content && Number.isFinite(content.scrollTop)) {
            const top = String(content.scrollTop);
            if (read(book) !== top) write(book, top);
        }
    };
    const stop = (savePosition = true) => {
        if (savePosition) save();
        if (restoreTimer !== null) cancel(restoreTimer);
        restoreTimer = null;
        active = false;
    };
    const restore = book => {
        stop(false);
        active = true;
        const top = Math.max(0, Number(read(book)) || 0);
        restoreTimer = schedule(() => {
            restoreTimer = null;
            const content = getContent();
            if (getBookName() === book && content) content.scrollTop = top;
        }, 100);
    };
    return {save, stop, restore, resume: () => { active = true; }};
}
