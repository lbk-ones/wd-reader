export const MINI_MENU_GROUPS = [
    {label: '翻页与章节', actions: [
        {id: 'previous-page', label: '上一页', shortcut: 'F3'},
        {id: 'next-page', label: '下一页', shortcut: 'F4'},
        {id: 'previous-chapter', label: '上一章', shortcut: 'F5'},
        {id: 'next-chapter', label: '下一章', shortcut: 'F6'},
    ]},
    {label: '阅读与窗口', actions: [
        {id: 'contents', label: '目录', shortcut: 'F2'},
        {id: 'preferences', label: '阅读设置'},
        {id: 'title-bar', label: '标题开关'},
        {id: 'exit-mini', label: '退出迷你模式', shortcut: 'F8'},
        {id: 'stop-auto-scroll', label: '取消自动滚动'},
        {id: 'stop-auto-page', label: '取消自动翻页'},
    ]},
    {label: '更多操作', actions: [
        {id: 'library', label: '返回书架'},
        {id: 'help', label: '使用说明', shortcut: 'F1'},
        {id: 'minimize', label: '最小化'},
        {id: 'quit', label: '退出系统', danger: true},
    ]},
];

export const MINI_MENU_ACTION_IDS = MINI_MENU_GROUPS.flatMap(group => group.actions.map(action => action.id));

export function miniMenuNeedsNormalWindow(id) {
    return ['contents', 'preferences', 'title-bar', 'library', 'help', 'exit-mini'].includes(id);
}

export async function runMiniMenuAction(id, {isMiniMode, restoreWindow, showTitle, dispatch}) {
    if (isMiniMode && miniMenuNeedsNormalWindow(id)) await restoreWindow();
    // Mini mode hides the title itself. Restoring the saved window may already
    // reveal it, so this command must show it explicitly rather than toggle it.
    if (id === 'title-bar' && isMiniMode) return showTitle();
    if (id !== 'exit-mini') return dispatch(id);
}
