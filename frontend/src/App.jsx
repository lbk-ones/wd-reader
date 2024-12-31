import {useEffect, useRef, useState} from 'react';
import pinyin from 'pinyin'
import CsSVg from "./assets/images/cs.svg"
import './App.css';
import {
    Button,
    Checkbox,
    ColorPicker, Drawer,
    Form,
    Input,
    InputNumber,
    message, notification,
    Popover, Space,
    Spin, Statistic,
    Switch,
    theme,
    Tooltip
} from 'antd';
import {generate, green, presetPalettes, red} from '@ant-design/colors';
import {filter, findIndex, get, isEmpty, isEqual, trim} from 'lodash-es'
import classNames from "classnames"
import {
    GetAppPath,
    GetBookList,
    GetChapterListByFileName,
    GetServerUrl, GetVersion,
    Greet,
    OpenFileDialog, ParseEpubToTxt, GetBooksPath, AddFile
} from "../wailsjs/go/main/App";
import {DeleteEpubFile, GetChapterContentByChapterName} from "../wailsjs/go/main/App.js";
import useAllState from "./components/lib/hooks/UseAllState.jsx";
import ContextMenu from "./components/ContextMenu.jsx";
import {SketchPicker} from "react-color";
import {
    BrowserOpenURL,
    Hide, OnFileDrop, OnFileDropOff,
    Quit,
    WindowMinimise,
    WindowReload,
    WindowReloadApp,
    WindowSetAlwaysOnTop
} from "../wailsjs/runtime/runtime.js";
import {
    CloseOutlined, DatabaseOutlined, DisconnectOutlined,
    ExclamationCircleOutlined, LineOutlined,
    MinusOutlined, PlusOutlined,
    PushpinOutlined,
    RedoOutlined, SettingOutlined
} from "@ant-design/icons";

function App() {
    const [display, setDisplay] = useState(true)
    let pageContentRef = useRef(null);
    let topRef = useRef(null);

    const [api, contextHolder] = notification.useNotification();

    const [settingState, setSettingState, getSettingState] = useAllState({
        fontColor: window.localStorage.getItem('fontColor') || '#000',
        fontLineHeight: window.localStorage.getItem('fontLineHeight') || '30',
        bgColor: window.localStorage.getItem('bgColor') || '#fff',
        fontSize: window.localStorage.getItem('fontSize') || '16',
        clickPage: window.localStorage.getItem('clickPage') || '1',
        showProgress: window.localStorage.getItem('showProgress') || '0',
        isAlwaysTop: window.localStorage.getItem('isAlwaysTop') || '1',
        transparentMode: window.localStorage.getItem('transparentMode') || '0',
        leaveWindowHid: window.localStorage.getItem('leaveWindowHid') || '0',
    })

    const [state, setState, getState] = useAllState({
        currentBookList:[],
        currentBookName: "",
        currentBookChapterList: [],
        currentBookChapterName: "",
        currentBookChapterContent: [],
        loadIngBookList: false,
        showTitle: window.localStorage.getItem("标题开关") !== 'false',
        settingVisible: false,
        fontColor: '#000',
        muluVisible: false,
        serverUrl: '',
        loadingBook: false,
        loadingBookTip: "加载中。。",
        errorInfo: '',
        version: '',
        booksPath: '',
        downloadFromUrlVisible:false,
        downloadFromUrlList:[null],
    })


    function handlerAddFileRes(res,list){
        if(!hasError(res)){
            console.log(res)
            try{
                let parse = JSON.parse(res);
                let entries = Object.entries(parse);
                console.log('entryis',entries)
                let errorLength = entries.length;
                let allLength = list.length;
                entries.forEach((value, index, array) =>{
                    let key = value[0];
                    let Value = value[1];
                    api.info({
                        message: key,
                        description: `${Value}`,
                        placement:"bottomRight"
                    });
                })
                setState({
                    loadIngBook:true,
                    loadingBookTip:"加载中。。",
                })
                reloadBookList();
                message.info("共成功:"+(allLength-errorLength)+"份文件")
            }catch (e){

            }
        }
    }

    function AddFleAndHandlerRes(paths) {
        setState({
            loadIngBook:true,
            loadingBookTip:"解析中。。",
        })
        AddFile(paths).then(res => {
            handlerAddFileRes(res, paths);
        })
    }

    useEffect(function () {
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
        OnFileDrop(function (x,y,paths){
            // console.log('paths--->',paths)
            // message.info(JSON.stringify(paths))
            AddFleAndHandlerRes(paths);

        },true)

        return ()=>{
            OnFileDropOff()
        }
    }, [])

    function reloadBookList(cb = null) {
        setState({
            loadIngBook: true
        })
        GetBookList().then(res => {
            if (!hasError(res)) {
                setState({
                    loadIngBook: false,
                    currentBookList: res.split("\n").map(e => e.trim()).filter(Boolean)
                })

                cb && cb()
            }else{
                setState({
                    loadIngBook:false
                })
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
        return convertedData.filter(item => item.initials.startsWith(query));
    }

    function pageDown() {
        pageContentRef.current.scrollBy({
            top: pageContentRef.current.clientHeight,
            behavior: 'smooth' // 平滑滚动，可根据需要设置
        });
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
                placement:"bottomRight"
            });
            return true;
        }
        return false;
    }

    function pageUp() {
        pageContentRef.current.scrollBy({
            top: -pageContentRef.current.clientHeight,
            behavior: 'smooth' // 平滑滚动，可根据需要设置
        });
    }

    function goChapterByName(fileName, chapterName, cb = null) {
        GetChapterListByFileName(fileName).then(res => {
            if (!hasError(res)) {
                let strings = res.split("\n");
                let firstChapter = isEmpty(chapterName) ? get(strings, "[0]", "") : chapterName;
                GetChapterContentByChapterName(fileName, firstChapter).then(chapterContent => {
                    if (!hasError(chapterContent)) {
                        window.localStorage.setItem(fileName, firstChapter)
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
                        let currentBookChapterListElement = currentBookChapterList[lastChpaterIndex];
                        let currentBookName = getState().currentBookName;
                        goChapterByName(currentBookName, currentBookChapterListElement);
                        toCurrentChapterNameInList()
                        // if (topRef.current) {
                        //     console.log('topRef.current---->',topRef.current)
                        //     topRef.current.scrollIntoView({
                        //         behavior:"instant"
                        //     })
                        // }
                        let current = pageContentRef.current;
                        current.scrollTop = 0
                    }
                }

            }
        }
    }

    function nextChpater() {
        let currentBookChapterList = getState().currentBookChapterList;
        if (!isEmpty(currentBookChapterList)) {
            let currentBookChapterName = getState().currentBookChapterName;
            if (!isEmpty(currentBookChapterName)) {
                let index = findIndex(currentBookChapterList, e => e === currentBookChapterName);
                if (index === currentBookChapterList.length - 1) {
                    // alert("已经是最后一章了")
                } else {
                    let lastChpaterIndex = index + 1;
                    let currentBookChapterListElement = currentBookChapterList[lastChpaterIndex];
                    let currentBookName = getState().currentBookName;
                    goChapterByName(currentBookName, currentBookChapterListElement);
                    toCurrentChapterNameInList()
                    // if (topRef.current) {
                    //     topRef.current.scrollIntoView({
                    //         behavior:"instant"
                    //     })
                    // }
                    let current = pageContentRef.current;
                    current.scrollTop = 0
                }

            }
        }
    }

    const handleSelect = (option) => {
        if (option.label === '返回书架') {
            reloadBookList(() => {

                setState({
                    currentBookChapterName: '',
                    currentBookName: '',
                    currentBookChapterContent: [],
                    currentBookChapterList: [],
                    muluVisible: false,
                    settingVisible: false
                })

            })

        } else if (option.label === '下一章') {
            nextChpater();
        } else if (option.label === '上一章') {
            lastChpater();
        } else if (option.label === '标题开关') {
            let showTitle = getState().showTitle;
            let b = !showTitle;
            window.localStorage.setItem("标题开关", b);
            setState({
                showTitle: b
            })
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
        {label: '下一页'},
        {label: '上一页'},
        {label: '目录'},
        {label: '下一章'},
        {label: '上一章'},
        {label: '设置',},
        {label: '返回书架'},
        {label: '标题开关'},
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

    function sortBookList() {
        let currentBookList = getState().currentBookList;

        if (!isEmpty(currentBookList)) {
            let item = window.localStorage.getItem("LastClickBook") || "";
            let first = [];
            let last = [];
            (item.split(",")).forEach(e => {
                if (currentBookList.indexOf(e) >= 0) {
                    first.push(e);
                }
            })
            currentBookList.forEach(e => {
                if (first.indexOf(e) < 0) {
                    last.push(e);
                }
            })
            return first.concat(last);
        }

        return currentBookList;
    }

    function getMenuSuffixBtn(type = "0") {

        return (
            <>
                {
                    type === '1' && (
                        <SettingOutlined title={"设置！"}
                                         onClick={() => {
                                             setState({
                                                 settingVisible: true
                                             })
                                         }}/>
                    )
                }

                {
                    type === '1' && (
                        <DatabaseOutlined title={"返回书架！"}
                                          onClick={() => {
                                              reloadBookList(() => {
                                                  setState({
                                                      currentBookChapterName: '',
                                                      currentBookName: '',
                                                      currentBookChapterContent: [],
                                                      currentBookChapterList: [],
                                                      muluVisible: false,
                                                      settingVisible: false
                                                  })

                                              })
                                          }}/>
                    )
                }
                <DisconnectOutlined
                    title={"隐藏自己！"}
                    style={{color: getSettingState().leaveWindowHid === '1' ? 'gray' : 'unset'}}
                    onClick={() => {
                        let qf = getSettingState().leaveWindowHid === '1' ? '0' : '1';
                        window.localStorage.setItem("leaveWindowHid", qf)
                        setSettingState({
                            leaveWindowHid: qf
                        })
                    }}
                />
                <PushpinOutlined
                    title={"窗口置顶"}
                    style={{color: getSettingState().isAlwaysTop === '1' ? 'gray' : 'unset'}}
                    onClick={() => {
                        let qf = getSettingState().isAlwaysTop === '1' ? '0' : '1';
                        isAlwaysTop(qf)
                        window.localStorage.setItem("isAlwaysTop", qf)
                        setSettingState({
                            isAlwaysTop: qf
                        })
                    }}/>
                {
                    type !== '1' && (
                        <RedoOutlined title={"刷新"} onClick={() => {
                            WindowReloadApp()
                        }}/>
                    )
                }
                <MinusOutlined onClick={() => {
                    WindowMinimise();
                }}/>
                <CloseOutlined onClick={() => {
                    Quit();
                }}/>
            </>

        )
    }

    function clickBookToFirst(tem) {
        let item1 = window.localStorage.getItem("LastClickBook") || tem;
        if (item1) {
            item1 = [tem, ...((item1.replace(tem, "")).split(","))].filter(Boolean).join(",")
        }
        window.localStorage.setItem("LastClickBook", item1);
    }

    return (
        <div id="App"
             style={{
                 border: display ? getState().currentBookChapterName ? "1px solid transparent" : "1px solid rgba(204, 204, 204, 0.32)" : "none",
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
            // className={classNames({
            //     "track-transparent": !!getState().currentBookChapterName
            // })}
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
            {/*<Spin spinning={getState().loadingBook} tip={"加载中。。"}>*/}
            {
                display && (getState().showTitle || isEmpty(getState().currentBookName)) && (
                    <div className={"title"}>

                        {
                            !isEmpty(getState().currentBookName) && (
                                <div className={"title-div font-bold"}
                                     style={{"--wails-draggable": 'drag', display: 'flex', flex: 1}}
                                     title={getState().currentBookName.replace(".txt", "")}>
                                    &nbsp;<img src={CsSVg} alt=""/>&nbsp;
                                    {
                                        `${getState().currentBookName.replace(".txt", "")}`
                                    }
                                </div>
                            )
                        }

                        {
                            !isEmpty(getState().currentBookName) && (
                                <div className={"title-div font-bold mr-5"}
                                     style={{justifyContent: 'flex-end', gap: 3, fontSize: 20}}
                                     title={getState().currentBookChapterName}>

                                    {
                                        getMenuSuffixBtn("1")
                                    }
                                </div>
                            )
                        }

                        {
                            isEmpty(getState().currentBookName) && (
                                <div className={"title-div"} style={{
                                    fontWeight: 'bold',
                                    flex: 1
                                }}>
                                    <div style={{
                                        width: '100%',
                                        color: 'darkred',
                                        display: 'flex',
                                        flexShrink: 0,
                                        justifyContent: 'space-between',
                                        alignItems: 'center'
                                    }}>
                                        <div style={{
                                            "--wails-draggable": 'drag',
                                            display: 'flex',
                                            flex: 'auto',
                                            alignItems: 'center',
                                            gap: '2px'
                                        }}>
                                            <img src={CsSVg} alt=""/>
                                            <span>上班偷看小说神器</span>
                                            <span>{getState().version || ''}</span>
                                        </div>

                                        <div style={{display: "flex", flexFlow: 'row nowrap', gap: 3, fontSize: 20}}
                                             onClick={(e) => {
                                                 e.stopPropagation()
                                             }}>
                                            {
                                                getMenuSuffixBtn()
                                            }
                                        </div>

                                    </div>
                                </div>
                            )
                        }

                    </div>
                )
            }
            {
                isEmpty(state.currentBookChapterName) && display && isEmpty(getState().errorInfo) && (function () {
                    // if (!getState().loadIngBookList && isEmpty(getState().currentBookList)) {
                    //
                    //     return (
                    //         <div className={"flex-center"} style={{
                    //             fontSize: 18,
                    //             padding: 20,
                    //             color: 'darkred'
                    //         }}><span
                    //             className={"shrink-1"}>未读取到文件，文件目录为:{getState().booksPath}</span>
                    //         </div>
                    //     );
                    // }
                    return (
                        <div className="book-list">
                            <ul className={"book-list-ul flex flex-column-nowrap"}>
                                <div className="book-list-top-box">
                                    <div className={"add-book"} title={"添加文件，右键更多功能"} style={{
                                        "--wails-drop-target":"drop"
                                    }} onClick={()=>{
                                        OpenFileDialog().then(r => {
                                            AddFleAndHandlerRes([r]);
                                        })
                                    }}>
                                        <PlusOutlined/>

                                        <ContextMenu
                                            fontSize={16}
                                            options={
                                                [{label: 'URL下载'},
                                                {label: '去下载'}]}
                                            onSelect={function (option){
                                                if(option.label === 'URL下载') {
                                                    setState({
                                                        downloadFromUrlVisible:true
                                                    })
                                                }else if(option.label === '去下载'){
                                                    BrowserOpenURL("https://zh.opendelta.org/")
                                                }
                                            }}
                                        />
                                    </div>
                                    <div className={"search-box"}>
                                        <Input.Search allowClear={true} placeholder={"搜索"} style={{
                                            width: '100%',
                                            // borderBottom: "1px solid #ccc",
                                            borderRadius: 'none'
                                        }}
                                               onSearch={(_value,e)=>{
                                                   let value = trim(_value)
                                                   reloadBookList(()=>{
                                                       if(!value){
                                                          return
                                                       }
                                                       let searchByInitials1 = searchByInitials(value.toLowerCase(),getState().currentBookList);
                                                       let searchSuccess = false;
                                                       if(!isEmpty(searchByInitials1)) {
                                                           searchSuccess  = true;
                                                           searchByInitials1.forEach(et=>{
                                                               clickBookToFirst(et.name)
                                                           })
                                                       }else {
                                                           let filter1 = filter(getState().currentBookList, e=>e.indexOf(value)>=0);
                                                           searchSuccess = !isEmpty(filter1);
                                                           filter1.forEach(e=>{
                                                               clickBookToFirst(e)
                                                           })
                                                       }
                                                       if(searchSuccess){
                                                           message.success("查询到列表，已经置顶")
                                                       }else{
                                                           message.success("未查询到列表")
                                                       }
                                                   })

                                               }}
                                        />

                                        <div className={"flex flex-row gap10 align-item-center"}>
                                            <Statistic title="正在阅读" value={ (window.localStorage.getItem("LastClickBook") || "").split(",").length + "本"}/>
                                            <Statistic title="拥有图书" value={getState().currentBookList.length + "本"}/>
                                        </div>

                                    </div>
                                </div>
                                <div className={"flex-auto"}>
                                    <div className={"w-100 h-100 over-y-auto"}>
                                        {
                                            sortBookList()
                                                .filter(Boolean)
                                                .map((tem, index) => {
                                                    return <li className={classNames("book-list-ul-li", {
                                                        "book-list-ul-li-active": (window.localStorage.getItem("LastClickBook") || "").split(",").indexOf(tem) === 0
                                                    })}
                                                               key={"book-list-ul-li" + index}
                                                               title={tem} onClick={() => {

                                                        clickBookToFirst(tem);
                                                        setState({
                                                            loadingBook: true
                                                        })
                                                        if (tem.endsWith("epub")) {

                                                            ParseEpubToTxt(tem).then(res => {
                                                                if (res.startsWith("错误信息:")) {
                                                                    message.error(res)
                                                                }

                                                                let s = tem.replace(".epub", ".txt");
                                                                let item = window.localStorage.getItem(tem);
                                                                reloadBookList(() => {
                                                                    goChapterByName(s, item, () => {
                                                                        setState({
                                                                            loadingBook: false
                                                                        })
                                                                        DeleteEpubFile(tem).then(res => {
                                                                            // hasError(tem)
                                                                            console.log(res)
                                                                        })
                                                                    });
                                                                })

                                                            })
                                                        } else {
                                                            let item = window.localStorage.getItem(tem);
                                                            goChapterByName(tem, item, () => {
                                                                setState({
                                                                    loadingBook: false
                                                                })
                                                            });
                                                        }
                                                    }}>{tem}</li>
                                                })
                                        }
                                    </div>

                                </div>

                                <li className={'book-list-ul-li flex align-item-center no-shrink'} style={{
                                    width:'100%',
                                    justifyContent:'space-between',
                                    position: 'sticky',
                                    bottom: 0,
                                    backgroundColor: '#fff',
                                    color: "darkred",
                                    borderTop: '1px solid rgba(204, 204, 204, 0.32)',
                                    // borderBottom: '1px solid #ccc',
                                }}>
                                    <span>
                                    {
                                        ["没有人比我更懂你的需求😀", "加油吧打工人💪", "打工人，轻松一下😊", "Life Is A Fucking Move!😞", "什么时候才能不为了一日三餐奔波啊✿"][parseInt(Math.random() * 4)]
                                    }
                                    </span>



                                    <SettingOutlined style={{
                                        fontSize:'20px',
                                        paddingRight:'5px'
                                    }}  title={"设置！"}
                                                     onClick={() => {
                                                         setState({
                                                             settingVisible: true
                                                         })
                                                     }}/>

                                </li>
                            </ul>

                        </div>

                    )
                })()
            }


            {/*内容页面*/}
            {
                !isEmpty(state.currentBookChapterName) && !getState().currentBookName.endsWith(".epub") && (
                    <div className={"book-content-div"} style={{
                        lineHeight: Number(getSettingState().fontLineHeight) + "px",
                        padding: "0 10px",
                        // width: 'inherit',
                        // height: 'inherit',
                        boxSizing: 'border-box',
                        textAlign: 'left',
                        textIndent: '2em',
                        display: 'flex',
                        flexFlow: 'column nowrap',
                        flex: 'auto',
                        overflow: 'auto',
                        color: getSettingState().fontColor,
                        fontSize: Number(getSettingState().fontSize) + "px",
                        visibility: !display ? 'hidden' : 'visible',
                        backgroundColor: `${getSettingState().transparentMode !== '1' ? getSettingState().bgColor : 'rgba(0,0,0,0)'}`

                    }} ref={pageContentRef}
                         onContextMenu={e => {
                             e.preventDefault();
                         }}
                         onClick={(e) => {
                             // e.stopPropagation();
                             if (getSettingState().clickPage === '1') {
                                 pageDown();
                             }
                         }}
                    >
                        <div className={"flex flex-auto flex-column"}>
                            <div className={"w-100 h-100"}>
                                <div id={"book-content-div-top"} style={{width: '100%', height: '1px'}} ref={topRef}></div>
                                {
                                    !isEmpty(getState().currentBookChapterName)
                                    && (
                                        <p className={"mb-10 font-bold"}>
                                            {getState().currentBookChapterName || ''}
                                        </p>
                                    )
                                }
                                {
                                    getState().currentBookChapterContent.map((res, index) => {

                                        if (res && res.trim()) {
                                            if (index === 0 && res !== getState().currentBookChapterName) {
                                                return <p key={"mulu-li"+index} className={"mb-5"}>
                                                    {res.trim()}
                                                </p>
                                            } else if (index > 0) {
                                                return <p key={"mulu-li"+index} className={"mb-5"}>
                                                    {res.trim()}
                                                </p>
                                            }
                                        }
                                        return null;
                                    })
                                }
                            </div>
                        </div>

                        {
                            getState().showTitle && getSettingState().transparentMode !== '1' && (
                                <div className={"book-content-div-footer-btn"} style={{
                                    color: getSettingState().fontColor,
                                    backgroundColor: `${getSettingState().transparentMode !== '1' ? getSettingState().bgColor : 'rgba(0,0,0,0)'}`
                                }}>
                                    <a className={"text-indent0 h-100 cursor-point text-nowrap"} onClick={(e) => {
                                        e.stopPropagation();
                                        lastChpater();
                                    }}>上一章</a>
                                    <a className={"text-indent0 h-100 cursor-point text-nowrap"} onClick={(e) => {
                                        e.stopPropagation();
                                        nextChpater();
                                    }}>下一章</a>
                                    <span
                                        className={'book-content-div-footer-btn-txt cursor-point'}
                                        title={getState().currentBookChapterList[findIndex(getState().currentBookChapterList, e => e === getState().currentBookChapterName) + 1] || "无"}
                                        style={{width: "120px", overflow: 'hidden'}}>
                                        {getState().currentBookChapterList[findIndex(getState().currentBookChapterList, e => e === getState().currentBookChapterName) + 1] || "无"}
                                    </span>
                                </div>
                            )
                        }


                        <ContextMenu options={options} onSelect={handleSelect}/>
                    </div>
                )
            }

            {/*设置 -- 弹窗*/}
            {
                display && getState().settingVisible && (
                    <div className="setting-modal">
                            <span className={'setting-modal-close'} onClick={() => {
                                setState({
                                    settingVisible: false
                                })
                            }}>
                                ×
                            </span>
                        <div className={"setting-modal-center"} onClick={(e) => {
                            e.stopPropagation();
                        }}>


                            <Form
                                layout={"horizontal"}
                                name="basic"
                                labelCol={{
                                    span: 9,
                                    xs: 9,
                                    sm: 9
                                }}
                                wrapperCol={{
                                    span: 15,
                                    xs: 15,
                                    sm: 15,
                                }}
                                initialValues={{
                                    layout: "horizontal",
                                }}
                                // onFinish={onFinish}
                                // onFinishFailed={onFinishFailed}
                                autoComplete="off"
                            >
                                <Form.Item
                                    label="字体大小"
                                    name="字体大小"
                                    layout="horizontal"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <InputNumber style={{width: 100}} defaultValue={Number(getSettingState().fontSize)}
                                                 onChange={(value) => {
                                                     window.localStorage.setItem('fontLineHeight', value)
                                                     setSettingState({
                                                         fontSize: value
                                                     })
                                                 }}/>
                                </Form.Item>

                                <Form.Item
                                    label="间距"
                                    name="间距"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <InputNumber style={{width: 100}}
                                                 defaultValue={Number(getSettingState().fontLineHeight)}
                                                 onChange={(value) => {
                                                     window.localStorage.setItem('fontLineHeight', value)
                                                     setSettingState({
                                                         fontLineHeight: value
                                                     })
                                                 }}/>
                                    {/*<Input style={{width:100}} value={getSettingState().fontLineHeight} onChange={(e)=>{*/}
                                    {/*    let value = e.target.value;*/}
                                    {/*    window.localStorage.setItem('bgColor',hex)*/}
                                    {/*    setSettingState({*/}
                                    {/*        bgColor: color.hex*/}
                                    {/*    })*/}
                                    {/*}}/>*/}
                                </Form.Item>
                                <Form.Item
                                    label="背景色"
                                    name="背景色"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <Popover zIndex={112} content={() => {
                                        return (
                                            <SketchPicker
                                                color={getSettingState().bgColor}
                                                onChange={(color) => {
                                                    window.localStorage.setItem('bgColor', color.hex)
                                                    setSettingState({
                                                        bgColor: color.hex
                                                    })
                                                }}
                                                styles={{
                                                    // picker: {
                                                    //     width: '', // 调整宽度
                                                    //     height: '50%' // 调整高度
                                                    // }
                                                    // saturation:'50%'
                                                }}
                                            />
                                        )
                                    }} trigger="click" title="颜色选择">

                                        <div
                                            style={{
                                                width: '20px',
                                                height: '20px',
                                                // margin: '20px',
                                                backgroundColor: getSettingState().bgColor
                                            }}
                                        />

                                    </Popover>
                                </Form.Item>
                                <Form.Item
                                    label="字体颜色"
                                    name="字体颜色"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <Popover zIndex={112} content={() => {
                                        return (
                                            <SketchPicker
                                                color={getSettingState().fontColor}
                                                onChange={(color) => {
                                                    let hex = color.hex;
                                                    console.log('hex--->', hex)
                                                    window.localStorage.setItem('fontColor', hex)
                                                    setSettingState({
                                                        fontColor: hex
                                                    })
                                                }}
                                                styles={{
                                                    // picker: {
                                                    //     width: '', // 调整宽度
                                                    //     height: '50%' // 调整高度
                                                    // }
                                                    // saturation:'50%'
                                                }}
                                            />
                                        )
                                    }} trigger="click" title="颜色选择">

                                        <div
                                            style={{
                                                width: '20px',
                                                height: '20px',
                                                // margin: '20px',
                                                backgroundColor: getSettingState().fontColor
                                            }}
                                        />

                                    </Popover>

                                    {/*<ColorPicker size={'small'} onChange={(color,css)=>{*/}
                                    {/*    console.log(color.toHex(),css)*/}
                                    {/*    let s = "#"+color.toHex();*/}
                                    {/*    setSettingState({*/}
                                    {/*        fontColor: s*/}
                                    {/*    })*/}
                                    {/*    window.localStorage.setItem('fontColor',)*/}
                                    {/*}} presets={presets} value={getSettingState().fontColor} />*/}

                                </Form.Item>
                                <Form.Item
                                    label="点击翻页"
                                    name="clickPage"
                                    // initialValue={isChecked}
                                >
                                    <input style={{zIndex: 9000, width: 30, height: 30}} type={'checkbox'}
                                           checked={getSettingState().clickPage === '1'} onChange={e => {
                                        let string = e.target.checked === true ? "1" : "0";
                                        console.log('gagaga', e.target.checked)
                                        window.localStorage.setItem('clickPage', string)
                                        setSettingState({
                                            clickPage: string
                                        })
                                    }}/>
                                    {/*<Switch checked={getSettingState().clickPage === '1'} onChange={(value)=>{*/}
                                    {/*    let string = value === true ? "1":"0";*/}
                                    {/*    window.localStorage.setItem('clickPage',string)*/}
                                    {/*    setSettingState({*/}
                                    {/*        clickPage: string*/}
                                    {/*    })*/}
                                    {/*}} />*/}
                                    {/*<Checkbox checked={isChecked} onChange={function (e){*/}
                                    {/*    let string = e.target.checked === true ? "1":"0";*/}
                                    {/*    console.log('check--0-',string)*/}
                                    {/*    window.localStorage.setItem('clickPage',string)*/}
                                    {/*    setSettingState({*/}
                                    {/*        clickPage: string*/}
                                    {/*    })*/}
                                    {/*}}>是</Checkbox>*/}
                                </Form.Item>
                                <Form.Item
                                    label="显示进度"
                                    name="显示进度"
                                >
                                    <Switch checked={getSettingState().showProgress === '1'} onChange={(value) => {
                                        let string = value === true ? "1" : "0";
                                        window.localStorage.setItem('showProgress', string)
                                        setSettingState({
                                            showProgress: string
                                        })
                                    }}/>
                                </Form.Item>
                                <Form.Item
                                    label="窗口置顶"
                                    name="窗口置顶"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <Switch checked={getSettingState().isAlwaysTop === '1'} onChange={(value) => {
                                        let string = value === true ? "1" : "0";
                                        window.localStorage.setItem('isAlwaysTop', string)
                                        setSettingState({
                                            isAlwaysTop: string
                                        })
                                        isAlwaysTop(string)
                                    }}/>
                                </Form.Item>
                                <Form.Item
                                    label={<div>窗口隐藏 <Tooltip placement="left"
                                                                  title={"鼠标移入的时候才显示，移除变透明"}>
                                        <ExclamationCircleOutlined/>
                                    </Tooltip></div>}
                                    name="窗口隐藏"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <Switch checked={getSettingState().leaveWindowHid === '1'} onChange={(value) => {
                                        let string = value === true ? "1" : "0";
                                        window.localStorage.setItem('leaveWindowHid', string)
                                        setSettingState({
                                            leaveWindowHid: string
                                        })
                                        if (string === "0") {
                                            setDisplay(true)
                                        }
                                    }}/>
                                </Form.Item>
                                <Form.Item
                                    label={<div>透明模式 <Tooltip placement="left" title={"透明模式会把字体变为白色"}>
                                        <ExclamationCircleOutlined/>
                                    </Tooltip></div>}
                                    name="透明模式"
                                    rules={[
                                        {
                                            required: false,
                                            message: 'Please input your username!',
                                        },
                                    ]}
                                >
                                    <Switch checked={getSettingState().transparentMode === '1'} onChange={(value) => {
                                        let string = value === true ? "1" : "0";
                                        window.localStorage.setItem('transparentMode', string)
                                        setSettingState({
                                            transparentMode: string
                                        })
                                        if (string === '1') {
                                            window.localStorage.setItem('fontColor', '#fff')
                                            setSettingState({
                                                fontColor: "#fff"
                                            })
                                        }
                                    }}/>
                                </Form.Item>

                                <Form.Item label={null}>
                                    <Button type="primary" onClick={() => {
                                        window.localStorage.setItem('fontColor', '#000')
                                        window.localStorage.setItem('fontLineHeight', '30')
                                        window.localStorage.setItem('bgColor', '#fff')
                                        window.localStorage.setItem('fontSize', '16')
                                        window.localStorage.setItem('clickPage', "1")
                                        window.localStorage.setItem('showProgress', "0")
                                        window.localStorage.setItem('isAlwaysTop', "1")
                                        window.localStorage.setItem('transparentMode', "0")
                                        window.localStorage.setItem('leaveWindowHid', "1")
                                        setSettingState({
                                            fontColor: "#000",
                                            fontLineHeight: "30",
                                            bgColor: "#fff",
                                            fontSize: "16",
                                            clickPage: "1",
                                            showProgress: "0",
                                            isAlwaysTop: "1",
                                            transparentMode: "0",
                                            leaveWindowHid: "1",
                                        })
                                        setDisplay(true)
                                        isAlwaysTop(window.localStorage.getItem('isAlwaysTop'));
                                    }}>
                                        恢复默认
                                    </Button>
                                </Form.Item>
                            </Form>

                        </div>
                    </div>
                )
            }

            {/*目录 -- 弹窗*/}
            <div className="mulu-modal" style={{
                visibility: (display && getState().muluVisible && getState().currentBookChapterName) ? "visible" : "hidden"
            }}>
                            <span className={'mulu-modal-close'} onClick={() => {
                                setState({
                                    muluVisible: false
                                })
                            }}>
                                <span style={{fontSize: 16}}>
                                    {`${findIndex(getState().currentBookChapterList, e => e === getState().currentBookChapterName) + 1}/${getState().currentBookChapterList.length}`}
                                </span>
                                <span className={'mulu-modal-close-x'}>×</span>
                            </span>
                <div className={"mulu-modal-center"} onClick={(e) => {
                    e.stopPropagation();
                }}>
                    <ul className={'mulu-modal-center-ul'}>

                        {
                            getState().currentBookChapterList.map((ie, index) => {
                                return <li key={"mulu-modal-center-ul-li" + index}
                                           title={ie}
                                           className={classNames('mulu-modal-center-ul-li', {"mulu-modal-center-ul-li-active": ie === getState().currentBookChapterName})}
                                           onClick={(e) => {
                                               e.stopPropagation();
                                               goChapterByName(getState().currentBookName, ie)
                                               setState({
                                                   muluVisible: false
                                               })
                                           }}
                                >
                                    {ie}
                                </li>

                            })
                        }
                    </ul>
                </div>
            </div>
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
                width={500}
                onClose={function (){
                    setState({
                        downloadFromUrlVisible:false
                    })
                }}
                open={getState().downloadFromUrlVisible}
                extra={
                    <Space>
                        <Button onClick={(e)=>{
                            message.info("已取消操作")
                            setState({
                                downloadFromUrlVisible:false
                            })
                        }}>取消</Button>
                        <Button type="primary" onClick={(e)=>{
                            let filter1 = getState().downloadFromUrlList.filter(Boolean);
                            console.log('filter1--->',filter1)
                            if(isEmpty(filter1)){
                                message.error("请输入要下载的地址在提交")
                            }else{
                                AddFleAndHandlerRes(filter1);

                            }}}
                        >
                            提交
                        </Button>
                    </Space>
                }
            >
                <p className={"mb-5"}>
                    <Space>
                        <Button onClick={(e)=>{
                            let downloadFromUrlList = getState().downloadFromUrlList;
                            downloadFromUrlList.push(null)
                            setState({
                                downloadFromUrlList
                            })
                        }}><PlusOutlined/></Button>

                        <Button onClick={(e)=>{
                            let downloadFromUrlList = getState().downloadFromUrlList;
                            if(downloadFromUrlList.length>1) {
                                downloadFromUrlList.pop()
                                setState({
                                    downloadFromUrlList
                                })
                            }

                        }}><LineOutlined /></Button>
                    </Space>

                </p>

                <div className="flex flex-column-nowrap gap5">
                    {
                        getState().downloadFromUrlList.map((ite,index)=>{
                            return <Input key={"downloadFromUrlList-"+index} style={{width:'100%'}} onChange={(e)=>{
                                let value = e.target.value;
                                let downloadFromUrlList = getState().downloadFromUrlList;
                                downloadFromUrlList[index] = value
                                setState({
                                    downloadFromUrlList
                                })
                            }} placeHolder={"请输入地址"}/>
                        })
                    }
                </div>


            </Drawer>
            {/*</Spin>*/}

            {contextHolder}
        </div>
    )
}

export default App
