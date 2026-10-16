export function prepareReadingShortcut(event, state, display, hasModal = () => false) {
    if (!/^F[2-8]$/.test(event.key) || event.defaultPrevented
        || event.ctrlKey || event.altKey || event.metaKey || event.shiftKey) return false;
    // F5 must not turn into refresh when a reading panel blocks navigation.
    if (state.currentBookChapterName && event.key === 'F5') event.preventDefault();
    if (!display || !state.currentBookChapterName || state.loadingBook
        || state.settingVisible || state.helpVisible || state.sysSettingVisible
        || state.downloadFromUrlVisible || state.gotoMuluIndexSearchVisible
        || event.target?.closest('input, textarea, select, [contenteditable="true"], .ant-modal, .ant-select-dropdown, [role="textbox"]')
        || hasModal() || (state.muluVisible && event.key !== 'F2')) return false;
    event.preventDefault();
    return !event.repeat;
}
