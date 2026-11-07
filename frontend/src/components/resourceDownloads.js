export const isDownloadActive = (task) => ['queued', 'downloading', 'importing'].includes(task.status);
export function formatResourceSize(bytes) {
    if (!(bytes > 0)) return '';
    return bytes < 1024 * 1024 ? `${Math.ceil(bytes / 1024)} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
export function resourceDownloadProgress(task) {
    if (task.status === 'done' || task.status === 'importing') return 100;
    return task.total > 0 ? Math.min(99, Math.max(0, Math.floor((task.received / task.total) * 100))) : 0;
}
export function mergeResourceResults(books, resourceType = 'all') {
    const seen = new Set();
    const formats = { txt: ['txt'], epub: ['epub'], kindle: ['mobi', 'azw', 'azw3'] };
    const candidates =
        resourceType === 'all'
            ? books
            : books.map((book) => ({
                  ...book,
                  files: (book.files || []).filter((file) => {
                      const format = (file.archiveFormat || file.format || file.name?.split('.').pop() || '')
                          .trim()
                          .toLowerCase()
                          .split(/[\s(]/)[0];
                      return formats[resourceType]?.includes(format);
                  }),
              }));
    const filtered = candidates.filter((book) => {
        const key = book.sourceId + ':' + book.id;
        if (!book.files?.length || seen.has(key)) return false;
        seen.add(key);
        return true;
    });
    const score = (book) => (book.matchType === 'exact' ? 2 : book.matchType === 'related' ? 0 : 1);
    return filtered.sort((a, b) => score(b) - score(a));
}
