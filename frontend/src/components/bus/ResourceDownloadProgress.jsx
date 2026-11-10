import { Progress } from 'antd';
import { t, useI18n } from '../../i18n/index.jsx';
import { formatResourceSize, resourceDownloadProgress } from '../resourceDownloads.js';

// 展示统一的下载进度条，并按任务类型显示章节数或文件大小。
export default function ResourceDownloadProgress({ task }) {
    useI18n();
    const percent = resourceDownloadProgress(task);
    const known = task.total > 0 || task.status === 'done' || task.status === 'importing';
    // 抓取任务在目录解析完成后才得到总章数。在此之前用阶段提示代替百分比。
    const novel = !!task.file?.novelSourceId;
    const size = novel
        ? task.total > 0
            ? t('已完成 {0}/{1} 章', { 0: task.received, 1: task.total })
            : task.status === 'downloading'
              ? t('正在读取章节目录')
              : ''
        : formatResourceSize(task.received) + (task.total > 0 ? ` / ${formatResourceSize(task.total)}` : '');
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
                <span className="resource-task-percent" title={known || novel ? undefined : t('文件总大小未知')}>
                    {known ? `${percent}%` : '—'}
                </span>
            </div>
            <div className="resource-task-size">{size}</div>
        </div>
    );
}
