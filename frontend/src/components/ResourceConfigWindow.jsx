import { useEffect, useRef, useState } from 'react';
import { Alert, App, Button, Empty, Popconfirm, Space, Tag, Typography, message } from 'antd';
import {
    CloseOutlined,
    DeleteOutlined,
    PlusOutlined,
    ReloadOutlined,
    SaveOutlined,
    UndoOutlined,
} from '@ant-design/icons';
import { Call } from '@wailsio/runtime';
import { SourceKind } from '../../bindings/wd-reader/internal/resourcecatalog/models.js';
import { t, useI18n } from '../i18n/index.jsx';
import ResourceSourceForm, { readConfigDraft, sourceKindLabels, writeConfigDraft } from './ResourceSourceForm.jsx';
import { defaultSourceGroup, sourceGroupName } from './resourceDownloads.js';
import './ResourceConfigWindow.css';

const invoke = (name, ...args) => Call.ByName(`wd-reader/internal/desktop.App.${name}`, ...args);

// 统一管理成品下载和逐章抓取配置，负责草稿、保存状态及配置窗口的生命周期。
export default function ResourceConfigWindow() {
    useI18n();
    const { modal } = App.useApp();
    const [draft, setDraft] = useState(null);
    const [selected, setSelected] = useState(0);
    const [saved, setSaved] = useState('');
    const [path, setPath] = useState('');
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const [notice, setNotice] = useState('');
    const saveRef = useRef(null);
    // 与最近成功加载或保存的快照对比，判断表单中是否还有未保存的改动。
    const snapshot = draft ? JSON.stringify(draft) : '';
    const dirty = snapshot !== saved;
    const source = draft?.sources[selected];
    const busy = loading || saving;

    // 从磁盘装载配置草稿。后端返回原文时，也可展示可解析但规则无效的配置供修复。
    async function load() {
        setLoading(true);
        setError('');
        setNotice('');
        try {
            const next = await invoke('GetResourceSources');
            setPath(next.path);
            const loaded = readConfigDraft(next.content);
            setDraft(loaded);
            setSaved(JSON.stringify(loaded));
            setSelected(0);
            setError(next.error || '');
        } catch (failure) {
            setError(String(failure));
        } finally {
            setLoading(false);
        }
    }
    useEffect(() => {
        load();
    }, []);
    useEffect(() => {
        document.title = `${t('资源配置')} · wd-reader`;
        const unload = (event) => {
            if (dirty) {
                event.preventDefault();
                event.returnValue = '';
            }
        };
        window.addEventListener('beforeunload', unload);
        return () => window.removeEventListener('beforeunload', unload);
    }, [dirty]);
    // 保存当前草稿。后端校验并写入成功后更新快照，并通过广播通知独立页面刷新。
    async function save() {
        if (loading || saving || !dirty) return;
        setSaving(true);
        setError('');
        try {
            await invoke('SaveResourceSources', writeConfigDraft(draft));
            setSaved(snapshot);
            setNotice('');
            if (typeof BroadcastChannel === 'function') {
                const channel = new BroadcastChannel('wd-reader-resource-sources');
                channel.postMessage('saved');
                channel.close();
            }
            message.success(t('资源配置已保存'));
        } catch (failure) {
            setError(String(failure));
        } finally {
            setSaving(false);
        }
    }
    // 键盘监听只注册一次，通过 ref 调用当前渲染对应的保存函数。
    saveRef.current = save;
    useEffect(() => {
        const keydown = (event) => {
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
                event.preventDefault();
                saveRef.current?.();
            }
        };
        window.addEventListener('keydown', keydown);
        return () => window.removeEventListener('keydown', keydown);
    }, []);
    // 默认配置先进入草稿。用户确认保存后才写入磁盘。
    async function restore() {
        setLoading(true);
        try {
            const defaults = await invoke('GetDefaultResourceSources');
            setDraft(readConfigDraft(defaults));
            setSelected(0);
            setError('');
            setNotice(t('默认配置已载入，点击保存后生效。'));
        } catch (failure) {
            setError(String(failure));
        } finally {
            setLoading(false);
        }
    }
    // 重新加载会替换当前草稿，有修改时先确认是否丢弃。
    function reload() {
        if (!dirty) {
            load();
            return;
        }
        modal.confirm({ title: t('放弃未保存的修改？'), content: t('重新加载会读取磁盘上的配置。'), onOk: load });
    }
    // 原生窗口由后端隐藏以保留草稿，浏览器中的独立页面由浏览器关闭。
    async function close() {
        try {
            if (!(await invoke('CloseResourceConfig'))) window.close();
        } catch (failure) {
            setError(String(failure));
        }
    }
    // 在草稿末尾追加一个空白成品来源并切换到该条目。
    function addSource() {
        const current = draft || { version: 2, sources: [] };
        setSelected(current.sources.length);
        setDraft({
            ...current,
            sources: [
                ...current.sources,
                {
                    id: '',
                    name: '',
                    enabled: true,
                    kind: SourceKind.HTMLKind,
                    group: defaultSourceGroup,
                    description: '',
                },
            ],
        });
    }
    // 用编辑后的条目替换当前来源，其他来源继续沿用原草稿。
    function updateSource(next) {
        setDraft((current) => ({
            ...current,
            sources: current.sources.map((item, index) => (index === selected ? next : item)),
        }));
    }
    // 移除当前来源后，将选中位置限制在剩余条目的索引范围内。
    function removeSource() {
        const sources = draft.sources.filter((_, index) => index !== selected);
        setDraft({ ...draft, sources });
        setSelected(Math.max(0, Math.min(selected, sources.length - 1)));
    }
    return (
        <main className="resource-config-window">
            <header className="resource-config-toolbar">
                <div>
                    <h1>{t('资源配置')}</h1>
                    <span>{t('编辑来源与下载规则，保存后立即生效。')}</span>
                </div>
                <Space wrap>
                    <Button icon={<ReloadOutlined />} onClick={reload} disabled={loading || saving}>
                        {t('重新加载')}
                    </Button>
                    <Button icon={<UndoOutlined />} onClick={restore} disabled={loading || saving}>
                        {t('恢复默认设置')}
                    </Button>
                    <Button
                        type="primary"
                        icon={<SaveOutlined />}
                        onClick={save}
                        loading={saving}
                        disabled={loading || !dirty}
                    >
                        {t('保存配置')}
                    </Button>
                    <Button icon={<CloseOutlined />} onClick={close} aria-label={t('关闭配置窗口')} />
                </Space>
            </header>
            <div className="resource-config-location">
                <Typography.Text copyable={!!path}>{path || t('正在加载配置…')}</Typography.Text>
            </div>
            {error && <Alert type="error" showIcon message={t('配置未保存')} description={error} />}
            {notice && <Alert type="info" showIcon message={notice} />}
            <div className="resource-config-workspace">
                <aside className="resource-config-sidebar">
                    <div className="resource-config-sidebar-heading">
                        <span>
                            {t('下载来源')} <Tag>{draft?.sources.length ?? 0}</Tag>
                        </span>
                        <Button
                            size="small"
                            icon={<PlusOutlined />}
                            onClick={addSource}
                            disabled={busy || draft?.sources.length >= 100}
                        >
                            {t('新增来源')}
                        </Button>
                    </div>
                    {draft?.sources.map((item, index) => (
                        <button
                            type="button"
                            className="resource-config-source"
                            aria-pressed={selected === index}
                            key={index}
                            onClick={() => setSelected(index)}
                        >
                            <span>
                                <strong>{item.name || item.id || t('未命名来源')}</strong>
                                <small>
                                    {t(sourceGroupName(item.group))} ·{' '}
                                    {t(sourceKindLabels[item.kind] || item.kind || '—')}
                                </small>
                            </span>
                            {!item.enabled && <Tag>{t('已禁用')}</Tag>}
                        </button>
                    ))}
                </aside>
                <section className="resource-config-editor-pane">
                    {source ? (
                        <>
                            <div className="resource-config-editor-heading">
                                <strong>{t('编辑来源')}</strong>
                                <Popconfirm title={t('移除当前来源？')} onConfirm={removeSource}>
                                    <Button size="small" type="text" danger icon={<DeleteOutlined />} disabled={busy}>
                                        {t('移除来源')}
                                    </Button>
                                </Popconfirm>
                            </div>
                            <ResourceSourceForm
                                key={selected}
                                source={source}
                                groups={draft.sources.map((item) => item.group)}
                                onChange={updateSource}
                                disabled={busy}
                            />
                        </>
                    ) : (
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('点击新增来源开始配置')} />
                    )}
                </section>
            </div>
            <footer className="resource-config-footer">
                <span>{t(dirty ? '有未保存的修改' : '与磁盘配置一致')}</span>
                <span>Ctrl / ⌘ + S · {t('保存配置')}</span>
            </footer>
        </main>
    );
}
