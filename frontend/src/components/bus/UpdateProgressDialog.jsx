import { useEffect, useState } from 'react';
import { Alert, Button, Modal, Progress, Space, Spin } from 'antd';
import { Browser } from '@wailsio/runtime';
import { t, useI18n } from '../../i18n/index.jsx';
import { formatUpdateBytes, updateDownloadPercent } from '../updateInstallation.js';

const labels = {
    preparing: '正在准备更新…',
    checking: '正在获取更新信息…',
    downloading: '正在下载更新…',
    verifying: '正在校验更新文件…',
    installing: '正在准备安装…',
    restarting: '正在安装并重启，请稍候…',
    cancelling: '正在取消更新…',
    cancelled: '更新已取消',
    failed: '更新失败',
};

export default function UpdateProgressDialog({ progress, onCancel, onRetry, onClose }) {
    useI18n();
    const [now, setNow] = useState(Date.now);
    const busy = progress && !['failed', 'cancelled'].includes(progress.stage);
    useEffect(() => {
        if (!busy) return;
        const timer = setInterval(() => setNow(Date.now()), 1000);
        return () => clearInterval(timer);
    }, [busy]);
    if (!progress) return null;

    const { stage, info, written, total, rate, error, lastActivity } = progress;
    const percent = updateDownloadPercent(progress);
    const slow = busy && ['checking', 'downloading'].includes(stage) && now - lastActivity >= 20000;
    return (
        <Modal
            open
            title={t('更新至 {0}', { 0: info.latestVersion })}
            width={420}
            closable={!busy}
            maskClosable={!busy}
            keyboard={!busy}
            onCancel={onClose}
            footer={
                <Space wrap>
                    <Button onClick={() => Browser.OpenURL(info.url)}>{t('查看发布页')}</Button>
                    {busy ? (
                        <Button
                            loading={stage === 'cancelling'}
                            disabled={!['checking', 'downloading', 'cancelling'].includes(stage)}
                            onClick={onCancel}
                        >
                            {t('取消更新')}
                        </Button>
                    ) : (
                        <>
                            <Button onClick={onClose}>{t('关闭')}</Button>
                            <Button type="primary" onClick={() => onRetry(info)}>
                                {t('重试更新')}
                            </Button>
                        </>
                    )}
                </Space>
            }
        >
            <div className="update-progress-status" role="status" aria-live="polite">
                {busy && <Spin size="small" />}
                <strong>{t(labels[stage])}</strong>
            </div>
            {stage === 'downloading' && (
                <>
                    <Progress percent={percent ?? 0} showInfo={percent !== null} status="active" />
                    <p className="update-progress-detail">
                        {total > 0
                            ? t('已下载 {0} / {1} · {2}/s', {
                                  0: formatUpdateBytes(written),
                                  1: formatUpdateBytes(total),
                                  2: formatUpdateBytes(rate),
                              })
                            : t('已下载 {0} · {1}/s', { 0: formatUpdateBytes(written), 1: formatUpdateBytes(rate) })}
                    </p>
                </>
            )}
            {slow && (
                <Alert
                    type="warning"
                    showIcon
                    message={t('网络响应较慢，正在等待。你可以取消后重试，或前往发布页下载安装包。')}
                />
            )}
            {stage === 'failed' && (
                <Alert
                    type="error"
                    showIcon
                    message={t('安装更新失败，请重试或前往发布页下载安装包')}
                    description={
                        <details>
                            <summary>{t('错误详情')}</summary>
                            <pre className="update-progress-error">{error}</pre>
                        </details>
                    }
                />
            )}
            {stage === 'restarting' && (
                <p className="update-progress-detail">{t('更新文件已下载并通过校验，程序将自动重新打开。')}</p>
            )}
        </Modal>
    );
}
