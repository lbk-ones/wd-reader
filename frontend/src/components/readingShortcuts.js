export function prepareReadingShortcut(event, state, display, hasModal = () => false, settings = {}) {
    if (
        !/^(F[2-9]|PageUp|PageDown|ArrowUp|ArrowDown|ArrowLeft|ArrowRight)$/.test(event.key) ||
        event.defaultPrevented ||
        event.ctrlKey ||
        event.altKey ||
        event.metaKey ||
        event.shiftKey
    )
        return false;
    if (
        (['ArrowUp', 'ArrowDown'].includes(event.key) && settings.arrowPage === '0' && settings.autoScroll !== '1') ||
        (['ArrowLeft', 'ArrowRight'].includes(event.key) && settings.arrowChapter === '0')
    )
        return false;
    // F5 must not turn into refresh when a reading panel blocks navigation.
    if (state.currentBookChapterName && event.key === 'F5') event.preventDefault();
    if (
        !display ||
        !state.currentBookChapterName ||
        state.loadingBook ||
        state.settingVisible ||
        state.helpVisible ||
        state.sysSettingVisible ||
        state.downloadFromUrlVisible ||
        state.gotoMuluIndexSearchVisible ||
        state.miniMenuOpen ||
        event.target?.closest(
            'input, textarea, select, [contenteditable="true"], .ant-modal, .ant-select-dropdown, [role="textbox"]',
        ) ||
        hasModal() ||
        (state.muluVisible && event.key !== 'F2')
    )
        return false;
    event.preventDefault();
    return !event.repeat;
}
