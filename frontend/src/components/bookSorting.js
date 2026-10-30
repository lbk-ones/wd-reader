export function sortBooksByLastOpened(names, metadata) {
    return [...names].sort((a, b) => {
        const aTime = Date.parse(metadata[a]?.lastOpenedAt || metadata[a]?.importedAt) || 0;
        const bTime = Date.parse(metadata[b]?.lastOpenedAt || metadata[b]?.importedAt) || 0;
        return bTime - aTime || a.localeCompare(b);
    });
}
