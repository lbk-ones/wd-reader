import { useEffect, useRef, useState } from 'react';
import { Alert, App as AntdApp, Button, Space, Spin } from 'antd';
import { DownloadOutlined, UploadOutlined } from '@ant-design/icons';
import { Events, Window } from '@wailsio/runtime';
import { t, useI18n } from '../../i18n/index.jsx';
import {
    ExportResources,
    PrepareResourceImport,
    CommitResourceImport,
    DiscardResourceImport,
    LogError,
} from '../../../bindings/wd-reader/internal/desktop/app.js';
import { mergeFrontendCache, snapshotFrontendCache } from '../resourceCache.js';

const stageLabels = {
    exporting: '正在打包资源…',
    validating: '正在校验资源包…',
    importing: '正在合并资源…',
    selecting: '请选择文件位置…',
};

export default function ResourceTransfer({ disabled, beforeTransfer, afterTransfer, onBusyChange }) {
    useI18n();
    const { modal } = AntdApp.useApp();
    const [busy, setBusy] = useState('');
    const [progress, setProgress] = useState(null);
    const [error, setError] = useState('');
    const [exportedPath, setExportedPath] = useState('');
    const busyRef = useRef(false);
    useEffect(
        () =>
            Events.On('reader:resource-progress', (event) => {
                if (busyRef.current) setProgress(event.data);
            }),
        [],
    );

    async function transfer(mode) {
        if (disabled || busyRef.current) return;
        busyRef.current = true;
        onBusyChange(true);
        setBusy(mode);
        setProgress({ stage: 'selecting' });
        setError('');
        setExportedPath('');
        let token;
        let cacheChange;
        let committed = false;
        try {
            beforeTransfer();
            if (mode === 'export') {
                const result = await ExportResources(snapshotFrontendCache());
                if (result) setExportedPath(result.path);
            } else {
                const preview = await PrepareResourceImport();
                if (!preview) return;
                token = preview.token;
                // Browser quota failures happen before the library is touched.
                cacheChange = mergeFrontendCache(preview.cache, preview.bookNames);
                setProgress({ stage: 'importing' });
                const result = await CommitResourceImport(token);
                committed = true;
                window.dispatchEvent(new Event('wd-cache-change'));
                modal.success({
                    title: t('资源导入完成'),
                    content: t('新增 {0} 个文件，跳过 {1} 个相同文件，重命名 {2} 个同名文件，合并 {3} 项缓存。', {
                        0: result.filesAdded,
                        1: result.filesSkipped,
                        2: result.filesRenamed,
                        3: cacheChange.count,
                    }),
                    okText: t('重新加载'),
                    keyboard: false,
                    closable: false,
                    maskClosable: false,
                    onOk: () => Window.ForceReload(),
                });
            }
        } catch (failure) {
            let diagnostic = String(failure);
            if (cacheChange && !committed) {
                try {
                    cacheChange.rollback();
                } catch (rollbackError) {
                    diagnostic += '\n' + String(rollbackError);
                }
            }
            setError(diagnostic);
            LogError(diagnostic).catch(() => {});
        } finally {
            if (token) await DiscardResourceImport(token).catch((failure) => LogError(String(failure)).catch(() => {}));
            afterTransfer();
            busyRef.current = false;
            setBusy('');
            setProgress(null);
            onBusyChange(false);
        }
    }

    return (
        <section className="app-settings-section">
            <h3>{t('资源导入与导出')}</h3>
            <p className="resource-transfer-description">
                {t('导出 books 目录和全部前端缓存，包括书籍、封面、分类、收藏、阅读进度及设置。')}
            </p>
            <p className="resource-transfer-description">
                {t('导入时合并当前资源：相同文件跳过，同名不同内容保留两份；已有设置和进度优先保留。')}
            </p>
            <Space wrap>
                <Button
                    size="small"
                    icon={<DownloadOutlined />}
                    loading={busy === 'export'}
                    disabled={disabled || !!busy}
                    onClick={() => transfer('export')}
                >
                    {t('导出资源')}
                </Button>
                <Button
                    size="small"
                    icon={<UploadOutlined />}
                    loading={busy === 'import'}
                    disabled={disabled || !!busy}
                    onClick={() => transfer('import')}
                >
                    {t('导入资源并合并')}
                </Button>
            </Space>
            {progress && (
                <div className="resource-transfer-status" role="status">
                    <Spin size="small" />
                    <span>
                        {t(stageLabels[progress.stage] || '正在合并资源…')}
                        {progress.total > 0 && ` ${progress.completed} / ${progress.total}`}
                    </span>
                </div>
            )}
            {exportedPath && (
                <Alert
                    type="success"
                    showIcon
                    message={t('资源已导出')}
                    description={<code className="resource-transfer-path">{exportedPath}</code>}
                />
            )}
            {error && (
                <Alert
                    type="error"
                    showIcon
                    message={t('资源操作失败，请查看详情后重试')}
                    description={
                        <details>
                            <summary>{t('错误详情')}</summary>
                            <pre className="update-progress-error">{error}</pre>
                        </details>
                    }
                />
            )}
        </section>
    );
}
