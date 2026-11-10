import { useEffect, useMemo, useRef, useState } from 'react';
import {
    Alert,
    Button,
    Checkbox,
    Drawer,
    Empty,
    Input,
    InputNumber,
    Modal,
    Select,
    Space,
    Spin,
    Switch,
    Tabs,
    Tag,
    Tooltip,
    message,
} from 'antd';
import {
    CheckCircleFilled,
    ClearOutlined,
    ClockCircleOutlined,
    CloseCircleFilled,
    DownOutlined,
    DownloadOutlined,
    ExclamationCircleFilled,
    ExportOutlined,
    ReloadOutlined,
    SettingOutlined,
    UpOutlined,
} from '@ant-design/icons';
import { Browser, Call, Events } from '@wailsio/runtime';
import { SourceKind } from '../../../bindings/wd-reader/internal/resourcecatalog/models.js';
import { t, useI18n } from '../../i18n/index.jsx';
import {
    defaultSourceGroup,
    formatResourceSize,
    isDownloadActive,
    mergeResourceResults,
    normalizeSourceGroup,
    sourceGroupName,
} from '../resourceDownloads.js';
import './ResourceDownloads.css';
import ResourceDownloadProgress from './ResourceDownloadProgress.jsx';

const invoke = (name, ...args) => Call.ByName(`wd-reader/internal/desktop.App.${name}`, ...args);
const taskLabels = {
    queued: '等待下载',
    downloading: '正在下载',
    importing: '正在加入书架',
    done: '已加入书架',
    cancelled: '已取消',
    error: '下载失败',
};
const healthLabels = {
    available: '可访问',
    blocked: '访问受限',
    unavailable: '无法访问',
};
const healthIcons = {
    available: CheckCircleFilled,
    blocked: ExclamationCircleFilled,
    unavailable: CloseCircleFilled,
};
const resultPageSize = 10;
const defaultDownloadConcurrency = 5;
const maxDownloadConcurrency = 16;
// 输入框先区分书名和链接，详情地址是否受支持由后端的书源规则判断。
const isNovelLink = (value) => /^https?:\/\//i.test(value.trim());
// 抓取来源未配置搜索地址时仍可解析详情链接，只禁用书名搜索勾选项。
const isURLOnlySource = (source) => source.kind === SourceKind.CrawlerKind && !source.searchUrl;
const canSearchSource = (source) => source.enabled && !isURLOnlySource(source);
// 与后端的任务去重维度一致，同一详情地址可以由不同抓取规则分别处理。
const fileKey = (file) => `${file.novelSourceId || ''}:${file.url}`;
const downloadDisplayName = (name, hideExtension) => (hideExtension ? name.replace(/\.epub$/i, '') : name);

// 资源下载面板管理来源选择、站点检测、搜索结果和后台下载任务。
export default function ResourceDownloads({ open, onClose, onDownloaded, disabled }) {
    useI18n();
    const [config, setConfig] = useState(null);
    const [selected, setSelected] = useState([]);
    const [query, setQuery] = useState('');
    const [resourceType, setResourceType] = useState('all');
    const [searched, setSearched] = useState('');
    const [books, setBooks] = useState([]);
    const [resultLimit, setResultLimit] = useState(resultPageSize);
    // 搜索状态与访问检测分开记录。首页可访问时，搜索接口仍可能限流或报错。
    const [statuses, setStatuses] = useState({});
    const [health, setHealth] = useState({});
    const [showSources, setShowSources] = useState(false);
    const [sourceGroup, setSourceGroup] = useState(defaultSourceGroup);
    const [healthRefresh, setHealthRefresh] = useState(0);
    const [resolving, setResolving] = useState(false);
    const [tasks, setTasks] = useState([]);
    const [starting, setStarting] = useState('');
    const [downloadOptions, setDownloadOptions] = useState(null);
    const [tab, setTab] = useState('books');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [nativeConfig, setNativeConfig] = useState(null);
    useEffect(() => {
        invoke('ResourceConfigUsesNativeWindow')
            .then(setNativeConfig)
            .catch((failure) => setError(String(failure)));
    }, []);
    // 分开记录配置和查询的代号，清除结果时可以忽略旧查询响应，同时让正在读取的配置完成。
    const configGeneration = useRef(0);
    const searchGeneration = useRef(0);
    const seenCompleted = useRef(new Set());
    const downloadedRef = useRef(onDownloaded);
    downloadedRef.current = onDownloaded;
    const active = tasks.some(isDownloadActive);
    const busy = resolving || Object.values(statuses).some((s) => s.state === 'loading');
    const selectedIds = useMemo(() => new Set(selected), [selected]);
    const searchableIds = useMemo(
        () => (config?.sources || []).filter(canSearchSource).map((source) => source.id),
        [config],
    );
    const allSelected = searchableIds.length > 0 && searchableIds.every((id) => selectedIds.has(id));
    const groupedSources = useMemo(() => {
        // 先登记完本组，使它排在首位，同时保留其余分组在配置中的顺序。
        const groups = new Map([[defaultSourceGroup, []]]);
        for (const source of config?.sources || []) {
            const group = normalizeSourceGroup(source.group) || defaultSourceGroup;
            if (!groups.has(group)) groups.set(group, []);
            groups.get(group).push(source);
        }
        if (!groups.get(defaultSourceGroup).length) groups.delete(defaultSourceGroup);
        return groups;
    }, [config]);
    const downloadSourceIds = useMemo(
        () => new Set(groupedSources.get(defaultSourceGroup)?.map((source) => source.id)),
        [groupedSources],
    );
    const filteredBooks = useMemo(
        () => mergeResourceResults(books, resourceType, downloadSourceIds),
        [books, resourceType, downloadSourceIds],
    );
    // 书名结果随勾选项筛选，直接粘贴的详情页结果可以来自未勾选的书源。
    const matchingBooks = useMemo(
        () => (isNovelLink(searched) ? filteredBooks : filteredBooks.filter((book) => selectedIds.has(book.sourceId))),
        [filteredBooks, searched, selectedIds],
    );
    // 保留已检索的结果，只把当前批次范围内的卡片加入界面。
    const displayedBooks = matchingBooks.slice(0, resultLimit);
    const hasMore = displayedBooks.length < matchingBooks.length;
    const sourceResultCounts = useMemo(() => {
        const counts = new Map();
        for (const { sourceId } of filteredBooks) counts.set(sourceId, (counts.get(sourceId) || 0) + 1);
        return counts;
    }, [filteredBooks]);

    // 读取当前配置并恢复默认勾选。仅支持详情链接的来源不参与书名查询。
    async function load() {
        const token = ++configGeneration.current;
        searchGeneration.current++;
        setStatuses({});
        setResolving(false);
        setLoading(true);
        setError('');
        try {
            const next = await invoke('GetResourceSources');
            if (token !== configGeneration.current) return;
            setConfig(next);
            setError(next.error || '');
            setSelected(
                next.sources
                    .filter((s) => canSearchSource(s) && (s.kind !== SourceKind.CrawlerKind || s.group === 'main'))
                    .map((s) => s.id),
            );
        } catch (failure) {
            if (token === configGeneration.current) setError(String(failure));
        } finally {
            if (token === configGeneration.current) setLoading(false);
        }
    }
    // 面板打开时读取配置，关闭时让仍在等待的加载或搜索结果失效。
    useEffect(() => {
        if (open) load();
        return () => {
            configGeneration.current++;
            searchGeneration.current++;
        };
    }, [open]);
    // 使用有限数量的工作循环检测所有已启用来源，每个来源完成后单独更新状态。
    useEffect(() => {
        if (!open || !config) return;
        const sources = config.sources.filter((source) => source.enabled);
        const pending = new Set();
        let disposed = false;
        let next = 0;
        setHealth({});
        // 每个工作循环从共享索引领取下一条来源，完成后继续处理未检查项。
        async function check() {
            while (!disposed && next < sources.length) {
                const { id } = sources[next++];
                const call = invoke('CheckResourceSource', id);
                pending.add(call);
                try {
                    const result = await call;
                    if (!disposed) setHealth((current) => ({ ...current, [id]: result }));
                } catch (failure) {
                    if (!disposed) {
                        setHealth((current) => ({
                            ...current,
                            [id]: { status: 'unavailable', detail: String(failure) },
                        }));
                    }
                } finally {
                    pending.delete(call);
                }
            }
        }
        for (let worker = 0; worker < Math.min(sources.length, 8); worker++) void check();
        return () => {
            // 关闭面板、修改配置或手动重测时取消上一批仍在执行的请求。
            disposed = true;
            pending.forEach((call) => call.cancel());
        };
    }, [open, config, healthRefresh]);
    // 面板关闭后，仍为活动任务轮询状态，以便下载入库时及时刷新书架。
    useEffect(() => {
        if (!open && !active) return;
        let disposed = false;
        let timer;
        async function refresh() {
            try {
                const next = await invoke('ListResourceDownloads');
                if (disposed) return;
                setTasks(next);
                // 已通知过的任务仍留在历史中，按任务标识避免重复刷新书架。
                const completed = next.filter((task) => task.status === 'done' && !seenCompleted.current.has(task.id));
                if (completed.length) {
                    completed.forEach((task) => seenCompleted.current.add(task.id));
                    downloadedRef.current?.();
                }
            } catch (failure) {
                if (!disposed) setError(String(failure));
            } finally {
                if (!disposed) timer = setTimeout(refresh, 800);
            }
        }
        refresh();
        return () => {
            disposed = true;
            clearTimeout(timer);
        };
    }, [open, active]);

    // 详情链接交给后端匹配抓取来源，书名则在当前勾选的来源中并行查询。
    async function search(value) {
        const keyword = value.trim();
        const byLink = isNovelLink(keyword);
        if (loading || !keyword || (!byLink && !selected.length)) return;
        const token = ++searchGeneration.current;
        setTab('books');
        setSearched(keyword);
        setBooks([]);
        setResultLimit(resultPageSize);
        setError('');
        setResolving(byLink);
        if (byLink) {
            setStatuses({});
            try {
                const book = await invoke('ResolveResourceNovel', keyword);
                if (token !== searchGeneration.current) return;
                // 详情页返回 EPUB 下载项，清掉上次搜索留下的格式筛选以确保结果可见。
                setResourceType('all');
                setBooks([book]);
            } catch (failure) {
                if (token === searchGeneration.current) setError(String(failure));
            } finally {
                if (token === searchGeneration.current) setResolving(false);
            }
            return;
        }
        setStatuses(Object.fromEntries(selected.map((id) => [id, { state: 'loading' }])));
        // 各来源独立提交结果和错误，较慢或失败的来源不会挡住其他书源的结果。
        await Promise.allSettled(
            selected.map(async (id) => {
                try {
                    const result = await invoke('FindDownloadableBooks', id, keyword);
                    if (token !== searchGeneration.current) return;
                    setBooks((current) => [...current, ...result.books]);
                    setStatuses((current) => ({
                        ...current,
                        [id]: { state: 'done', count: result.books.length, failed: result.failed },
                    }));
                } catch (failure) {
                    if (token === searchGeneration.current)
                        setStatuses((current) => ({ ...current, [id]: { state: 'error', error: String(failure) } }));
                }
            }),
        );
    }
    // 清空本次结果并让尚未返回的查询失效，避免清除后又出现旧条目。
    function clearResults() {
        searchGeneration.current++;
        setSearched('');
        setBooks([]);
        setStatuses({});
        setResolving(false);
        setResultLimit(resultPageSize);
        setError(config?.error || '');
    }
    function isOutsideDownloadGroup(file) {
        if (file?.sourceGroup) return normalizeSourceGroup(file.sourceGroup) !== defaultSourceGroup;
        return !!file?.novelSourceId && !downloadSourceIds.has(file.novelSourceId);
    }
    function requestDownload(file, outsideDownloadGroup = isOutsideDownloadGroup(file)) {
        if (disabled || starting) return;
        // 章节抓取任务保存显式协程数，重试时可恢复设置。
        if (outsideDownloadGroup && file.novelSourceId) {
            setDownloadOptions({
                file,
                parallel: file.concurrency !== 1,
                count: file.concurrency > 1 ? file.concurrency : defaultDownloadConcurrency,
            });
            return;
        }
        void startDownload(file);
    }
    // 把下载描述提交给后端队列，收到任务快照后立即显示在下载标签页。
    async function startDownload(file) {
        if (disabled || starting) return;
        setStarting(fileKey(file));
        try {
            const task = await invoke('StartResourceDownload', file);
            setTasks((current) => [task, ...current.filter((item) => item.id !== task.id)]);
            setTab('tasks');
        } catch (failure) {
            message.error(String(failure));
        } finally {
            setStarting('');
        }
    }
    // 请求后端取消任务，并读取最新快照以反映队列或入库状态。
    async function cancel(task) {
        try {
            await invoke('CancelResourceDownload', task.id);
            setTasks(await invoke('ListResourceDownloads'));
        } catch (failure) {
            message.error(String(failure));
        }
    }
    // 打开桌面原生配置窗口，浏览器版本则由按钮链接打开独立页面。
    async function openConfig() {
        try {
            await invoke('OpenResourceConfig');
        } catch (failure) {
            message.error(String(failure));
        }
    }
    // 原生窗口用 Wails 事件通知配置变动，独立页面用 BroadcastChannel。
    useEffect(() => {
        if (!open) return;
        const off = Events.On('resource-sources-changed', () => load());
        const channel =
            typeof BroadcastChannel === 'function' ? new BroadcastChannel('wd-reader-resource-sources') : null;
        if (channel) channel.onmessage = () => load();
        return () => {
            off();
            channel?.close();
        };
    }, [open]);

    // 书源条目同时显示是否可勾选、站点检测结果及当前搜索结果数量。
    function renderSource(source) {
        const status = statuses[source.id];
        const reachability = health[source.id];
        const state = reachability?.status || 'checking';
        const label = t(healthLabels[state] || '检测中');
        const HealthIcon = healthIcons[state] || ClockCircleOutlined;
        const urlOnly = isURLOnlySource(source);
        const count = sourceResultCounts.get(source.id) || 0;
        return (
            <div className="resource-source-chip" key={source.id}>
                <Tooltip title={source.description || source.homeUrl}>
                    <Checkbox
                        checked={selectedIds.has(source.id)}
                        disabled={!canSearchSource(source) || loading}
                        onChange={(event) =>
                            setSelected((current) =>
                                event.target.checked
                                    ? [...current, source.id]
                                    : current.filter((id) => id !== source.id),
                            )
                        }
                    >
                        {source.name}
                    </Checkbox>
                </Tooltip>
                {urlOnly && <Tag>{t('详情链接')}</Tag>}
                {source.enabled && (
                    <Tooltip title={reachability?.detail ? `${label} · ${reachability.detail}` : label}>
                        <HealthIcon className={`resource-source-health is-${state}`} aria-label={label} />
                    </Tooltip>
                )}
                {status?.state === 'loading' && <Spin size="small" />}
                {status?.state === 'done' && (
                    <Tag color={count ? 'green' : 'default'} title={t('{0} 项可下载资源', { 0: count })}>
                        {count}
                    </Tag>
                )}
                {status?.failed > 0 && (
                    <Tooltip title={t('部分文件列表未能读取')}>
                        <Tag color="gold">!</Tag>
                    </Tooltip>
                )}
                {!source.enabled && <Tag>{t('已禁用')}</Tag>}
                {status?.state === 'error' && (
                    <Tooltip title={status.error}>
                        <Tag color="red" aria-label={t('资源查询失败')}>
                            !
                        </Tag>
                    </Tooltip>
                )}
            </div>
        );
    }

    const sources = config?.sources || [];
    const enabledSources = sources.filter((source) => source.enabled);
    const availableCount = enabledSources.filter((source) => health[source.id]?.status === 'available').length;
    const checking = enabledSources.some((source) => !health[source.id]);
    // 配置修改后若原分组已不存在，回退到仍可用的第一组。
    const visibleGroup = groupedSources.has(sourceGroup) ? sourceGroup : groupedSources.keys().next().value;
    const concurrencyCount = downloadOptions?.count;
    const validConcurrency =
        Number.isInteger(concurrencyCount) && concurrencyCount >= 2 && concurrencyCount <= maxDownloadConcurrency;

    const results = (
        <>
            <div className="resource-search-toolbar">
                <Input.Search
                    value={query}
                    maxLength={2048}
                    allowClear
                    aria-label={t('书名或小说详情链接')}
                    placeholder={t('书名或小说详情链接')}
                    onChange={(event) => setQuery(event.target.value)}
                    onSearch={search}
                    enterButton={t('查找资源')}
                    className="resource-search-input"
                    loading={resolving}
                    disabled={loading || !config}
                />
                <Select
                    aria-label={t('资源类型')}
                    value={resourceType}
                    onChange={(value) => {
                        setResourceType(value);
                        setResultLimit(resultPageSize);
                    }}
                    className="resource-type-select"
                    options={[
                        { value: 'all', label: t('全部') },
                        { value: 'txt', label: 'txt' },
                        { value: 'epub', label: 'epub' },
                        { value: 'kindle', label: 'mobi/azw/azw3' },
                    ]}
                />
            </div>
            {sources.length > 0 && (
                <section className="resource-sources">
                    <div className="resource-health-toolbar">
                        <Button
                            size="small"
                            type="text"
                            icon={showSources ? <UpOutlined /> : <DownOutlined />}
                            aria-expanded={showSources}
                            onClick={() => setShowSources((value) => !value)}
                        >
                            {t('书源（已选 {0}）', { 0: selected.length })}
                        </Button>
                        <Button
                            size="small"
                            type="link"
                            className="resource-select-all"
                            disabled={loading || !searchableIds.length}
                            onClick={() => setSelected(allSelected ? [] : searchableIds)}
                        >
                            {t(allSelected ? '取消全选' : '全选')}
                        </Button>
                        {enabledSources.length > 0 && (
                            <div className="resource-health-status">
                                <Tooltip title={t('自动检测站点访问状态，搜索和章节下载可能受站点限制。')}>
                                    <small aria-live="polite">
                                        {checking
                                            ? t('检测中')
                                            : t('{0}/{1} 可访问', { 0: availableCount, 1: enabledSources.length })}
                                    </small>
                                </Tooltip>
                                <Tooltip title={t('重新检测')}>
                                    <Button
                                        size="small"
                                        type="text"
                                        icon={<ReloadOutlined />}
                                        aria-label={t('重新检测')}
                                        disabled={loading}
                                        onClick={() => setHealthRefresh((current) => current + 1)}
                                    />
                                </Tooltip>
                            </div>
                        )}
                    </div>
                    {showSources && (
                        <div className="resource-source-panel">
                            <div className="resource-source-groups" role="group" aria-label={t('书源分组')}>
                                {Array.from(groupedSources, ([group, members]) => {
                                    const label = t(sourceGroupName(group));
                                    return (
                                        <button
                                            key={group}
                                            type="button"
                                            className="resource-source-group"
                                            aria-pressed={visibleGroup === group}
                                            title={label}
                                            onClick={() => setSourceGroup(group)}
                                        >
                                            <span>{label}</span>
                                            <small>{members.length}</small>
                                        </button>
                                    );
                                })}
                            </div>
                            <div className="resource-catalog-sources">
                                {groupedSources.get(visibleGroup)?.map(renderSource)}
                            </div>
                        </div>
                    )}
                </section>
            )}
            {searched && (
                <div className="resource-results-summary">
                    {busy && <Spin size="small" />}
                    <span aria-live="polite">
                        {hasMore
                            ? t('显示 {0}/{1} 项可下载资源', { 0: displayedBooks.length, 1: matchingBooks.length })
                            : t('找到 {0} 项可下载资源', { 0: matchingBooks.length })}
                    </span>
                    <Tooltip title={t('清除查询结果')}>
                        <Button
                            size="small"
                            type="text"
                            icon={<ClearOutlined />}
                            aria-label={t('清除查询结果')}
                            onClick={clearResults}
                        />
                    </Tooltip>
                </div>
            )}
            {displayedBooks.map((book) => (
                <article className="download-resource-book" key={book.sourceId + ':' + book.id}>
                    <div className="resource-book-heading">
                        <div className="resource-book-heading-text">
                            <h3>
                                {downloadDisplayName(book.title, !downloadSourceIds.has(book.sourceId))}{' '}
                                {book.matchType === 'related' && <Tag>{t('相关作品')}</Tag>}
                            </h3>
                            <p className="resource-source-description">
                                {book.author || t('作者未提供')} · {book.sourceName}
                            </p>
                        </div>
                        <Tooltip title={t('查看资源详情')}>
                            <Button
                                size="small"
                                type="text"
                                icon={<ExportOutlined />}
                                aria-label={t('查看资源详情')}
                                onClick={() =>
                                    Browser.OpenURL(book.url).catch((failure) => message.error(String(failure)))
                                }
                            />
                        </Tooltip>
                    </div>
                    <div className="resource-file-list">
                        {book.files.map((file) => {
                            const outsideDownloadGroup = !downloadSourceIds.has(book.sourceId);
                            const name = downloadDisplayName(file.name, outsideDownloadGroup);
                            const task = tasks.find(
                                (item) =>
                                    fileKey(item.file || item) === fileKey(file) &&
                                    (isDownloadActive(item) || item.status === 'done'),
                            );
                            return (
                                <div className="resource-file-row" key={file.url}>
                                    <div className="resource-file-info">
                                        <Tag>{file.format}</Tag>
                                        <Tooltip title={name}>
                                            <span className="resource-file-name">{name}</span>
                                        </Tooltip>
                                        <small>{formatResourceSize(file.size)}</small>
                                    </div>
                                    <Button
                                        size="small"
                                        type={file.format === 'EPUB' ? 'primary' : 'default'}
                                        icon={<DownloadOutlined />}
                                        disabled={disabled || !!task || !!starting}
                                        loading={starting === fileKey(file)}
                                        onClick={() => requestDownload(file, outsideDownloadGroup)}
                                    >
                                        {task ? t(taskLabels[task.status]) : t('下载')}
                                    </Button>
                                </div>
                            );
                        })}
                    </div>
                </article>
            ))}
            {hasMore && (
                <div className="resource-results-more">
                    <Button
                        size="small"
                        icon={<DownOutlined />}
                        onClick={() => setResultLimit((limit) => limit + resultPageSize)}
                    >
                        {t('更多')}
                    </Button>
                </div>
            )}
            {!busy && !matchingBooks.length && (
                <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description={
                        searched ? t('未找到可下载文件，可换一个书名或来源重试') : t('选择来源，输入书名开始查找')
                    }
                />
            )}
        </>
    );
    const downloads = (
        <>
            <p className="resource-download-intro">{t('关闭面板后继续下载；正在加入书架时请等待完成。')}</p>
            {tasks.map((task) => {
                const name = downloadDisplayName(task.name, isOutsideDownloadGroup(task.file));
                return (
                    <article className="download-resource-book" key={task.id}>
                        <div className="resource-task-heading">
                            <h3 title={name}>{name}</h3>
                            <Tag color={task.status === 'done' ? 'green' : task.status === 'error' ? 'red' : 'default'}>
                                {t(taskLabels[task.status])}
                            </Tag>
                            {['queued', 'downloading'].includes(task.status) && (
                                <Button size="small" onClick={() => cancel(task)}>
                                    {t('取消下载')}
                                </Button>
                            )}
                            {['error', 'cancelled'].includes(task.status) && (
                                <Button
                                    size="small"
                                    disabled={disabled || !!starting}
                                    onClick={() => requestDownload(task.file || { url: task.url, name: task.name })}
                                >
                                    {t('重新下载')}
                                </Button>
                            )}
                        </div>
                        <ResourceDownloadProgress task={task} />
                        {task.error && <Alert type="error" showIcon message={task.error} />}
                    </article>
                );
            })}
            {!tasks.length && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无下载任务')} />}
        </>
    );
    return (
        <Drawer
            title={t('资源下载')}
            open={open}
            width={560}
            onClose={(event) => {
                setDownloadOptions(null);
                onClose?.(event);
            }}
            className="resource-download-drawer"
            extra={
                <Space>
                    <Button
                        size="small"
                        icon={<ReloadOutlined />}
                        loading={loading}
                        onClick={load}
                        aria-label={t('重新加载来源')}
                    />
                    <Button
                        size="small"
                        icon={<SettingOutlined />}
                        disabled={nativeConfig === null}
                        href={nativeConfig === false ? '?view=resource-config' : undefined}
                        target={nativeConfig === false ? 'wd-reader-resource-config' : undefined}
                        rel="noopener"
                        onClick={nativeConfig ? openConfig : undefined}
                    >
                        {t('资源配置')}
                    </Button>
                </Space>
            }
        >
            {error && <Alert type="error" showIcon message={t('资源操作失败')} description={error} />}
            <Tabs
                size="small"
                activeKey={tab}
                onChange={setTab}
                items={[
                    { key: 'books', label: t('查找电子书'), children: results },
                    {
                        key: 'tasks',
                        label: t('下载任务（{0}）', { 0: tasks.filter(isDownloadActive).length }),
                        children: downloads,
                    },
                ]}
            />
            {downloadOptions && (
                <Modal
                    open={open}
                    centered
                    width={360}
                    title={t('下载设置')}
                    okText={t('开始下载')}
                    cancelText={t('取消')}
                    okButtonProps={{ disabled: disabled || (downloadOptions.parallel && !validConcurrency) }}
                    onCancel={() => setDownloadOptions(null)}
                    onOk={() => {
                        const { file, parallel, count } = downloadOptions;
                        setDownloadOptions(null);
                        void startDownload({ ...file, concurrency: parallel ? count : 1 });
                    }}
                >
                    <div className="resource-download-options">
                        <p className="resource-file-name" title={downloadDisplayName(downloadOptions.file.name, true)}>
                            {downloadDisplayName(downloadOptions.file.name, true)}
                        </p>
                        <label>
                            {t('启用章节并发抓取')}
                            <Switch
                                size="small"
                                checked={downloadOptions.parallel}
                                aria-label={t('启用章节并发抓取')}
                                onChange={(parallel) => setDownloadOptions((current) => ({ ...current, parallel }))}
                            />
                        </label>
                        <label>
                            {t('协程数量')}
                            <InputNumber
                                size="small"
                                min={2}
                                max={maxDownloadConcurrency}
                                precision={0}
                                value={concurrencyCount}
                                disabled={!downloadOptions.parallel}
                                aria-label={t('协程数量')}
                                onChange={(count) => setDownloadOptions((current) => ({ ...current, count }))}
                            />
                        </label>
                    </div>
                </Modal>
            )}
        </Drawer>
    );
}
