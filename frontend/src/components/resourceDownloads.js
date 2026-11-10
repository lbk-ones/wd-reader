// 排队、传输和入库都属于活动任务，用于轮询、去重和计数。
export const isDownloadActive = (task) => ['queued', 'downloading', 'importing'].includes(task.status);
// 与后端对旧成品来源补写的默认分组一致。
export const defaultSourceGroup = 'download';
const sourceGroupNames = {
    [defaultSourceGroup]: '完本下载',
    main: '常用书源',
    'proxy-required': '可能需要代理',
    'rate-limit': '下载限流',
    'no-search': '仅支持详情链接',
    cloudflare: '可能要求浏览器验证',
};
export const sourceGroups = Object.keys(sourceGroupNames);
// 兼容曾直接使用中文名称作为标识的配置。
export function normalizeSourceGroup(group) {
    const key = typeof group === 'string' ? group.trim() : group;
    return key === '完本下载' ? defaultSourceGroup : key;
}
// 内置分组标识映射为显示名称，用户自定义组名直接展示。
export function sourceGroupName(group) {
    const key = normalizeSourceGroup(group) || defaultSourceGroup;
    return Object.hasOwn(sourceGroupNames, key) ? sourceGroupNames[key] : key;
}
// 文件大小未知时留空，已知时按字节数换算为 KB 或 MB。
export function formatResourceSize(bytes) {
    if (!(bytes > 0)) return '';
    return bytes < 1024 * 1024 ? `${Math.ceil(bytes / 1024)} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
// 章节数和字节数采用同一套百分比算法。传输阶段最多显示 99%，进入入库阶段才显示 100%。
export function resourceDownloadProgress(task) {
    if (task.status === 'done' || task.status === 'importing') return 100;
    return task.total > 0 ? Math.min(99, Math.max(0, Math.floor((task.received / task.total) * 100))) : 0;
}
// 按来源和书籍标识去重。完本下载优先，同一优先级内仍按书名匹配程度排序。
export function mergeResourceResults(books, resourceType = 'all', downloadSourceIds = new Set()) {
    const seen = new Set();
    const formats = { txt: ['txt'], epub: ['epub'], kindle: ['mobi', 'azw', 'azw3'] };
    const candidates =
        resourceType === 'all'
            ? books
            : books.map((book) => ({
                  ...book,
                  files: (book.files || []).filter((file) => {
                      // ZIP 按解压后要导入的格式筛选。普通文件优先使用后端给出的格式。
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
    const downloadPriority = (book) => Number(downloadSourceIds.has(book.sourceId));
    return filtered.sort((a, b) => downloadPriority(b) - downloadPriority(a) || score(b) - score(a));
}
