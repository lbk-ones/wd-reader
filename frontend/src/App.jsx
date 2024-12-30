import {useEffect, useRef, useState} from 'react';
import './App.css';
import {
    Button,
    Checkbox,
    ColorPicker,
    Form,
    Input,
    InputNumber,
    message,
    Popover,
    Spin,
    Switch,
    theme,
    Tooltip
} from 'antd';
import {generate, green, presetPalettes, red} from '@ant-design/colors';
import {findIndex, get, isEmpty} from 'lodash-es'
import classNames from "classnames"
import {
    GetAppPath,
    GetBookList,
    GetChapterListByFileName,
    GetServerUrl,
    Greet,
    OpenFileDialog, ParseEpubToTxt
} from "../wailsjs/go/main/App";
import {DeleteEpubFile, GetChapterContentByChapterName} from "../wailsjs/go/main/App.js";
import useAllState from "./components/lib/hooks/UseAllState.jsx";
import ContextMenu from "./components/ContextMenu.jsx";
import {SketchPicker} from "react-color";
import {
    Hide,
    Quit,
    WindowMinimise,
    WindowReload,
    WindowReloadApp,
    WindowSetAlwaysOnTop
} from "../wailsjs/runtime/runtime.js";
import {CloseOutlined, ExclamationCircleOutlined, MinusOutlined, RedoOutlined} from "@ant-design/icons";

function App() {
    const [display, setDisplay] = useState(true)
    let pageContentRef = useRef(null);
    let topRef = useRef(null);

    const [settingState, setSettingState, getSettingState] = useAllState({
        fontColor: window.localStorage.getItem('fontColor') || '#000',
        fontLineHeight: window.localStorage.getItem('fontLineHeight') || '30',
        bgColor: window.localStorage.getItem('bgColor') || '#fff',
        fontSize: window.localStorage.getItem('fontSize') || '16',
        clickPage: window.localStorage.getItem('clickPage') || '1',
        showProgress: window.localStorage.getItem('showProgress') || '0',
        isAlwaysTop: window.localStorage.getItem('isAlwaysTop') || '1',
        transparentMode: window.localStorage.getItem('transparentMode') || '0',
        leaveWindowHid: window.localStorage.getItem('leaveWindowHid') || '1',
    })

    const [state, setState, getState] = useAllState({
        bookList: [],
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
        loadingBook: false
    })
    const [location, setLocation] = useState(0)

    useEffect(function () {
        isAlwaysTop(null)
        reloadBookList();

        GetServerUrl().then(res => {
            setState({
                serverUrl: res
            })
        })
    }, [])

    function reloadBookList(cb = null) {
        setState({
            loadIngBookList: true
        })
        GetBookList().then(res => {
            if (res.startsWith("错误信息:")) {
                message.error(res);
            }
            setState({
                loadIngBookList: false,
                currentBookList: res.split("\n").map(e => e.trim()).filter(Boolean)
            })
            cb && cb()
        });
    }

    function pageDown() {
        pageContentRef.current.scrollBy({
            top: pageContentRef.current.clientHeight,
            behavior: 'smooth' // 平滑滚动，可根据需要设置
        });
    }

    function pageUp() {
        pageContentRef.current.scrollBy({
            top: -pageContentRef.current.clientHeight,
            behavior: 'smooth' // 平滑滚动，可根据需要设置
        });
    }

    function goChapterByName(fileName, chapterName, cb = null) {
        GetChapterListByFileName(fileName).then(res => {
            let strings = res.split("\n");
            let firstChapter = isEmpty(chapterName) ? get(strings, "[0]", "") : chapterName;
            GetChapterContentByChapterName(fileName, firstChapter).then(chapterContent => {
                window.localStorage.setItem(fileName, firstChapter)
                setState({
                    currentBookName: fileName,
                    currentBookChapterList: strings,
                    currentBookChapterName: firstChapter,
                    currentBookChapterContent: chapterContent.split("\n")
                })
                cb && cb();
            })

        })
    }

    function isAlwaysTop(_isAlwaysTop) {

        let isAlwaysTop = _isAlwaysTop ? _isAlwaysTop : getSettingState().isAlwaysTop;
        console.log('getSettingState().isAlwaysTop--->', isAlwaysTop)
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
    return (
        <div id="App"
             style={{
                 // border: display?"#ccc solid 1px":"none",
                 backgroundColor: (display && isEmpty(getState().currentBookChapterName)) ? '#fff' : 'rgba(0,0,0,0)',
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
            {/*<Spin spinning={getState().loadingBook} tip={"加载中。。"}>*/}
            {
                display && (getState().showTitle || isEmpty(getState().currentBookName)) && (
                    <div className={"title"}>
                        {
                            !isEmpty(getState().currentBookName) && (
                                <div className={"title-div"} style={{"--wails-draggable": 'drag', display: 'flex', flex: 1}}
                                     title={getState().currentBookName.replace(".txt", "")}>
                                    {
                                        `${getState().currentBookName.replace(".txt", "")}`
                                    }
                                </div>
                            )
                        }

                        {
                            !isEmpty(getState().currentBookName) && (
                                <div className={"title-div"} style={{justifyContent: 'flex-end'}}
                                     title={getState().currentBookChapterName}>
                                    {
                                        `${getState().currentBookChapterName}`
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
                                        <div style={{"--wails-draggable": 'drag', display: 'flex', flex: 'auto'}}>
                                            上班偷看小说神器
                                        </div>

                                        <div style={{display: "flex", flexFlow: 'row nowrap', gap: 5, fontSize: 20}}
                                             onClick={(e) => {
                                                 e.stopPropagation()
                                             }}>
                                            <RedoOutlined onClick={() => {
                                                WindowReloadApp()
                                            }}/>
                                            <MinusOutlined onClick={() => {
                                                WindowMinimise();
                                            }}/>
                                            <CloseOutlined onClick={() => {
                                                Quit();
                                            }}/>
                                        </div>

                                    </div>
                                </div>
                            )
                        }

                    </div>
                )
            }
            {
                isEmpty(state.currentBookChapterName) && display && (function () {
                    if (getState().loadIngBookList) {
                        return (
                            <div className={"flex-center"}>加载中....</div>
                        )
                    }
                    if (!getState().loadIngBookList && isEmpty(getState().currentBookList)) {

                        return (
                            <div className={"flex-center"} style={{
                                fontSize: 18,
                                padding: 20,
                                color: 'darkred'
                            }}>未检索到文件列表,请将（txt、epub）格式文件放到程序同级目录books下面</div>
                        );
                    }
                    if (!getState().loadIngBookList && !isEmpty(getState().currentBookList)) {
                        return (
                            <div className="book-list">
                                <ul className={"book-list-ul"}>
                                    {
                                        getState().currentBookList.map((tem, index) => {
                                            return <li className={"book-list-ul-li"} key={"book-list-ul-li" + index}
                                                       title={tem} onClick={() => {
                                                if (tem.endsWith("epub")) {
                                                    setState({
                                                        loadingBook: true
                                                    })
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
                                                                    console.log(res)
                                                                })
                                                            });
                                                        })

                                                    })
                                                } else {
                                                    let item = window.localStorage.getItem(tem);
                                                    goChapterByName(tem, item);
                                                }
                                            }}>{tem}</li>
                                        })
                                    }
                                </ul>

                            </div>

                        )
                    }
                })()
            }


            {/*内容页面*/}
            {
                !isEmpty(state.currentBookChapterName) && !getState().currentBookName.endsWith(".epub") && (
                    <div className={"book-content-div"} style={{
                        lineHeight: Number(getSettingState().fontLineHeight) + "px",
                        padding: "0 10px",
                        width: 'inherit',
                        height: 'inherit',
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
                        <div id={"book-content-div-top"} style={{width: '100%', height: '1px'}} ref={topRef}></div>
                        {
                            getState().currentBookChapterContent.map(res => {
                                return <p>
                                    {res}
                                </p>
                            })
                        }

                        <div className={"book-content-div-footer-btn"}>
                            <Button size={"small"} onClick={(e) => {
                                e.stopPropagation();
                                lastChpater();
                            }}>上一章</Button>
                            <Button size={"small"} onClick={(e) => {
                                e.stopPropagation();
                                nextChpater();
                            }}>下一章</Button>
                            <span
                                className={'book-content-div-footer-btn-txt'}>下一章: &nbsp;{getState().currentBookChapterList[findIndex(getState().currentBookChapterName) + 1] || "无"}</span>
                        </div>
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
                                    span: 8,
                                    xs: 8,
                                    sm: 8
                                }}
                                wrapperCol={{
                                    span: 16,
                                    xs: 16,
                                    sm: 16,
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
            {
                display && getState().muluVisible && (
                    <div className="mulu-modal">
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
                                                   onClick={() => {
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
                )
            }
            <Spin spinning={getState().loadingBook} tip={"加载中。。"} fullscreen/>
            {/*</Spin>*/}
        </div>
    )
}

export default App
