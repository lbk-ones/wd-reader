export function sortBooksByImportTime(names, metadata) {
    return [...names].sort((a, b) => {
        const aTime = Date.parse(metadata[a]?.importedAt) || 0;
        const bTime = Date.parse(metadata[b]?.importedAt) || 0;
        return bTime - aTime || a.localeCompare(b);
    });
}
