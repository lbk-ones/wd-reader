import { Progress } from 'antd';
import { t, useI18n } from '../../i18n/index.jsx';
import { formatResourceSize, resourceDownloadProgress } from '../resourceDownloads.js';

export default function ResourceDownloadProgress({ task }) {
    useI18n();
    const percent = resourceDownloadProgress(task);
    const known = task.total > 0 || task.status === 'done' || task.status === 'importing';
    return (
        <div className="resource-task-transfer">
            <div className="resource-task-progress">
                <Progress
                    size="small"
                    percent={percent}
                    status={task.status === 'error' ? 'exception' : task.status === 'downloading' ? 'active' : 'normal'}
                    strokeColor={task.status === 'error' ? '#c74e45' : '#476654'}
                    trailColor="#e2e7df"
                    showInfo={false}
                    aria-label={t('下载进度')}
                />
                <span className="resource-task-percent" title={known ? undefined : t('文件总大小未知')}>
                    {known ? `${percent}%` : '—'}
                </span>
            </div>
            <div className="resource-task-size">
                {formatResourceSize(task.received)}
                {task.total > 0 && ` / ${formatResourceSize(task.total)}`}
            </div>
        </div>
    );
}
