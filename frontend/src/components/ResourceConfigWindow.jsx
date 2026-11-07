import { useEffect, useMemo, useRef, useState } from 'react';
import { Alert, App, Button, Input, Space, Switch, Tag, Typography, message } from 'antd';
import { CloseOutlined, ReloadOutlined, SaveOutlined, UndoOutlined } from '@ant-design/icons';
import { Call } from '@wailsio/runtime';
import { t, useI18n } from '../i18n/index.jsx';
import './ResourceConfigWindow.css';

const invoke = (name, ...args) => Call.ByName(`wd-reader/internal/desktop.App.${name}`, ...args);

export default function ResourceConfigWindow() {
    useI18n();
    const { modal } = App.useApp();
    const [content, setContent] = useState('');
    const [saved, setSaved] = useState('');
    const [path, setPath] = useState('');
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const [notice, setNotice] = useState('');
    const saveRef = useRef(null);
    const dirty = content !== saved;
    const parsed = useMemo(() => {
        try {
            const config = JSON.parse(content);
            return Array.isArray(config.sources) &&
                config.sources.every(
                    (source) =>
                        source &&
                        typeof source === 'object' &&
                        !Array.isArray(source) &&
                        ['id', 'name', 'kind'].every(
                            (key) => source[key] === undefined || typeof source[key] === 'string',
                        ),
                )
                ? config
                : null;
        } catch {
            return null;
        }
    }, [content]);

    async function load() {
        setLoading(true);
        setError('');
        setNotice('');
        try {
            const next = await invoke('GetResourceSources');
            setContent(next.content);
            setSaved(next.content);
            setPath(next.path);
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
    async function save() {
        if (loading || saving || !dirty) return;
        setSaving(true);
        setError('');
        try {
            await invoke('SaveResourceSources', content);
            setSaved(content);
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
    async function restore() {
        setLoading(true);
        try {
            const defaults = await invoke('GetDefaultResourceSources');
            setContent(JSON.stringify(JSON.parse(defaults), null, 2));
            setError('');
            setNotice(t('默认配置已载入，点击保存后生效。'));
        } catch (failure) {
            setError(String(failure));
        } finally {
            setLoading(false);
        }
    }
    function reload() {
        if (!dirty) {
            load();
            return;
        }
        modal.confirm({ title: t('放弃未保存的修改？'), content: t('重新加载会读取磁盘上的配置。'), onOk: load });
    }
    function format() {
        try {
            setContent(JSON.stringify(JSON.parse(content), null, 2));
            setError('');
        } catch (failure) {
            setError(String(failure));
        }
    }
    async function close() {
        try {
            if (!(await invoke('CloseResourceConfig'))) window.close();
        } catch (failure) {
            setError(String(failure));
        }
    }
    function toggleSource(index, enabled) {
        const next = {
            ...parsed,
            sources: parsed.sources.map((source, i) => (i === index ? { ...source, enabled } : source)),
        };
        setContent(JSON.stringify(next, null, 2));
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
                        {t('下载来源')} <Tag>{parsed?.sources.length ?? '—'}</Tag>
                    </div>
                    {parsed?.sources.map((source, index) => (
                        <div className="resource-config-source" key={index}>
                            <div>
                                <strong>{source.name || source.id || '—'}</strong>
                                <small>{source.kind || '—'}</small>
                            </div>
                            <Switch
                                size="small"
                                checked={!!source.enabled}
                                disabled={loading || saving}
                                aria-label={`${t('启用来源')} ${source.name || source.id}`}
                                onChange={(checked) => toggleSource(index, checked)}
                            />
                        </div>
                    ))}
                    {!parsed && <p>{t('输入有效的 JSON 后显示来源。')}</p>}
                </aside>
                <section className="resource-config-editor-pane">
                    <div className="resource-config-editor-heading">
                        <strong>resource-sources.json</strong>
                        <Button size="small" type="text" onClick={format} disabled={loading || saving}>
                            {t('格式化 JSON')}
                        </Button>
                    </div>
                    <Input.TextArea
                        value={content}
                        onChange={(event) => setContent(event.target.value)}
                        disabled={loading || saving}
                        spellCheck={false}
                        aria-label={t('资源配置内容')}
                        className="resource-config-code"
                    />
                </section>
            </div>
            <footer className="resource-config-footer">
                <span>{t(dirty ? '有未保存的修改' : '与磁盘配置一致')}</span>
                <span>Ctrl / ⌘ + S · {t('保存配置')}</span>
            </footer>
        </main>
    );
}
