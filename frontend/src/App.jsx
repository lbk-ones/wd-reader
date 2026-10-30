import {t, useI18n, localizeError} from './i18n/index.jsx';
import LanguageSelect from './i18n/LanguageSelect.jsx';
import {useCallback, useEffect, useRef, useState, useSyncExternalStore} from 'react';
import pinyin from 'pinyin'
import './App.css';
import {Alert, App as AntdApp, Button, Drawer, Input, message, notification, Space, Spin, theme} from 'antd';
import {presetPalettes} from '@ant-design/colors';
import {filter, find, findIndex, get, isEmpty, throttle, trim} from 'lodash-es'
import {
    AddFile,
    GetBookList,
    GetBooksPath,
    GetChapterListByFileName,
    GetServerUrl,
    GetVersion,
    OpenFileDialog,
    LogError
} from "../bindings/wd-reader/app";
import {GetChapterContentByChapterName} from "../bindings/wd-reader/app.js";
import useAllState from "./components/lib/hooks/UseAllState";
import {Application, Browser, Call, Events, Window} from "@wailsio/runtime";
import {LineOutlined, PlusOutlined} from "@ant-design/icons";
import Header from "./components/bus/Header";
import BookList from "./components/bus/BookList";
import {
    clearCache,
    clearReadingHistory,
    getCacheCount,
    subscribeCacheChanges,
    getCacheItem,
    getSplitMap,
    getSplitType,
    getSplitTypeValue,
    setCacheItem
} from "./components/Utils";
import useMemoizedFn from "ahooks/es/useMemoizedFn";
import MuluList from "./components/bus/MuluList.jsx";
import ContentSetting from "./components/bus/ContentSetting.jsx";
import Content from "./components/bus/Content.jsx";
import UsageGuide from "./components/bus/UsageGuide.jsx";
import {calculateReadingLayout} from "./components/readingLayout.js";
import {createReadingProgress} from "./components/readingProgress.js";
import {prepareReadingShortcut} from "./components/readingShortcuts.js";
import {createAutoPageTimer} from "./components/autoPageTimer.js";
import {turnReaderPage} from "./components/readingPagination.js";
import {createAutoScroll} from "./components/autoScroll.js";
import {DEFAULT_READING_TYPOGRAPHY} from "./components/readingTypography.js";
import {DEFAULT_READING_COLORS} from "./components/readingColors.js";
import {runMiniMenuAction} from './components/readerMenu.js';
import {importedBookName} from './components/importedBookName.js';
import {CloseMiniMenu, OpenMiniMenu} from '../bindings/wd-reader/app.js';


const CACHE_SCROLL_TOP = "scroll-top"


function App() {
    const {language} = useI18n();
    const {modal} = AntdApp.useApp();
    const cacheCount = useSyncExternalStore(subscribeCacheChanges, getCacheCount);
    const [display, setDisplay] = useState(true)
    let pageContentRef = useRef(null);
    let bookListRef = useRef(null);
    let appRef = useRef(null);
    const normalWindowRef = useRef(null);
    const shortcutMiniBusyRef = useRef(false);
    const progressRef = useRef(null);
    const autoPageTimerRef = useRef(null);
    const autoScrollRef = useRef(null);
    const autoScrollSaveTimeRef = useRef(0);

    const [api, contextHolder] = notification.useNotification();

    const [settingState, setSettingState, getSettingState] = useAllState({
        fontColor: getCacheItem('fontColor') || DEFAULT_READING_COLORS.fontColor,
        fontLineHeight: getCacheItem('fontLineHeight') || String(DEFAULT_READING_TYPOGRAPHY.lineHeight),
        blankLineHeight: 0,// 舍弃高度
        newContentHeight: 0,// 总高度
        bgColor: getCacheItem('bgColor') || DEFAULT_READING_COLORS.bgColor,
        fontSize: getCacheItem('fontSize') || String(DEFAULT_READING_TYPOGRAPHY.fontSize),
        clickPage: getCacheItem('clickPage') || '1',
        autoPage: getCacheItem('autoScroll') === '1' ? '0' : getCacheItem('autoPage') || '0',
        autoPageInterval: Math.min(3600, Math.max(1, Math.round(Number(getCacheItem('autoPageInterval')) || 30))),
        autoScroll: getCacheItem('autoScroll') || '0',
        autoScrollSpeed: Math.min(100, Math.max(5, Number(getCacheItem('autoScrollSpeed')) || 20)),
        showProgress: getCacheItem('showProgress') || '1',
        isAlwaysTop: getCacheItem('isAlwaysTop') || '1',
        transparentMode: getCacheItem('transparentMode') || '0',
        leaveWindowHid: getCacheItem('leaveWindowHid') || '0',
        miniMode: getCacheItem('miniMode') || '0',
        miniRows: Math.min(4, Math.max(1, Number(getCacheItem('miniRows')) || 3)),
        appTitle: getCacheItem('AppTitle') || '',
        splitMap: getSplitMap() // 拆分方式
    })

    const [state, setState, getState] = useAllState({
        currentBookList: [],
        currentBookName: "",
        currentBookChapterList: [],
        currentBookChapterName: "",
        currentBookChapterContent: [],
        loadIngBookList: false,
        showTitle: getCacheItem("标题开关") !== 'false',
        settingVisible: false,
        helpVisible: false,
        fontColor: '#000',
        muluVisible: false,
        serverUrl: '',
        loadingBook: false,
        loadingBookTip: "加载中。。",
        errorInfo: '',
        version: '',
        booksPath: '',
        downloadFromUrlVisible: false,
        downloadFromUrlList: [null],
        sysSettingVisible: false,
        lastSearchMulu: -1,
        lastSearchMuluName: "",
        gotoMuluIndexSearchVisible: false,
        oneLine: false,
        miniMenuOpen: false,
    })


    if (progressRef.current === null) {
        progressRef.current = createReadingProgress({
            getBookName: () => getState().currentBookName,
            getContent: () => pageContentRef.current,
            read: book => getCacheItem(CACHE_SCROLL_TOP + book),
            write: (book, top) => setCacheItem(CACHE_SCROLL_TOP + book, top),
        });
    }

    const postponeAutoPage = useMemoizedFn(() => {
        autoPageTimerRef.current?.reset();
    });

    const setAutoReadingMode = useMemoizedFn((mode, enabled) => {
        const patch = {[mode]: enabled ? '1' : '0'};
        if (enabled) patch[mode === 'autoPage' ? 'autoScroll' : 'autoPage'] = '0';
        Object.entries(patch).forEach(([key, value]) => setCacheItem(key, value));
        setSettingState(patch);
    });
    const pauseAutoScroll = useMemoizedFn(() => autoScrollRef.current?.pause());

    const handleReadingShortcut = useMemoizedFn(event => {
        if (!prepareReadingShortcut(event, getState(), display,
            () => document.querySelector('.ant-modal-wrap'))) return;
        switch (event.key) {
            case 'F2':
                handleSelect({id: 'contents'});
                break;
            case 'F3':
                if (pageContentRef.current) pageUp();
                break;
            case 'F4':
                if (pageContentRef.current) pageDown();
                break;
            case 'F5':
                lastChpater();
                break;
            case 'F6':
                nextChpater();
                break;
            case 'F7': {
                const autoPage = getSettingState().autoPage === '1' ? '0' : '1';
                setAutoReadingMode('autoPage', autoPage === '1');
                message.info(autoPage === '1' ? t("自动翻页已开启，每 {0} 秒翻页", {0: getSettingState().autoPageInterval}) : t("自动翻页已关闭"));
                break;
            }
            case 'F9': {
                const enabled = getSettingState().autoScroll !== '1';
                setAutoReadingMode('autoScroll', enabled);
                message.info(enabled ? t("自动滚动已开启") : t("自动滚动已关闭"));
                break;
            }
            case 'F8':
                if (shortcutMiniBusyRef.current) return;
                shortcutMiniBusyRef.current = true;
                miniSize(getSettingState().miniRows, getSettingState().miniMode !== '1')
                    .catch(error => message.error(String(error)))
                    .finally(() => {
                        shortcutMiniBusyRef.current = false;
                    });
                break;
        }
    });

    function handlerAddFileRes(res, list, editAfterImport = false) {
        if (!hasError(res)) {
            try {
                let parse = JSON.parse(res);
                let entries = Object.entries(parse);
                let errorLength = entries.length;
                let allLength = list.length;
                entries.forEach((value, index, array) => {
                    let key = value[0];
                    let Value = value[1];
                    api.info({
                        message: key,
                        description: localizeError(Value),
                        placement: "bottomRight"
                    });
                })
                setState({
                    loadingBook: true,
                    loadingBookTip: "加载中。。",
                })
                reloadBookList(names => {
                    if (editAfterImport && entries.length === 0 && list.length === 1) {
                        const name = importedBookName(list[0]);
                        if (names.includes(name)) bookListRef.current?.editImportedBook(name);
                    }
                });
                message.info(t('成功导入 {count} 份文件', {count: allLength - errorLength}))
            } catch (e) {
                setState({loadingBook: false});
                message.error(String(e));
            }
        } else {
            setState({loadingBook: false});
        }
    }

    function AddFleAndHandlerRes(paths, editAfterImport = false) {
        paths = (paths || []).filter(Boolean)
        setState({
            loadingBook: true,
            loadingBookTip: "解析中。。,非txt和 epub解析时间随文件大小递增",
        })
        AddFile(paths).then(res => {
            handlerAddFileRes(res, paths, editAfterImport);
        }).catch(error => {
            setState({loadingBook: false});
            message.error(String(error));
        });
    }

    function handlerError(e) {
        LogError(e.error?.stack || e.message || String(e)).catch(console.error)
    }

    function unhandledrejection(e) {
        LogError("unhandledrejection:" + e.reason).catch(console.error)
    }

    useEffect(function () {
        // error log collect
        window.addEventListener("error", handlerError)
        window.addEventListener('unhandledrejection', unhandledrejection)
        GetVersion().then(res => {
            setState({
                version: res
            })
        })
        GetBooksPath().then(res => {
            setState({
                booksPath: res
            })
        })
        isAlwaysTop(null)
        reloadBookList();

        GetServerUrl().then(res => {
            setState({
                serverUrl: res
            })
        })
        const offFileDrop = Events.On("files-dropped", event => {
            AddFleAndHandlerRes(event.data);
        });
        const offMiniAction = Events.On('mini-menu-action', event => {
            handleMiniMenuAction(event.data);
        });
        const offMiniClosed = Events.On('mini-menu-closed', () => {
            setState({miniMenuOpen: false});
            if (getSettingState().leaveWindowHid === '1' && !appRef.current?.matches(':hover')) {
                setDisplay(false);
            }
        });

        function handleReaderKeyDown(event) {
            if (event.key === 'F1') {
                event.preventDefault();
                if (!event.repeat) openUsageGuide();
                return;
            }
            handleReadingShortcut(event);
            if (event.key === 'PageDown' || event.key === 'PageUp') {
                event.preventDefault();
            }
            if (event.key === 'Escape' && !event.repeat && !event.defaultPrevented
                && getSettingState().miniMode === '1' && getState().currentBookName
                && !getState().settingVisible && !getState().muluVisible && !getState().helpVisible
                && !event.target?.closest('input, textarea, select, [contenteditable="true"], .ant-modal, .ant-select-dropdown')) {
                event.preventDefault();
                exitMiniMode();
            }
        }

        document.addEventListener('keydown', handleReaderKeyDown);
        const saveReadingPosition = () => progressRef.current.save();
        window.addEventListener('pagehide', saveReadingPosition);
        window.addEventListener('beforeunload', saveReadingPosition);
        let calculateFontLinesAuto = throttle((e) => {
            calculateFontLines()
        }, 300);

        window.addEventListener("resize", calculateFontLinesAuto)

        return () => {
            window.removeEventListener("error", handlerError)
            window.removeEventListener("unhandledrejection", unhandledrejection)
            offFileDrop()
            offMiniAction();
            offMiniClosed();
            CloseMiniMenu().catch(() => {});
            document.removeEventListener('keydown', handleReaderKeyDown)
            window.removeEventListener('pagehide', saveReadingPosition);
            window.removeEventListener('beforeunload', saveReadingPosition);
            progressRef.current.stop();
            window.removeEventListener("resize", calculateFontLinesAuto)
            calculateFontLinesAuto.cancel()
        }
    }, [])

    function recordTop() {
        const now = performance.now();
        if (getSettingState().autoScroll !== '1' || now - autoScrollSaveTimeRef.current >= 500) {
            progressRef.current.save();
            autoScrollSaveTimeRef.current = now;
        }
        postponeAutoPage();
    }

    function reloadBookList(cb = null) {
        setState({
            loadingBook: true
        })
        GetBookList().then(res => {
            let strings = null;
            if (!hasError(res)) {
                strings = res.split("\n").map(e => e.trim()).filter(Boolean);
                setState({
                    loadingBook: false,
                    currentBookList: strings
                })


                if (typeof cb === 'function') cb(strings)
            } else {
                setState({
                    loadingBook: false
                })
            }

            // 刷新一下缓存
            let item = getCacheItem("LastClickBook");
            if (item) {
                if (!strings) {
                    setCacheItem("LastClickBook", "");
                    return;
                }
                let strings1 = item.split(",");
                let nweCacheList = [];
                strings1.forEach(et => {
                    if (find(strings, e => e === et)) {
                        nweCacheList.push(et);
                    }
                })
                setCacheItem("LastClickBook", nweCacheList.join(","))
            }
        });
    }


    function convertToInitials(arr) {
        return arr.map(item => {
            // 使用 pinyin 库将中文转换为拼音
            const pinyinArray = pinyin(item, {
                style: pinyin.STYLE_FIRST_LETTER  // 只取首字母
            });
            // 将拼音数组转换为字符串
            const initials = pinyinArray.map(letterArray => letterArray[0]).join('');
            return {
                name: item,
                initials: initials.toLowerCase()
            };
        });
    }

    function searchByInitials(query, data) {
        const convertedData = convertToInitials(data);
        return convertedData.filter(item => item.initials.indexOf(query) >= 0);
    }

    // function getContentClientHeight() {
    //     let clientHeight1 = pageContentRef.current.clientHeight;
    //     if (getState().showTitle && getSettingState().transparentMode !== '1') {
    //         clientHeight1 = clientHeight1 - 30;
    //     }
    //
    //     return clientHeight1
    // }

    function pageDown() {
        pauseAutoScroll();
        postponeAutoPage();
        turnReaderPage(pageContentRef.current, getSettingState().fontLineHeight, 1);
    }

    function checkLastPage() {
        let domR = pageContentRef.current;
        const scrollHeight = domR.scrollHeight;
        const clientHeight = domR.clientHeight;
        const scrollTop = domR.scrollTop;
        return Math.ceil(scrollTop + clientHeight) >= scrollHeight;
    }

    const advanceAutoPage = useMemoizedFn(() => {
        const content = pageContentRef.current;
        if (!content || content.clientHeight <= 0 || document.hidden) return;
        if (!checkLastPage()) {
            pageDown();
            return;
        }
        const current = getState();
        const chapterIndex = current.currentBookChapterList.indexOf(current.currentBookChapterName);
        if (chapterIndex < 0) return;
        if (chapterIndex === current.currentBookChapterList.length - 1) {
            setCacheItem('autoPage', '0');
            setSettingState({autoPage: '0'});
            message.info(t("已读完全书，自动翻页已关闭"));
            return;
        }
        nextChpater();
    });

    if (autoPageTimerRef.current === null) {
        autoPageTimerRef.current = createAutoPageTimer(advanceAutoPage);
    }

    useEffect(() => {
        if (settingState.autoPage !== '1' || !display || !state.currentBookChapterName
            || state.loadingBook || state.settingVisible || state.muluVisible || state.helpVisible
            || state.sysSettingVisible || state.downloadFromUrlVisible || state.gotoMuluIndexSearchVisible || state.miniMenuOpen) return;

        autoPageTimerRef.current.start(settingState.autoPageInterval * 1000);
        return () => autoPageTimerRef.current.stop();
    }, [settingState.autoPage, settingState.autoPageInterval, display, state.currentBookName,
        state.currentBookChapterName, state.loadingBook, state.settingVisible, state.muluVisible,
        state.helpVisible, state.sysSettingVisible, state.downloadFromUrlVisible,
        state.gotoMuluIndexSearchVisible, state.miniMenuOpen, advanceAutoPage]);

    const canAutoScroll = useMemoizedFn(() => {
        const current = getState();
        return getSettingState().autoScroll === '1' && display && !document.hidden
            && !!current.currentBookChapterName && !current.loadingBook
            && !current.settingVisible && !current.muluVisible && !current.helpVisible
            && !current.sysSettingVisible && !current.downloadFromUrlVisible
            && !current.gotoMuluIndexSearchVisible && !current.miniMenuOpen && !document.querySelector('.ant-modal-wrap');
    });
    const advanceAutoScrollChapter = useMemoizedFn(async () => {
        const current = getState();
        const index = current.currentBookChapterList.indexOf(current.currentBookChapterName);
        if (index < 0 || !canAutoScroll()) return;
        if (index === current.currentBookChapterList.length - 1) {
            setAutoReadingMode('autoScroll', false);
            progressRef.current.save();
            message.info(t("已读完全书，自动滚动已关闭"));
            return;
        }
        const success = await nextChpater();
        if (!success) setAutoReadingMode('autoScroll', false);
    });
    if (autoScrollRef.current === null) {
        autoScrollRef.current = createAutoScroll({
            getContent: () => pageContentRef.current,
            canScroll: canAutoScroll,
            onEnd: () => advanceAutoScrollChapter().catch(error => {
                setAutoReadingMode('autoScroll', false);
                message.error(String(error));
            }),
        });
    }
    useEffect(() => {
        if (settingState.autoScroll !== '1' || !display || !state.currentBookChapterName
            || state.loadingBook || state.settingVisible || state.muluVisible || state.helpVisible
            || state.sysSettingVisible || state.downloadFromUrlVisible || state.gotoMuluIndexSearchVisible || state.miniMenuOpen) return;
        autoScrollRef.current.start(settingState.autoScrollSpeed);
        return () => {
            autoScrollRef.current.stop();
            progressRef.current.save();
        };
    }, [settingState.autoScroll, settingState.autoScrollSpeed, display, state.currentBookName,
        state.currentBookChapterName, state.loadingBook, state.settingVisible, state.muluVisible,
        state.helpVisible, state.sysSettingVisible, state.downloadFromUrlVisible, state.gotoMuluIndexSearchVisible, state.miniMenuOpen]);

    function hasError(res, displayError = true) {
        if (res.startsWith("错误信息:")) {
            // message.error(res);
            if (displayError) {
                setState({
                    errorInfo: res
                })
            }
            api.info({
                message: t("提示"),
                description: localizeError(res),
                placement: "bottomRight"
            });
            return true;
        }
        return false;
    }

    function pageUp() {
        pauseAutoScroll();
        postponeAutoPage();
        turnReaderPage(pageContentRef.current, getSettingState().fontLineHeight, -1);
    }

    async function goChapterByName(fileName, chapterName, cb = null) {
        let splitType = getSplitType(fileName);
        let splitTypeValue = getSplitTypeValue(fileName);
        try {
            const res = await GetChapterListByFileName(fileName, splitType, splitTypeValue);
            if (hasError(res)) return;
            const chapters = res.split("\n").filter(name => name.trim());
            if (chapters.length === 0) {
                message.info(t("未找到有正文的章节，可在阅读偏好中调整断章方式"));
                return;
            }
            const selectedChapter = chapters.includes(chapterName) ? chapterName : chapters[0];
            const chapterContent = await GetChapterContentByChapterName(fileName, selectedChapter, splitType, splitTypeValue);
            if (hasError(chapterContent)) return;
            progressRef.current.stop();
            progressRef.current.resume();
            if (chapterName && selectedChapter !== chapterName) {
                // A filtered empty chapter must not restore an unrelated scroll position.
                setCacheItem(CACHE_SCROLL_TOP + fileName, '0');
                message.info(t("原章节已被过滤，已打开首个有效章节"));
            }
            setCacheItem(fileName, selectedChapter);
            setState({
                currentBookName: fileName,
                currentBookChapterList: chapters,
                currentBookChapterName: selectedChapter,
                currentBookChapterContent: chapterContent.split("\n").filter(Boolean)
            });
            if (typeof cb === 'function') cb();
            return true;
        } catch (error) {
            message.error(t("读取章节失败：{0}", {0: String(error)}));
        } finally {
            setState({loadingBook: false});
        }
    }

    function isAlwaysTop(_isAlwaysTop) {

        let isAlwaysTop = _isAlwaysTop ? _isAlwaysTop : getSettingState().isAlwaysTop;
        if (isAlwaysTop === '1') {
            Window.SetAlwaysOnTop(true)
        } else {
            Window.SetAlwaysOnTop(false)
        }
    }


    function toCurrentChapterNameInList() {
        let element = document.querySelector(".mulu-modal-center-ul-li-active");
        if (element) {
            element.scrollIntoView({
                behavior: "instant"
            })
            // element.scrollTop = 10;
        }
    }

    function lastChpater() {
        let currentBookChapterList = getState().currentBookChapterList;
        if (!isEmpty(currentBookChapterList)) {
            let currentBookChapterName = getState().currentBookChapterName;
            if (!isEmpty(currentBookChapterName)) {
                let index = findIndex(currentBookChapterList, e => e === currentBookChapterName);
                if (index === 0) {
                    // alert("已经是第一章了")
                } else {
                    let lastChpaterIndex = index - 1;
                    if (lastChpaterIndex >= 0) {
                        setState({
                            loadingBook: true
                        })
                        let currentBookChapterListElement = currentBookChapterList[lastChpaterIndex];
                        let currentBookName = getState().currentBookName;
                        goChapterByName(currentBookName, currentBookChapterListElement, () => {
                            let current = pageContentRef.current;
                            current.scrollTop = 0
                            setState({
                                loadingBook: false
                            })
                        });
                        toCurrentChapterNameInList()

                    }
                }

            }
        }
    }

    function getBookContainerHeight() {
        const state = getState();
        const settingState = getSettingState();
        let ht = 0
        ht += state.showTitle ? 30 : 0
        const showBottomBar = state.showTitle && settingState.transparentMode !== "1";
        ht += showBottomBar ? 30 : 0
        if (appRef.current) {
            return appRef.current.clientHeight - ht
        } else {
            //return getBookContainerHeight()
        }
    }

    /**
     * 动态计算行高 以至于让文字自适应页面
     * 废了点心思 基本实现文字自适应高度 不会再出现半截文字那种 当然 用鼠标滚轮肯定就另当别论了 如果重新调整行高之后需要从这一章节开始看
     * @param lineHeight2 指定行高
     * @param fontSize2 指定文字大小
     * @param fullScreen 是否全屏
     * @param fullRow 几行
     */
    function calculateFontLines(lineHeight2 = "", fontSize2 = "", fullScreen = 0, fullRow = 0) {
        const layout = calculateReadingLayout(
            fullScreen || getBookContainerHeight(),
            fontSize2 || getSettingState().fontSize,
            lineHeight2 || getSettingState().fontLineHeight,
            fullRow,
        );
        // Hidden or minimised windows may temporarily have no measurable height.
        if (!layout) return;
        setSettingState(layout);
        setCacheItem('fontLineHeight', layout.fontLineHeight);
        setCacheItem('fontSize', layout.fontSize);
    }

    function nextChpater() {
        let currentBookChapterList = getState().currentBookChapterList;
        if (!isEmpty(currentBookChapterList)) {
            let currentBookChapterName = getState().currentBookChapterName;
            if (!isEmpty(currentBookChapterName)) {
                let index = findIndex(currentBookChapterList, e => e === currentBookChapterName);
                if (index === currentBookChapterList.length - 1) {
                    message.warning("It's already the last chapter")
                } else {
                    let lastChpaterIndex = index + 1;
                    let currentBookChapterListElement = currentBookChapterList[lastChpaterIndex];
                    let currentBookName = getState().currentBookName;
                    setState({
                        loadingBook: true
                    })
                    const request = goChapterByName(currentBookName, currentBookChapterListElement, () => {
                        let current = pageContentRef.current;
                        current.scrollTop = 0
                        setState({
                            loadingBook: false
                        })
                    });
                    toCurrentChapterNameInList()
                    return request;

                }

            }
        }
    }


    function toggleTitle(visible) {
        let showTitle = getState().showTitle;
        let b = typeof visible === 'boolean' ? visible : !showTitle;
        setCacheItem("标题开关", b);
        setState({
            showTitle: b
        })
        setTimeout(() => {
            // 重新计算
            calculateFontLines()
        }, 100)

    }

    function backBookList(cb = null) {
        progressRef.current.stop();
        reloadBookList(() => {

            setState({
                currentBookChapterName: '',
                currentBookName: '',
                currentBookChapterContent: [],
                currentBookChapterList: [],
                muluVisible: false,
                gotoMuluIndexSearchVisible: false,
                lastSearchMulu: -1,
                lastSearchMuluName: "",
                settingVisible: false
            })
            if (typeof cb === 'function') cb()
        })
    }

    const handleSelect = (option) => {
        if (option.id === 'library') {
            backBookList()
        } else if (option.id === 'next-chapter') {
            nextChpater();
        } else if (option.id === 'previous-chapter') {
            lastChpater();
        } else if (option.id === 'title-bar') {
            toggleTitle()
        } else if (option.id === 'next-page') {
            pageDown();
        } else if (option.id === 'previous-page') {
            pageUp();
        } else if (option.id === 'preferences') {
            setState({
                settingVisible: !getState().settingVisible
            })
        } else if (option.id === 'contents') {
            setState({
                muluVisible: !getState().muluVisible
            })
            setTimeout(() => {
                toCurrentChapterNameInList()
            }, 200)
        } else if (option.id === 'minimize') {
            Window.Minimise();
        } else if (option.id === 'help') {
            openUsageGuide();
        } else if (option.id === 'quit') {
            Application.Quit();
        }
    };

    const openMiniMenu = useMemoizedFn(async (x, y) => {
        setState({miniMenuOpen: true});
        setDisplay(true);
        autoPageTimerRef.current.stop();
        pauseAutoScroll();
        try {
            await OpenMiniMenu(x, y, language, getState().currentBookName, getState().currentBookChapterName);
        } catch (error) {
            setState({miniMenuOpen: false});
            message.error(String(error));
        }
    });

    const handleMiniMenuAction = useMemoizedFn(async id => {
        try {
            setDisplay(true);
            await runMiniMenuAction(id, {
                isMiniMode: getSettingState().miniMode === '1',
                restoreWindow: () => miniSize(getSettingState().miniRows, false),
                showTitle: () => toggleTitle(true),
                dispatch: action => handleSelect({id: action}),
            });
        } catch (error) { message.error(String(error)); }
    });


    const options = [
        {id: 'title-bar', label: t("标题开关")},
        {id: 'preferences', label: t("设置"),},
        {id: 'next-page', label: t("下一页")},
        {id: 'previous-page', label: t("上一页")},
        {id: 'contents', label: t("目录")},
        {id: 'next-chapter', label: t("下一章")},
        {id: 'previous-chapter', label: t("上一章")},
        {id: 'library', label: t("返回书架")},
        {id: 'help', label: t("使用说明")},
        {id: 'minimize', label: t("最小化")},
        {id: 'quit', label: t("退出系统")},

    ];

    const genPresets = (presets = presetPalettes) =>
        Object.entries(presets).map(([label, colors]) => ({
            label,
            colors,
        }));

    const {token} = theme.useToken();
    const presets = genPresets({
        // primary: generate(token.colorPrimary),
        // red,
        // green,
    });

    function clickBookToFirst(tem) {
        let item1 = getCacheItem("LastClickBook") || tem;
        if (item1) {
            item1 = [tem, ...((item1.replace(tem, "")).split(","))].filter(Boolean).join(",")
        }
        setCacheItem("LastClickBook", item1);
    }

    function beginRecordTop(tem) {
        progressRef.current.restore(tem);
    }

    const muluSearch = useMemoizedFn((_value) => {
        let currentBookChapterList = getState().currentBookChapterList;
        let lastSearchMulu = getState().lastSearchMulu;
        let lastSearchMuluName = getState().lastSearchMuluName;
        let value = trim(_value);
        if (isEmpty(value)) {
            message.info("please input search content")
            return;
        }
        let searchByInitials1 = searchByInitials(value.toLowerCase(), currentBookChapterList);
        let pickList = [];
        if (!isEmpty(searchByInitials1)) {
            pickList = pickList.concat(searchByInitials1.map(e => e.name)).filter(Boolean)
        } else {
            let filter1 = filter(currentBookChapterList, e => e.indexOf(value) >= 0);
            pickList = pickList.concat(filter1).filter(Boolean);
        }
        let lastSearchMuluNameW = null;
        if (!isEmpty(pickList)) {
            lastSearchMuluNameW = pickList[0]
            let nextElementIndex = -1;
            let lastNotEnded = true;
            // change search content
            if (!pickList.includes(lastSearchMuluName)) {
                lastNotEnded = false
            }
            if (lastSearchMulu !== -1 && lastNotEnded) {
                let lastSearchName = pickList[lastSearchMulu];
                let pickListElement1 = pickList[lastSearchMulu + 1];
                // have next
                if (pickListElement1) {
                    // find next
                    let findIndex2 = findIndex(currentBookChapterList, e => e === pickListElement1);
                    if (findIndex2 >= 0) {
                        nextElementIndex = findIndex2;
                    }
                } else {
                    //no next
                    nextElementIndex = findIndex(currentBookChapterList, e => e === lastSearchName);
                }
            } else {
                nextElementIndex = findIndex(currentBookChapterList, e => e === lastSearchMuluNameW);
            }
            if (nextElementIndex > 0) {
                let element = document.querySelector(`[data-key="mulu-index-${nextElementIndex}"]`);
                if (!isEmpty(element)) {
                    element.scrollIntoView({
                        behavior: "instant"
                    })
                    setState({
                        lastSearchMulu: lastSearchMulu + 1,
                        lastSearchMuluName: currentBookChapterList.find((ite, inde) => inde === nextElementIndex),
                    })
                }
            }
        } else {
            setState({
                lastSearchMulu: -1,
                lastSearchMuluName: '',
            })
            message.info("not match " + value)
        }
    });

    const openUsageGuide = useMemoizedFn(async () => {
        if (getState().helpVisible) return;
        try {
            if (getSettingState().miniMode === '1' && getState().currentBookName) {
                await miniSize(getSettingState().miniRows, false);
            }
            setDisplay(true);
            setState({helpVisible: true, settingVisible: false, muluVisible: false, sysSettingVisible: false});
        } catch (error) {
            message.error(String(error));
        }
    });

    const exitMiniMode = useMemoizedFn(() => {
        setState({settingVisible: false});
        return miniSize(getSettingState().miniRows, false).catch(error => message.error(String(error)));
    });

    async function miniSize(rows, enabled = true) {
        const miniRows = Math.min(4, Math.max(1, Number(rows) || 3));
        if (enabled && !normalWindowRef.current) {
            let savedWindow;
            try {
                savedWindow = JSON.parse(getCacheItem('miniNormalWindow'));
            } catch {
                // Older installations have no saved normal window.
            }
            normalWindowRef.current = {
                ...await Window.Size(),
                showTitle: getState().showTitle,
                fontSize: Number(getSettingState().fontSize),
                fontLineHeight: Number(getSettingState().fontLineHeight)
            };
            if (getSettingState().miniMode === '1' && savedWindow?.width > 0 && savedWindow?.height > 0) {
                normalWindowRef.current = savedWindow;
            }
            setCacheItem('miniNormalWindow', JSON.stringify(normalWindowRef.current));
        }
        setCacheItem('miniMode', enabled ? '1' : '0');
        setCacheItem('miniRows', miniRows);
        setSettingState({miniMode: enabled ? '1' : '0', miniRows});

        const normalWindow = normalWindowRef.current || {
            width: 400, height: 690, showTitle: getCacheItem('标题开关') !== 'false'
        };
        // Older saved mini-mode snapshots only contain the window geometry.
        const normalFontSize = Number(normalWindow.fontSize) > 0 ? Number(normalWindow.fontSize) : DEFAULT_READING_TYPOGRAPHY.fontSize;
        const normalLineHeight = Number(normalWindow.fontLineHeight) > 0 ? Number(normalWindow.fontLineHeight) : DEFAULT_READING_TYPOGRAPHY.lineHeight;
        setState({oneLine: enabled, showTitle: enabled ? false : normalWindow.showTitle});
        if (enabled) {
            setSettingState({fontSize: 20});
        } else {
            // Restore before resizing so the resize handler cannot reuse the mini line height.
            setSettingState({fontSize: normalFontSize, fontLineHeight: normalLineHeight});
            setCacheItem('fontSize', normalFontSize);
            setCacheItem('fontLineHeight', normalLineHeight);
        }
        await Window.SetSize(enabled ? 600 : normalWindow.width, enabled ? 120 : normalWindow.height);
        await Window.Center();
        if (!enabled) {
            normalWindowRef.current = null;
            setCacheItem('miniNormalWindow', '');
        }
        await new Promise(resolve => window.requestAnimationFrame(() => {
            calculateFontLines(enabled ? '' : normalLineHeight, enabled ? 20 : normalFontSize,
                enabled ? 120 : 0, enabled ? miniRows : 0);
            resolve();
        }));
    }

    // Reapply the saved reading mode when opening a book after a reload.
    useEffect(() => {
        if (state.currentBookName && getSettingState().miniMode === '1') {
            miniSize(getSettingState().miniRows).catch(console.error);
        }
    }, [state.currentBookName]);

    const selectBook = useMemoizedFn(function (item) {
        bookListRef.current.selectBook(item)
    })
    const backBookListCallback = useMemoizedFn(backBookList);
    return (
        <div id="App"
             className={`${isEmpty(state.currentBookChapterName) ? 'is-library' : 'is-reading'} ${settingState.transparentMode === '1' ? 'is-transparent' : ''}`}
             ref={appRef}
             style={{
                 // border: display ? getState().currentBookChapterName ? "1px solid transparent" : "1px solid rgba(204, 204, 204, 0.32)" : "none",
                 backgroundColor: (display && isEmpty(getState().currentBookChapterName)) ? '#fff' : 'rgba(0,0,0,0)',
                 ...(function () {
                     if (!display) {
                         return {
                             color: 'rgba(0,0,0,0)',
                             border: 'none'
                         }
                     }
                     return {}
                 })
             }}
             onMouseOver={() => {
                 let leaveWindowHid = getSettingState().leaveWindowHid;
                 if (leaveWindowHid === '1') {
                     setDisplay(true)
                 }
             }}
             onMouseLeave={() => {
                 let leaveWindowHid = getSettingState().leaveWindowHid;
                 if (leaveWindowHid === '1' && !getState().helpVisible && !getState().miniMenuOpen) {
                     setDisplay(false)
                 }
             }}

        >

            <Header
                backBookList={backBookListCallback}
                toggleTitle={useMemoizedFn(toggleTitle)}
                exitMiniMode={exitMiniMode}
                onOpenHelp={openUsageGuide}
                display={display}
                state={getState()}
                setState={setState}
                isAlwaysTop={useMemoizedFn(isAlwaysTop)}
                settingstate={getSettingState()}
                setSettingState={setSettingState}
                title={getSettingState().appTitle || t('偷得浮生半日闲')}
                reloadBookList={useMemoizedFn(reloadBookList)}
            />

            {/*index book list*/}
            <BookList
                onOpenHelp={openUsageGuide}
                ref={bookListRef}
                state={state}
                setState={setState}
                calculateFontLines={useMemoizedFn(calculateFontLines)}
                reloadBookList={useMemoizedFn(reloadBookList)}
                goChapterByName={useMemoizedFn(goChapterByName)}
                beginRecordTop={useMemoizedFn(beginRecordTop)}
                display={display}
                hasError={useMemoizedFn(hasError)}
                clickBookToFirst={useMemoizedFn(clickBookToFirst)}
                clickBookPlus={() => {
                    OpenFileDialog(t("文件选择")).then(r => {
                        if (r) {
                            setState({loadingBook: true, loadingBookTip: "解析中。。,非txt和 epub解析时间随文件大小递增"});
                            Call.ByName('main.App.ImportBook', r).then(name => {
                                reloadBookList(() => bookListRef.current?.editImportedBook(name));
                                message.info(t('成功导入 {count} 份文件', {count: 1}));
                            }).catch(error => {
                                setState({loadingBook: false});
                                message.error(String(error));
                            });
                        }
                    }).catch(error => message.error(String(error)))
                }}
                onSelect={function (option) {
                    if (option.id === 'download') {
                        setState({
                            downloadFromUrlVisible: true,
                            downloadFromUrlList: [null]
                        })
                    }
                }}
                onSearch={(_value, metadataMatches = []) => {
                    let value = trim(_value)
                    if (!value) return [];
                    const names = getState().currentBookList;
                    const initials = searchByInitials(value.toLowerCase(), names).map(item => item.name);
                    const direct = names.filter(name => name.toLowerCase().includes(value.toLowerCase()));
                    return [...new Set([...metadataMatches, ...initials, ...direct])];
                }}
            />


            {/*content page*/}
            <Content
                getBookContainerHeight={useMemoizedFn(getBookContainerHeight)}
                state={getState()}
                settingState={getSettingState()}
                display={display}
                ref={pageContentRef}
                onScroll={recordTop}
                onWheel={pauseAutoScroll}
                onPointerDown={pauseAutoScroll}
                onContextMenu={e => {
                    e.preventDefault();
                }}
                onClick={(e) => {
                    // e.stopPropagation();
                    if (getSettingState().clickPage === '1') {
                        pageDown();
                    }
                }}
                onDoubleClick={() => {
                    if (checkLastPage()) {
                        nextChpater();
                    }
                }}
                lastChapter={e => {
                    e.stopPropagation();
                    lastChpater();
                }}
                nextChapter={e => {
                    e.stopPropagation();
                    nextChpater();
                }}
                options={options}
                onSelect={handleSelect}
                onOpenMenuWindow={openMiniMenu}
            />

            {/*content setting*/}
            <ContentSetting
                selectBook={selectBook}
                backBookList={backBookListCallback}
                miniSize={useMemoizedFn(miniSize)}
                toggleTitle={useMemoizedFn(toggleTitle)}
                state={getState()}
                calculateFontLines={useMemoizedFn(calculateFontLines)}
                setState={setState}
                settingState={getSettingState()}
                display={display}
                isAlwaysTop={isAlwaysTop}
                setDisplay={setDisplay}
                setSettingState={setSettingState}

                setAutoReadingMode={setAutoReadingMode}

            />

            {/*catlog modal*/}
            <MuluList
                display={display}
                currentBookChapterList={state.currentBookChapterList}
                muluVisible={state.muluVisible}
                currentBookChapterName={state.currentBookChapterName}
                gotoMuluIndexSearchVisible={state.gotoMuluIndexSearchVisible}
                onPressEnter={useMemoizedFn((e) => {
                    let value = e.target.value;
                    let element = document.querySelector(`[data-key="mulu-index-${value - 1}"]`);
                    if (!isEmpty(element)) {
                        element.scrollIntoView({
                            behavior: "instant"
                        })
                    } else {
                        message.error(t('未找到章节'))
                    }
                })}
                onClickSearch={useMemoizedFn(() => {
                    setState({
                        gotoMuluIndexSearchVisible: true
                    })
                })}
                closeDrawer={useMemoizedFn(() => {
                    setState({
                        muluVisible: false,
                        lastSearchMulu: -1,
                        lastSearchMuluName: "",
                        gotoMuluIndexSearchVisible: false
                    })
                })}
                onSearch={muluSearch}
                clickMulu={useMemoizedFn((ie, index) => {
                    setState({
                        loadingBook: true
                    })
                    goChapterByName(getState().currentBookName, ie, () => {
                        setState({
                            loadingBook: false,
                            muluVisible: false,
                            gotoMuluIndexSearchVisible: false,
                            lastSearchMulu: -1,
                            lastSearchMuluName: "",
                        })
                        let current = pageContentRef.current;
                        if (current) {
                            current.scrollTop = 0
                        }
                    })
                })}
            />


            <Spin spinning={getState().loadingBook} tip={t(getState().loadingBookTip)} fullscreen/>

            {
                getState().errorInfo && (
                    <div className={'fixed-txt-center color-red zIndex108'}>
                        <div className={"flex flex-column gap5 w-full align-item-center"}>
                            <p style={{textAlign: 'center'}}>{localizeError(getState().errorInfo)}</p>
                            <Button size={"small"} className={'w-50'} onClick={() => {
                                setState({
                                    errorInfo: ""
                                })
                            }}>{t("知道了")}</Button>
                        </div>
                    </div>
                )
            }


            <Drawer
                title={t("图书下载")}
                placement={"top"}
                // width={500}
                onClose={function () {
                    setState({
                        downloadFromUrlVisible: false,
                        downloadFromUrlList: [null]
                    })
                }}
                open={getState().downloadFromUrlVisible}
                extra={
                    <Space>
                        <Button onClick={(e) => {
                            message.info(t("已取消操作"))
                            setState({
                                downloadFromUrlVisible: false,
                                downloadFromUrlList: [null]
                            })
                        }}>{t("取消")}</Button>
                        <Button type="primary" onClick={(e) => {
                            let filter1 = getState().downloadFromUrlList.filter(Boolean);
                            if (isEmpty(filter1)) {
                                message.error(t("请输入要下载的地址在提交"))
                            } else {
                                AddFleAndHandlerRes(filter1);
                            }
                            setState({
                                downloadFromUrlVisible: false,
                                downloadFromUrlList: [null]
                            })
                        }}

                        >{t("提交")}</Button>
                    </Space>
                }
            >
                <Alert className={"mb-10"} message={t("最好下载txt、epub格式，解析最快最稳定")} type="info"/>

                <p className={"mb-5"}>
                    <Space>
                        <Button size={"small"} onClick={(e) => {
                            let downloadFromUrlList = getState().downloadFromUrlList;
                            downloadFromUrlList.push(null)
                            setState({
                                downloadFromUrlList
                            })
                        }}><PlusOutlined/></Button>

                        <Button size={"small"} onClick={(e) => {
                            let downloadFromUrlList = getState().downloadFromUrlList;
                            if (downloadFromUrlList.length > 1) {
                                downloadFromUrlList.pop()
                                setState({
                                    downloadFromUrlList
                                })
                            }

                        }}><LineOutlined/></Button>
                    </Space>

                </p>

                <div className="flex flex-column-nowrap gap5">
                    {
                        getState().downloadFromUrlList.map((ite, index) => {
                            return <Input key={"downloadFromUrlList-" + index} style={{width: '100%'}}
                                          onChange={(e) => {
                                              let value = e.target.value;
                                              let s = window.decodeURI(value);
                                              let s1 = window.encodeURI(s);
                                              let downloadFromUrlList = getState().downloadFromUrlList;
                                              downloadFromUrlList[index] = s1
                                              setState({
                                                  downloadFromUrlList
                                              })
                                          }} placeholder={t("请输入图书文件地址，点击提交即可下载")}/>
                        })
                    }
                </div>


                <Alert
                    className={"mt-10 mb-10"}
                    message={t("内置下载地址(点击跳转)：强烈推荐Z-Lib世界最大电子图书馆（可能需要翻墙）")} type="info"/>

                <p className={"flex flex-row flex-wrap gap5"}>

                    <Button title={t("zlib,世界最大的电子图书馆，什么都可以下！可能要魔法网")} onClick={() => {
                        Browser.OpenURL("https://www.bing.com/search?q=Z-Library")
                    }}>Z-Lib</Button>
                    {/*<Button title={"知轩藏书-精校版小说下载-校对全本TXT小说下载网"} onClick={() => {*/}
                    {/*    Browser.OpenURL("https://zxcsol.com/")*/}
                    {/*}}>知轩藏书</Button>*/}
                    <Button title={t("80电子书")} onClick={() => {
                        Browser.OpenURL("https://www.jiumodiary.com/")
                    }}>{t("鸠摩搜索")}</Button>
                    <Button title={t("顶点小说")} onClick={() => {
                        Browser.OpenURL("https://www.najjdd.com/")
                    }}>{t("顶点小说")}</Button>

                    <Button title={"epubor-converter"} onClick={() => {
                        Browser.OpenURL("https://s3.amazonaws.com/download.epubor.com.bk/epubor-converter.exe")
                    }}>{t("电子书格式转换器下载")}</Button>
                </p>

            </Drawer>


            {/*系统设置*/}
            {
                display && (
                    <Drawer
                        title={t("系统设置")}
                        placement={"bottom"}
                        height={'80%'}
                        onClose={function () {
                            setState({
                                sysSettingVisible: false
                            })
                        }}
                        open={getState().sysSettingVisible}
                    >
                        <div className={"flex flex-column gap10"}>

                            <LanguageSelect/>

                            <Button onClick={openUsageGuide}>{t("使用说明（F1）")}</Button>

                            <span>{t("自定义标题：")}<Input placeholder={t("自定义应用标题")}
                                                           value={getSettingState().appTitle}
                                                           onChange={(e) => {
                                                               let value = e.target.value;

                                                               setCacheItem("AppTitle", value)
                                                               setSettingState({
                                                                   appTitle: value
                                                               })

                                                           }}/></span>
                            <span title={state.booksPath}>{t("工作目录: ***")}</span>
                            <span title={window.location.href}>{t("地址: ***")}</span>
                            <span>{t("缓存:")}{cacheCount}</span>
                            {/*<span>linux系统要切换到xorg(x11)显示协议才生效</span>*/}
                            {/*<span>windows/linux系统老板键：ctrl+space</span>*/}
                            {/*<span>Mac系统老板键：command+space</span>*/}

                            <span>
                                <Button size={"small"} onClick={function () {
                                    modal.confirm({
                                        title: t("警告！"),
                                        type: 'warning',
                                        okText: t("我确定"),
                                        cancelText: t("取消"),
                                        content: t("清空缓存会丢失所有文本的阅读记录"),
                                        onOk: () => {
                                            progressRef.current.stop(false);
                                            clearCache();
                                            Window.ForceReload();
                                        }
                                    })
                                }}>{t("清空所有缓存")}</Button></span>
                            <span><Button size={"small"} onClick={function () {
                                progressRef.current.stop(false);
                                clearReadingHistory();
                                Window.ForceReload();
                            }}>{t("清除阅读记录")}</Button></span>

                        </div>
                    </Drawer>
                )
            }

            <UsageGuide open={state.helpVisible} onClose={() => setState({helpVisible: false})}/>
            {contextHolder}
        </div>
    )
}

export default App
