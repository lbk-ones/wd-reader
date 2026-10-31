export function sortBooksByLastOpened(names, metadata) {
    return [...names].sort((a, b) => {
        const aTime = Date.parse(metadata[a]?.lastOpenedAt || metadata[a]?.importedAt) || 0;
        const bTime = Date.parse(metadata[b]?.lastOpenedAt || metadata[b]?.importedAt) || 0;
        return bTime - aTime || a.localeCompare(b);
    });
}

export function getCurrentlyReadingBook(names, metadata) {
    return sortBooksByLastOpened(names, metadata).find(name => {
        const opened = Date.parse(metadata[name]?.lastOpenedAt);
        const imported = Date.parse(metadata[name]?.importedAt);
        return Number.isFinite(opened) && Number.isFinite(imported) && opened > imported;
    }) || null;
}
