import { useEffect, useMemo, useRef, useState } from 'react';
import {
    Alert,
    Button,
    Checkbox,
    Drawer,
    Empty,
    Input,
    Progress,
    Select,
    Space,
    Spin,
    Tabs,
    Tag,
    Tooltip,
    message,
} from 'antd';
import { DownloadOutlined, ExportOutlined, ReloadOutlined, SettingOutlined } from '@ant-design/icons';
import { Browser, Call, Events } from '@wailsio/runtime';
import { t, useI18n } from '../../i18n/index.jsx';
import {
    formatResourceSize,
    isDownloadActive,
    mergeResourceResults,
    resourceDownloadProgress,
} from '../resourceDownloads.js';
import './ResourceDownloads.css';

const invoke = (name, ...args) => Call.ByName(`wd-reader/internal/desktop.App.${name}`, ...args);
const taskLabels = {
    queued: '等待下载',
    downloading: '正在下载',
    importing: '正在加入书架',
    done: '已加入书架',
    cancelled: '已取消',
    error: '下载失败',
};

export default function ResourceDownloads({ open, onClose, onDownloaded, disabled }) {
    useI18n();
    const [config, setConfig] = useState(null);
    const [selected, setSelected] = useState([]);
    const [query, setQuery] = useState('');
    const [resourceType, setResourceType] = useState('all');
    const [searched, setSearched] = useState('');
    const [books, setBooks] = useState([]);
    const [statuses, setStatuses] = useState({});
    const [tasks, setTasks] = useState([]);
    const [starting, setStarting] = useState('');
    const [tab, setTab] = useState('books');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [nativeConfig, setNativeConfig] = useState(null);
    useEffect(() => {
        invoke('ResourceConfigUsesNativeWindow')
            .then(setNativeConfig)
            .catch((failure) => setError(String(failure)));
    }, []);
    const generation = useRef(0);
    const seenCompleted = useRef(new Set());
    const downloadedRef = useRef(onDownloaded);
    downloadedRef.current = onDownloaded;
    const active = tasks.some(isDownloadActive);
    const busy = Object.values(statuses).some((s) => s.state === 'loading');
    const filteredBooks = useMemo(() => mergeResourceResults(books, resourceType), [books, resourceType]);
    const visibleBooks = useMemo(
        () => filteredBooks.filter((book) => selected.includes(book.sourceId)),
        [filteredBooks, selected],
    );

    async function load() {
        const token = ++generation.current;
        setStatuses({});
        setLoading(true);
        setError('');
        try {
            const next = await invoke('GetResourceSources');
            if (token !== generation.current) return;
            setConfig(next);
            setError(next.error || '');
            setSelected(next.sources.filter((s) => s.enabled).map((s) => s.id));
        } catch (failure) {
            if (token === generation.current) setError(String(failure));
        } finally {
            if (token === generation.current) setLoading(false);
        }
    }
    useEffect(() => {
        if (open) load();
        return () => {
            generation.current++;
        };
    }, [open]);
    useEffect(() => {
        if (!open && !active) return;
        let disposed = false;
        let timer;
        async function refresh() {
            try {
                const next = await invoke('ListResourceDownloads');
                if (disposed) return;
                setTasks(next);
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

    async function search(value) {
        const keyword = value.trim();
        if (!keyword || !selected.length) return;
        const token = ++generation.current;
        setTab('books');
        setSearched(keyword);
        setBooks([]);
        setError('');
        setStatuses(Object.fromEntries(selected.map((id) => [id, { state: 'loading' }])));
        await Promise.allSettled(
            selected.map(async (id) => {
                try {
                    const result = await invoke('FindDownloadableBooks', id, keyword);
                    if (token !== generation.current) return;
                    setBooks((current) => [...current, ...result.books]);
                    setStatuses((current) => ({
                        ...current,
                        [id]: { state: 'done', count: result.books.length, failed: result.failed },
                    }));
                } catch (failure) {
                    if (token === generation.current)
                        setStatuses((current) => ({ ...current, [id]: { state: 'error', error: String(failure) } }));
                }
            }),
        );
    }
    async function download(file) {
        if (disabled || starting) return;
        setStarting(file.url);
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
    async function cancel(task) {
        try {
            await invoke('CancelResourceDownload', task.id);
            setTasks(await invoke('ListResourceDownloads'));
        } catch (failure) {
            message.error(String(failure));
        }
    }
    async function openConfig() {
        try {
            await invoke('OpenResourceConfig');
        } catch (failure) {
            message.error(String(failure));
        }
    }
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

    const results = (
        <>
            <p className="resource-download-intro">
                {t('输入书名，只列出含可下载文件的资源。下载完成后自动加入书架。')}
            </p>
            <div className="resource-search-toolbar">
                <Input.Search
                    value={query}
                    maxLength={100}
                    allowClear
                    aria-label={t('输入书名查找资源')}
                    placeholder={t('输入书名，例如：红楼梦 / Alice')}
                    onChange={(event) => setQuery(event.target.value)}
                    onSearch={search}
                    enterButton={t('查找资源')}
                    className="resource-search-input"
                    disabled={loading || !config || !selected.length}
                />
                <Select
                    aria-label={t('资源类型')}
                    value={resourceType}
                    onChange={setResourceType}
                    className="resource-type-select"
                    options={[
                        { value: 'all', label: t('全部') },
                        { value: 'txt', label: 'txt' },
                        { value: 'epub', label: 'epub' },
                        { value: 'kindle', label: 'mobi/azw/azw3' },
                    ]}
                />
            </div>
            <div className="resource-catalog-sources">
                {config?.sources.map((source) => {
                    const status = statuses[source.id];
                    const count = filteredBooks.filter((book) => book.sourceId === source.id).length;
                    return (
                        <div className="resource-source-chip" key={source.id}>
                            <Tooltip title={source.description}>
                                <Checkbox
                                    checked={selected.includes(source.id)}
                                    disabled={!source.enabled || loading}
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
                                    <Tag color="red">{t('资源查询失败')}</Tag>
                                </Tooltip>
                            )}
                        </div>
                    );
                })}
            </div>
            {searched && (
                <div className="resource-results-summary">{t('找到 {0} 项可下载资源', { 0: visibleBooks.length })}</div>
            )}
            {visibleBooks.map((book) => (
                <article className="download-resource-book" key={book.sourceId + ':' + book.id}>
                    <div className="resource-book-heading">
                        <div className="resource-book-heading-text">
                            <h3>
                                {book.title} {book.matchType === 'related' && <Tag>{t('相关作品')}</Tag>}
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
                            const task = tasks.find(
                                (item) => item.url === file.url && (isDownloadActive(item) || item.status === 'done'),
                            );
                            return (
                                <div className="resource-file-row" key={file.url}>
                                    <div className="resource-file-info">
                                        <Tag>{file.format}</Tag>
                                        <Tooltip title={file.name}>
                                            <span className="resource-file-name">{file.name}</span>
                                        </Tooltip>
                                        <small>{formatResourceSize(file.size)}</small>
                                    </div>
                                    <Button
                                        size="small"
                                        type={file.format === 'EPUB' ? 'primary' : 'default'}
                                        icon={<DownloadOutlined />}
                                        disabled={disabled || !!task || !!starting}
                                        loading={starting === file.url}
                                        onClick={() => download(file)}
                                    >
                                        {task ? t(taskLabels[task.status]) : t('下载')}
                                    </Button>
                                </div>
                            );
                        })}
                    </div>
                </article>
            ))}
            {!busy && !visibleBooks.length && (
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
            {tasks.map((task) => (
                <article className="download-resource-book" key={task.id}>
                    <div className="resource-task-heading">
                        <h3 title={task.name}>{task.name}</h3>
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
                                onClick={() => download(task.file || { url: task.url, name: task.name })}
                            >
                                {t('重新下载')}
                            </Button>
                        )}
                    </div>
                    <div className="resource-task-progress">
                        <Progress
                            size="small"
                            percent={resourceDownloadProgress(task)}
                            status={
                                task.status === 'error'
                                    ? 'exception'
                                    : task.status === 'downloading'
                                      ? 'active'
                                      : 'normal'
                            }
                            showInfo={task.total > 0 || task.status === 'done'}
                        />
                        <span className="resource-task-size">
                            {formatResourceSize(task.received)}
                            {task.total > 0 && ` / ${formatResourceSize(task.total)}`}
                        </span>
                    </div>
                    {task.error && <Alert type="error" showIcon message={task.error} />}
                </article>
            ))}
            {!tasks.length && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无下载任务')} />}
        </>
    );
    return (
        <Drawer
            title={t('资源下载')}
            open={open}
            width={560}
            onClose={onClose}
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
        </Drawer>
    );
}
