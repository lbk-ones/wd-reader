import {useCallback, useEffect, useRef, useState} from 'react';
import pinyin from 'pinyin'
import './App.css';
import {Button, Drawer, Input, message, Modal, notification, Space, Spin, theme} from 'antd';
import {presetPalettes} from '@ant-design/colors';
import {filter, find, findIndex, get, isEmpty, throttle, trim} from 'lodash-es'
import {
    AddFile,
    GetBookList,
    GetBooksPath,
    GetChapterListByFileName,
    GetServerUrl,
    GetVersion,
    OpenFileDialog
} from "../wailsjs/go/main/App";
import {GetChapterContentByChapterName} from "../wailsjs/go/main/App.js";
import useAllState from "./components/lib/hooks/UseAllState";
import {
    BrowserOpenURL,
    LogError,
    OnFileDrop,
    OnFileDropOff,
    Quit,
    WindowCenter,
    WindowMinimise,
    WindowReloadApp,
    WindowSetAlwaysOnTop,
    WindowSetSize
} from "../wailsjs/runtime/runtime.js";
import {LineOutlined, PlusOutlined} from "@ant-design/icons";
import Header from "./components/bus/Header";
import BookList from "./components/bus/BookList";
import {
    CACHE_PREFIX,
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


const CACHE_SCROLL_TOP = "scroll-top"


function App() {
    const [display, setDisplay] = useState(true)
    let pageContentRef = useRef(null);
    let bookListRef = useRef(null);
    let appRef = useRef(null);

    const [api, contextHolder] = notification.useNotification();

    const [settingState, setSettingState, getSettingState] = useAllState({
        fontColor: getCacheItem('fontColor') || '#000',
        fontLineHeight: getCacheItem('fontLineHeight') || '30',
        blankLineHeight:0,// 舍弃高度
        newContentHeight:0,// 总高度
        bgColor: getCacheItem('bgColor') || '#E8E3D7',
        fontSize: getCacheItem('fontSize') || '20',
        clickPage: getCacheItem('clickPage') || '1',
        showProgress: getCacheItem('showProgress') || '1',
        isAlwaysTop: getCacheItem('isAlwaysTop') || '1',
        transparentMode: getCacheItem('transparentMode') || '0',
        leaveWindowHid: getCacheItem('leaveWindowHid') || '0',
        appTitle: getCacheItem('AppTitle') || '偷得浮生半日闲',
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
        oneLine:false
    })


    function handlerAddFileRes(res, list) {
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
                        description: `${Value}`,
                        placement: "bottomRight"
                    });
                })
                setState({
                    loadingBook: true,
                    loadingBookTip: "加载中。。",
                })
                reloadBookList();
                message.info("共成功:" + (allLength - errorLength) + "份文件")
            } catch (e) {

            }
        }
    }

    function AddFleAndHandlerRes(paths) {
        setState({
            loadingBook: true,
            loadingBookTip: "解析中。。,非txt和 epub解析时间随文件大小递增",
        })
        AddFile(paths).then(res => {
            handlerAddFileRes(res, paths);
        })
    }

    function handlerError(e) {
        LogError(e.error.stack)
    }

    function unhandledrejection(e) {
        LogError("unhandledrejection:" + e.reason)
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
        OnFileDrop(function (x, y, paths) {
            AddFleAndHandlerRes(paths);

        }, true)

        document.addEventListener('keydown', function (event) {
            if (event.key === 'PageDown' || event.key === 'PageUp') {
                event.preventDefault();
            }
        });
        let calculateFontLinesAuto = throttle((e)=>{calculateFontLines()},300);

        window.addEventListener("resize",calculateFontLinesAuto)

        return () => {
            window.removeEventListener("error", handlerError)
            window.removeEventListener("unhandledrejection", unhandledrejection)
            OnFileDropOff()
            window.removeEventListener("resize",calculateFontLinesAuto)
        }
    }, [])

    function getContentTop() {
        return pageContentRef.current?.scrollTop
    }

    let timer = null;

    function recordTop() {
        if (pageContentRef.current) {
            let currentBookName = getState().currentBookName;
            if (currentBookName) {
                let scrollTop = getContentTop()
                setCacheItem(CACHE_SCROLL_TOP + currentBookName, scrollTop)
                window.clearTimeout(timer)
                timer = setTimeout(function () {
                    window.requestAnimationFrame(recordTop)
                }, 1000)
            }
        }
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


                cb && cb()
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
        // let clientHeight1 = getContentClientHeight()
        // const clientHeight = Math.floor(pageContentRef.current.clientHeight);
        const newContentHeight = getSettingState().newContentHeight;


        pageContentRef.current.scrollBy({
            top: newContentHeight,
            behavior: 'instant' // 平滑滚动，可根据需要设置
        });
    }

    function checkLastPage() {
        let domR = pageContentRef.current;
        const scrollHeight = domR.scrollHeight;
        const clientHeight = domR.clientHeight;
        const scrollTop = domR.scrollTop;
        return Math.ceil(scrollTop + clientHeight) >= scrollHeight;
    }

    function hasError(res, displayError = true) {
        if (res.startsWith("错误信息:")) {
            // message.error(res);
            if (displayError) {
                setState({
                    errorInfo: res
                })
            }
            api.info({
                message: `提示`,
                description: `${res}`,
                placement: "bottomRight"
            });
            return true;
        }
        return false;
    }

    function pageUp() {
        // let clientHeight1 = pageContentRef.current.clientHeight;
        // if (getState().showTitle && getSettingState().transparentMode !== '1') {
        //     clientHeight1 = clientHeight1 - 30;
        // }
        const newContentHeight = getSettingState().newContentHeight;
        pageContentRef.current.scrollBy({
            top: -newContentHeight,
            behavior: 'instant' // 平滑滚动，可根据需要设置
        });
    }

    function goChapterByName(fileName, chapterName, cb = null) {
        let splitType = getSplitType(fileName);
        let splitTypeValue = getSplitTypeValue(fileName);
        GetChapterListByFileName(fileName,splitType,splitTypeValue).then(res => {
            if (!hasError(res)) {
                let strings = res.split("\n");
                let firstChapter = isEmpty(chapterName) ? get(strings, "[0]", "") : chapterName;
                GetChapterContentByChapterName(fileName, firstChapter,splitType,splitTypeValue).then(chapterContent => {
                    if (!hasError(chapterContent)) {
                        setCacheItem(fileName, firstChapter)
                        setState({
                            currentBookName: fileName,
                            currentBookChapterList: strings,
                            currentBookChapterName: firstChapter,
                            currentBookChapterContent: chapterContent.split("\n").filter(Boolean)
                        })
                    }
                    cb && cb();
                })
            }


        })
    }

    function isAlwaysTop(_isAlwaysTop) {

        let isAlwaysTop = _isAlwaysTop ? _isAlwaysTop : getSettingState().isAlwaysTop;
        if (isAlwaysTop === '1') {
            WindowSetAlwaysOnTop(true)
        } else {
            WindowSetAlwaysOnTop(false)
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

    function getBookContainerHeight(){
        const state = getState();
        const settingState = getSettingState();
        let ht = 0
        ht+=state.showTitle?30:0
        const showBottomBar = state.showTitle && settingState.transparentMode !== "1";
        ht+=showBottomBar?30:0
        if(appRef.current){
            return appRef.current.clientHeight - ht
        }else{
            return getBookContainerHeight(height)
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
    function calculateFontLines(lineHeight2= "",fontSize2= "",fullScreen= 0,fullRow=0) {
        let clientHeight = getBookContainerHeight();
        let lineHeight = getSettingState().fontLineHeight;
        if(lineHeight2){
            lineHeight = lineHeight2
        }

        if(fullScreen){
            clientHeight = fullScreen
            lineHeight = fullScreen/fullRow
        }

        function getLines(lh) {
            let lines = Math.floor(clientHeight / lh);
            return lines === Infinity ? 1 : lines === 0 ? 1 : lines
        }

        // 计算行数 向下取整
        let lines = getLines(lineHeight);
        // 新行高 向下取整
        let newLineHeight = Math.floor(clientHeight/lines);
        let fontSize = getSettingState().fontSize;
        if(fontSize2){
            fontSize = fontSize2
        }

        let blankHeight = 0;
        function getContentHeight() {
            // 小于字体大小了之后最理想的行高
            const fontSizeScale = fontSize + 4;
            // 不能小于字体大小
            newLineHeight = newLineHeight < fontSize ? fontSizeScale : newLineHeight

            newLineHeight = newLineHeight === Infinity ? fontSizeScale : newLineHeight === 0 ? fontSizeScale : newLineHeight
            return newLineHeight * lines;
        }


        let newContentHeight = getContentHeight();

        // 高度溢出了 需要重新计算
        if(newContentHeight>clientHeight){
            while (newContentHeight>clientHeight){
                if(newLineHeight>1 && fontSize>1){
                    newLineHeight--;
                    fontSize--;
                    lines = getLines(newLineHeight);
                    newContentHeight = getContentHeight();
                }
            }
        }
        setSettingState({
            fontLineHeight:newLineHeight, // 新的行高
            fontSize:fontSize, // 新的行高
            blankLineHeight:blankHeight, // 之前的想法暂未用到
            newContentHeight:newContentHeight // 计算高度
        })
        setCacheItem('fontLineHeight', newLineHeight)
        setCacheItem('fontSize', fontSize)
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


    function toggleTitle(){
        let showTitle = getState().showTitle;
        let b = !showTitle;
        setCacheItem("标题开关", b);
        setState({
            showTitle: b
        })
        setTimeout(()=>{
            // 重新计算
            calculateFontLines()
        },100)

    }

    function backBookList(cb=null){
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
            cb && cb()
        })
    }

    const handleSelect = (option) => {
        if (option.label === '返回书架') {
            backBookList()
        } else if (option.label === '下一章') {
            nextChpater();
        } else if (option.label === '上一章') {
            lastChpater();
        } else if (option.label === '标题开关') {
            toggleTitle()
        } else if (option.label === '下一页') {
            pageDown();
        } else if (option.label === '上一页') {
            pageUp();
        } else if (option.label === '设置') {
            setState({
                settingVisible: !getState().settingVisible
            })
        } else if (option.label === '目录') {
            setState({
                muluVisible: !getState().muluVisible
            })
            setTimeout(() => {
                toCurrentChapterNameInList()
            }, 200)
        } else if (option.label === '最小化') {
            WindowMinimise();
        } else if (option.label === '退出系统') {
            Quit();
        }
    };


    const options = [
        {label: '标题开关'},
        {label: '设置',},
        {label: '下一页'},
        {label: '上一页'},
        {label: '目录'},
        {label: '下一章'},
        {label: '上一章'},
        {label: '返回书架'},
        {label: '最小化'},
        {label: '退出系统'},

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
        let cacheItem = getCacheItem(CACHE_SCROLL_TOP + tem) || "0";
        window.clearTimeout(timer);
        setTimeout(function () {
            let current = pageContentRef.current;
            if (current) {
                current.scrollTop = parseInt(cacheItem)
                window.requestAnimationFrame(recordTop)
            } else {
                beginRecordTop(tem);
            }
        }, 100)
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

    function miniSize(rows){

        setState({
            oneLine:true, // 目前没用到这个字段
            showTitle:false
        })
        setSettingState({
            fontSize:20
        })
        // setCacheItem("fontSize",25)
        WindowSetSize(600, 120)

        setTimeout(()=>{
            WindowCenter()
            calculateFontLines("","",120,rows)
        },100)


    }
    const selectBook = useMemoizedFn(function(item){
        bookListRef.current.selectBook(item)
    })
    const backBookListCallback = useMemoizedFn(backBookList);
    return (
        <div id="App"
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
                 if (leaveWindowHid === '1') {
                     setDisplay(false)
                 }
             }}

        >

            <Header
                backBookList={backBookListCallback}
                toggleTitle={useMemoizedFn(toggleTitle)}
                display={display}
                state={getState()}
                setState={setState}
                isAlwaysTop={useMemoizedFn(isAlwaysTop)}
                settingstate={getSettingState()}
                setSettingState={setSettingState}
                title={getSettingState().appTitle}
                reloadBookList={useMemoizedFn(reloadBookList)}
            />

            {/*index book list*/}
            <BookList
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
                    OpenFileDialog().then(r => {
                        if (r) {
                            AddFleAndHandlerRes([r]);
                        }
                    })
                }}
                onSelect={function (option) {
                    if (option.label === '去下载') {
                        setState({
                            downloadFromUrlVisible: true,
                            downloadFromUrlList: [null]
                        })
                    }
                }}
                onSearch={(_value, e) => {
                    let value = trim(_value)
                    reloadBookList(() => {
                        if (!value) {
                            return
                        }
                        let searchByInitials1 = searchByInitials(value.toLowerCase(), getState().currentBookList);
                        let searchSuccess = false;
                        if (!isEmpty(searchByInitials1)) {
                            searchSuccess = true;
                            searchByInitials1.forEach(et => {
                                clickBookToFirst(et.name)
                            })
                        } else {
                            let filter1 = filter(getState().currentBookList, e => e.indexOf(value) >= 0);
                            searchSuccess = !isEmpty(filter1);
                            filter1.forEach(e => {
                                clickBookToFirst(e)
                            })
                        }
                        if (searchSuccess) {
                            message.success("查询到列表，已经置顶")
                        } else {
                            message.success("未查询到列表")
                        }
                    })

                }}
            />


            {/*content page*/}
            <Content
                getBookContainerHeight={useMemoizedFn(getBookContainerHeight)}
                state={getState()}
                settingState={getSettingState()}
                display={display}
                ref={pageContentRef}
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
                lastChapter={e=>{
                    e.stopPropagation();
                    lastChpater();
                }}
                nextChapter={e=>{
                    e.stopPropagation();
                    nextChpater();
                }}
                options={options}
                onSelect={handleSelect}
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
                        message.error("no chapter")
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


            <Spin spinning={getState().loadingBook} tip={getState().loadingBookTip} fullscreen/>

            {
                getState().errorInfo && (
                    <div className={'fixed-txt-center color-red zIndex108'}>
                        <div className={"flex flex-column gap5 w-full align-item-center"}>
                            <p style={{textAlign: 'center'}}>{getState().errorInfo}</p>
                            <Button size={"small"} className={'w-50'} onClick={() => {
                                setState({
                                    errorInfo: ""
                                })
                            }}>知道了</Button>
                        </div>
                    </div>
                )
            }


            <Drawer
                title="从链接中下载"
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
                            message.info("已取消操作")
                            setState({
                                downloadFromUrlVisible: false,
                                downloadFromUrlList: [null]
                            })
                        }}>取消</Button>
                        <Button type="primary" onClick={(e) => {
                            let filter1 = getState().downloadFromUrlList.filter(Boolean);
                            if (isEmpty(filter1)) {
                                message.error("请输入要下载的地址在提交")
                            } else {
                                AddFleAndHandlerRes(filter1);
                            }
                            setState({
                                downloadFromUrlVisible: false,
                                downloadFromUrlList: [null]
                            })
                        }}

                        >
                            提交
                        </Button>
                    </Space>
                }
            >
                <p className={"mb-5"}>
                    <Space>
                        <Button onClick={(e) => {
                            let downloadFromUrlList = getState().downloadFromUrlList;
                            downloadFromUrlList.push(null)
                            setState({
                                downloadFromUrlList
                            })
                        }}><PlusOutlined/></Button>

                        <Button onClick={(e) => {
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
                                          }} placeHolder={"请输入地址"}/>
                        })
                    }
                </div>


                <p className={"mt-20 flex flex-row gap5"}>
                    <Button title={"zlib,世界最大的电子图书馆，什么都可以下！可能要魔法网"} onClick={() => {
                        BrowserOpenURL("https://zh.opendelta.org/")
                    }}>Z-Lib</Button>
                    <Button title={"知轩藏书-精校版小说下载-校对全本TXT小说下载网"} onClick={() => {
                        BrowserOpenURL("https://zxcsol.com/")
                    }}>知轩藏书</Button>
                    <Button title={"80电子书"} onClick={() => {
                        BrowserOpenURL("https://www.80637.com/")
                    }}>80电子书</Button>
                    <Button title={"顶点小说"} onClick={() => {
                        BrowserOpenURL("https://www.najjdd.com/")
                    }}>顶点小说</Button>

                    <Button title={"epubor-converter"} onClick={() => {
                        BrowserOpenURL("https://s3.amazonaws.com/download.epubor.com.bk/epubor-converter.exe")
                    }}>电子书格式转换器下载</Button>
                </p>

            </Drawer>


            {/*系统设置*/}
            {
                display && (
                    <Drawer
                        title="系统设置"
                        placement={"bottom"}
                        // width={500}
                        onClose={function () {
                            setState({
                                sysSettingVisible: false
                            })
                        }}
                        open={getState().sysSettingVisible}
                    >
                        <div className={"flex flex-column gap10"}>

                            <span>自定义标题：<Input placeHolder={"自定义应用标题"} value={getSettingState().appTitle}
                                                    onChange={(e) => {
                                                        let value = e.target.value;

                                                        setCacheItem("AppTitle", value)
                                                        setSettingState({
                                                            appTitle: value
                                                        })

                                                    }}/></span>
                            <span>工作目录: {state.booksPath}</span>
                            <span>地址: {window.location.href}</span>
                            <span>缓存: {window.localStorage.length}</span>

                            <span><Button size={"small"} onClick={function () {
                                Modal.confirm({
                                    title: "警告！",
                                    type: 'warning',
                                    okText: "我确定",
                                    cancelText: "取消",
                                    content: "清空缓存会丢失所有文本的阅读记录",
                                    onOk: () => {
                                        for (let i = 0; i < window.localStorage.length; i++) {
                                            let s = window.localStorage.key(i);
                                            if (s && s.startsWith(CACHE_PREFIX)) {
                                                window.localStorage.removeItem(s);
                                            }
                                        }
                                        WindowReloadApp();
                                    }
                                })
                            }}>清空所有缓存</Button></span>
                            <span><Button size={"small"} onClick={function () {
                                setCacheItem("LastClickBook", "")
                                WindowReloadApp();
                            }}>清除阅读记录</Button></span>
                        </div>
                    </Drawer>
                )
            }

            {contextHolder}
        </div>
    )
}

export default App
