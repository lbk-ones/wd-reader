import {t, useI18n} from '../../i18n/index.jsx';
import LanguageSelect from '../../i18n/LanguageSelect.jsx';
import {App as AntdApp, Button, Form, Input, InputNumber, Popover, Select, Switch, Tooltip} from "antd";
import {
    CACHE_PREFIX,
    getCacheItem,
    getSplitMap,
    getSplitType,
    getSplitTypeValue,
    putSplitMap,
    setCacheItem,
    setSplitTypeValue
} from "../Utils.jsx";
import {SketchPicker} from "react-color";
import {CloseOutlined, ExclamationCircleOutlined} from "@ant-design/icons";
import * as PropTypes from "prop-types";
import {Window} from "@wailsio/runtime";
import {DEFAULT_READING_TYPOGRAPHY, READING_TYPOGRAPHY_PRESETS} from '../readingTypography.js';
import {calculateReadingLayout} from '../readingLayout.js';
import {DEFAULT_READING_COLORS, READING_COLOR_PRESETS} from '../readingColors.js';
import {CHAPTER_REGEX_PRESETS} from '../chapterRegexPresets.js';

function ContentSetting(props) {
    useI18n();
    const {modal} = AntdApp.useApp();

    let state = props.state;
    let display = props.display;
    let settingState = props.settingState;
    const line = settingState.miniRows;
    let settingVisible = state.settingVisible;
    let setState = props.setState;
    let isAlwaysTop = props.isAlwaysTop;
    let setDisplay = props.setDisplay;
    let miniSize = props.miniSize;
    let toggleTitle = props.toggleTitle;
    let setSettingState = props.setSettingState;

    if (!(display && settingVisible)) {
        return null
    }

    let currentBookName = state.currentBookName;
    let splitMaElement = getSplitType(currentBookName);

    return (
        <div className="setting-modal" role="dialog" aria-labelledby="reading-preferences-title">
            <header className="settings-header">
                <h2 id="reading-preferences-title">{t("阅读偏好")}</h2>
                <LanguageSelect compact/>
                <button type="button" className="setting-modal-close" aria-label={t("关闭阅读偏好")}
                        onClick={() => setState({settingVisible: false})}>
                    <CloseOutlined/>
                </button>
            </header>
            <div className={"setting-modal-center"} onClick={(e) => {
                e.stopPropagation();
            }}>
                <Form
                    className="reading-settings-grid"
                    size="small"
                    colon={false}
                    labelAlign="left"
                    layout={"horizontal"}
                    name="basic"
                    labelCol={{
                        flex: '68px'
                    }}
                    wrapperCol={{
                        flex: 1,
                    }}
                    initialValues={{
                        layout: "horizontal",
                    }}
                    // onFinish={onFinish}
                    // onFinishFailed={onFinishFailed}
                    autoComplete="off"
                >
                    <h3 className="settings-section-title">{t("文字与配色")}</h3>
                    <div className="settings-typography-presets settings-wide" role="group" aria-label={t("阅读字号与行高预设")}>
                        {READING_TYPOGRAPHY_PRESETS.map(preset => {
                            const layout = calculateReadingLayout(settingState.newContentHeight, preset.fontSize,
                                preset.lineHeight, settingState.miniMode === '1' ? line : 0);
                            const active = layout && Number(settingState.fontSize) === layout.fontSize
                                && Number(settingState.fontLineHeight) === layout.fontLineHeight;
                            return <button type="button" key={preset.name}
                                           className={`settings-typography-preset${active ? ' is-active' : ''}`}
                                           aria-pressed={!!active}
                                           title={t("{0}：字号 {1}px，行高 {2}px。行高会按窗口高度微调以保持整行翻页。", {0: t(preset.name), 1: preset.fontSize, 2: preset.lineHeight})}
                                           onClick={() => props.calculateFontLines(preset.lineHeight, preset.fontSize,
                                               settingState.miniMode === '1' ? 120 : 0, settingState.miniMode === '1' ? line : 0)}>
                                <span>{t(preset.name)}</span>
                                <span className="settings-typography-values">{preset.fontSize} / {preset.lineHeight}</span>
                            </button>;
                        })}
                    </div>
                    <Form.Item label={t("字体大小")}>
                        <InputNumber aria-label={t("字体大小（像素）")} style={{width: 64}}
                                     value={Number(settingState.fontSize)}
                                     onChange={(value) => {
                                         if (value === null) return;
                                         props.calculateFontLines(null,value)
                                     }}/>
                    </Form.Item>

                    <Form.Item label={t("行间距")}>
                        <InputNumber aria-label={t("行高（像素）")} style={{width: 64}}
                                     value={Number(settingState.fontLineHeight)}
                                     onChange={(value) => {
                                         if (value === null) return;
                                         props.calculateFontLines(value)
                                     }}/>
                    </Form.Item>
                    <div className="settings-color-presets settings-wide" role="group" aria-label={t("阅读配色预设")}>
                        {READING_COLOR_PRESETS.map(preset => {
                            const active = settingState.transparentMode !== '1'
                                && settingState.bgColor.toUpperCase() === preset.bgColor
                                && settingState.fontColor.toUpperCase() === preset.fontColor;
                            return <button type="button" key={preset.name}
                                           className={`settings-color-preset${active ? ' is-active' : ''}`}
                                           style={{backgroundColor: preset.bgColor, color: preset.fontColor}}
                                           aria-pressed={active}
                                           aria-label={t("{0}配色", {0: t(preset.name)})}
                                           title={t("{0}：背景 {1}，字体 {2}。同时应用两种颜色并关闭透明模式。", {0: t(preset.name), 1: preset.bgColor, 2: preset.fontColor})}
                                           onClick={() => {
                                               const patch = {bgColor: preset.bgColor, fontColor: preset.fontColor, transparentMode: '0'};
                                               Object.entries(patch).forEach(([key, value]) => setCacheItem(key, value));
                                               setSettingState(patch);
                                           }}>
                                <span className="settings-color-preview" aria-hidden="true">Aa</span>
                                <span>{t(preset.name)}</span>
                            </button>;
                        })}
                    </div>
                    <Form.Item
                        className="settings-wide"
                        label={t("背景色")}
                        name="背景色"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Popover zIndex={1200} content={() => {
                            return (
                                <SketchPicker
                                    disableAlpha
                                    color={settingState.bgColor}
                                    onChange={(color) => {
                                        setCacheItem('bgColor', color.hex)
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
                        }} trigger="click" title={t("背景颜色")}>

                            <button type="button" className="settings-color-button" aria-label={t("选择背景色")}>
                                <span className="settings-color-swatch" style={{backgroundColor: settingState.bgColor}}/>
                                <span>{settingState.bgColor.toUpperCase()}</span>
                                <span className="settings-color-edit">{t("更改")}</span>
                            </button>

                        </Popover>
                    </Form.Item>
                    <Form.Item
                        className="settings-wide"
                        label={t("字体颜色")}
                        name="字体颜色"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Popover zIndex={1200} content={() => {
                            return (
                                <SketchPicker
                                    disableAlpha
                                    color={settingState.fontColor}
                                    onChange={(color) => {
                                        let hex = color.hex;
                                        setCacheItem('fontColor', hex)
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
                        }} trigger="click" title={t("字体颜色")}>

                            <button type="button" className="settings-color-button" aria-label={t("选择字体颜色")}>
                                <span className="settings-color-swatch" style={{backgroundColor: settingState.fontColor}}/>
                                <span>{settingState.fontColor.toUpperCase()}</span>
                                <span className="settings-color-edit">{t("更改")}</span>
                            </button>

                        </Popover>

                    </Form.Item>
                    <h3 className="settings-section-title">{t("阅读与窗口")}</h3>
                    <Form.Item
                        label={t("点击翻页")}
                        name="clickPage"
                    >
                        <Switch size="small" checked={settingState.clickPage === '1'} onChange={value => {
                            let string = value ? "1" : "0";
                            setCacheItem('clickPage', string)
                            setSettingState({
                                clickPage: string
                            })
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={t("显示进度")}
                        name="显示进度"
                    >
                        <Switch size="small" checked={settingState.showProgress === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('showProgress', string)
                            setSettingState({
                                showProgress: string
                            })
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={t("窗口置顶")}
                        name="窗口置顶"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch size="small" checked={settingState.isAlwaysTop === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('isAlwaysTop', string)
                            setSettingState({
                                isAlwaysTop: string
                            })
                            isAlwaysTop(string)
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={<div>{t("窗口隐藏")}<Tooltip placement="left"
                                                      title={<span>{t("鼠标移入的时候才显示,")}<br/>{t("移除变透明")}</span>}>
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
                        <Switch size="small" checked={settingState.leaveWindowHid === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('leaveWindowHid', string)
                            setSettingState({
                                leaveWindowHid: string
                            })
                            if (string === "0") {
                                setDisplay(true)
                            }
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={<div>{t("透明模式")}<Tooltip placement="left" title={t("透明模式会把字体变为白色")}>
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
                        <Switch size="small" checked={settingState.transparentMode === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('transparentMode', string)
                            setSettingState({
                                transparentMode: string
                            })
                            if (string === '1') {
                                setCacheItem('fontColor', '#fff')
                                setSettingState({
                                    fontColor: "#fff"
                                })
                            }
                        }}/>
                    </Form.Item>

                    <Form.Item
                        label={t("标题开关")}
                        name="标题开关"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch size="small" checked={state.showTitle === true} onChange={(value) => {
                            toggleTitle()
                        }}/>
                    </Form.Item>

                    <Form.Item
                        label={t("迷你化")}
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch size="small" aria-label={t("迷你化")} checked={settingState.miniMode === '1'} onChange={(value) => {
                                    setState({
                                        settingVisible: false
                                    })
                                    miniSize(line, value).catch(console.error)
                                }}
                        />
                    </Form.Item>
                    <Form.Item label={t("迷你行数")}>
                        <InputNumber
                            aria-label={t("迷你行数")}
                            value={line}
                            min={1}
                            max={4}
                            style={{
                                width:'64px'
                            }}
                            onChange={(value) => {
                                if (value === null) return;
                                setCacheItem('miniRows', value)
                                setSettingState({miniRows: value})
                                if (settingState.miniMode === '1') {
                                    miniSize(value, true).catch(console.error)
                                }
                            }}
                        />


                    </Form.Item>

                    <h3 className="settings-section-title">{t("自动阅读")}</h3>
                    <Form.Item className="settings-wide settings-auto-page" label={<div>{t("自动翻页")}<Tooltip title={t("按设定间隔翻到下一页，章末自动进入下一章；打开设置或目录时暂停，与自动滚动互斥。F7 可快速开关。")}><ExclamationCircleOutlined/></Tooltip></div>}>
                        <div className="flex gap3 flex-wrap" style={{alignItems: 'center'}}>
                            <Switch size="small" aria-label={t("自动翻页")} checked={settingState.autoPage === '1'}
                                    onChange={value => {
                                        props.setAutoReadingMode('autoPage', value);
                                    }}/>
                            <span>{t("每")}</span>
                            <InputNumber aria-label={t("自动翻页间隔（秒）")} min={1} max={3600} precision={0}
                                         style={{width: 64}} value={settingState.autoPageInterval}
                                         onChange={value => {
                                             if (value === null || !Number.isFinite(Number(value))) return;
                                             const autoPageInterval = Math.min(3600, Math.max(1, Math.round(Number(value))));
                                             setCacheItem('autoPageInterval', autoPageInterval);
                                             setSettingState({autoPageInterval});
                                         }}/>
                            <span>{t("秒")}</span>
                        </div>
                    </Form.Item>
                    <Form.Item className="settings-wide settings-auto-scroll" label={<div>{t("自动滚动")}<Tooltip title={t("与自动翻页互斥。打开设置或目录时暂停；手动滚动后暂缓 1.5 秒。F9 可快速开关。")}><ExclamationCircleOutlined/></Tooltip></div>}>
                        <div className="flex gap3 flex-wrap" style={{alignItems: 'center'}}>
                            <Switch size="small" aria-label={t("自动向下滚动")} checked={settingState.autoScroll === '1'}
                                    onChange={value => props.setAutoReadingMode('autoScroll', value)}/>
                            <span>{t("速度")}</span>
                            <InputNumber aria-label={t("自动滚动速度（像素每秒）")} min={5} max={100} precision={0}
                                         style={{width: 64}} value={settingState.autoScrollSpeed}
                                         onChange={value => {
                                             if (value === null || !Number.isFinite(Number(value))) return;
                                             const autoScrollSpeed = Math.min(100, Math.max(5, Math.round(Number(value))));
                                             setCacheItem('autoScrollSpeed', autoScrollSpeed);
                                             setSettingState({autoScrollSpeed});
                                         }}/>
                            <span>{t("像素/秒")}</span>
                        </div>
                    </Form.Item>
                    <h3 className="settings-section-title settings-split-title">{t("断章方式")}<Tooltip title={t("默认智能识别章节。章节混乱时可切换为按段落数或自定义正则分章，修改后点击应用。")}>
                            <ExclamationCircleOutlined/>
                        </Tooltip>
                    </h3>
                    <Form.Item className="settings-wide">
                        <div className={`settings-chapter-controls${splitMaElement === '2' ? ' is-paragraph' : ''}`}>
                            <Select aria-label={t("断章方式")} className="settings-split-method" value={splitMaElement} onChange={(value, option)=>{
                                putSplitMap(currentBookName,value);
                                let splitTypeValue
                                if(value === '2') {
                                    splitTypeValue = setSplitTypeValue(currentBookName,"100");
                                }else {
                                    splitTypeValue = setSplitTypeValue(currentBookName,"");
                                }
                                setSettingState({
                                    splitMap:splitTypeValue
                                })
                                setCacheItem(currentBookName, "")
                            }}>
                                <Select.Option key={"1"} value={"1"}>{t("智能识别")}</Select.Option>
                                <Select.Option key={"2"} value={"2"}>{t("按段落")}</Select.Option>
                                <Select.Option key={"3"} value={"3"}>{t("自定义正则")}</Select.Option>
                            </Select>
                            {
                                splitMaElement === '3' && (<>
                                    <Select className="settings-split-preset" aria-label={t("正则预设")}
                                            placeholder={t("选择正则预设")} value={null}
                                            onChange={value => {
                                                const splitMap = setSplitTypeValue(currentBookName, value);
                                                setSettingState({splitMap});
                                            }}
                                            options={CHAPTER_REGEX_PRESETS.map(preset => ({value: preset.pattern, label: t(preset.label)}))}/>
                                    <Input allowClear className="settings-split-regex" aria-label={t("断章正则表达式")}
                                           placeholder={t("请输入正则表达式")}
                                           value={getSplitTypeValue(currentBookName)}
                                           onChange={e=>{
                                               let value = e.target.value;
                                               let splitTypeValue = setSplitTypeValue(currentBookName,value);
                                               setSettingState({
                                                   splitMap:splitTypeValue
                                               })
                                           }}
                                    />
                                </>)
                            }

                            {
                                splitMaElement === '2' && (
                                    <InputNumber
                                        className="settings-split-count"
                                        aria-label={t("每章段落数")}
                                        addonAfter={t("段")}
                                        precision={0}
                                        value={Number(getSplitTypeValue(currentBookName))}
                                        min={10}
                                        max={1000}
                                        onChange={(value) => {
                                            if (value === null) return;
                                            let splitTypeValue = setSplitTypeValue(currentBookName,value);
                                            setSettingState({
                                                splitMap:splitTypeValue
                                            })
                                        }}
                                    />
                                )
                            }
                            <Button className="settings-split-apply" title={t("应用断章方式")} aria-label={t("应用断章方式")} onClick={()=>{
                                modal.confirm({
                                    title: t("提示！"),
                                    type: 'warning',
                                    okText: t("我确定"),
                                    cancelText: t("取消"),
                                    content: t("是否清空该本书的章节阅读历史"),
                                    onOk: () => {

                                        props.backBookList(()=>{
                                            setCacheItem(currentBookName, "")

                                            props.selectBook(currentBookName)
                                        })

                                    },
                                    onCancel:()=>{
                                        setState({
                                            settingVisible: false
                                        })
                                        props.backBookList(function (){
                                            props.selectBook(currentBookName)
                                        })

                                    }
                                })

                            }}>{t("应用")}</Button>

                        </div>



                    </Form.Item>

                    <Form.Item
                        style={{
                            marginTop:'5px'
                        }}
                        label={t("操作")}
                        className="settings-wide"
                        name="操作"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Button type="primary" onClick={async () => {
                            if (settingState.miniMode === '1') {
                                try {
                                    await miniSize(3, false)
                                } catch (error) {
                                    console.error(error)
                                    return;
                                }
                            }
                            let lineHeight = DEFAULT_READING_TYPOGRAPHY.lineHeight
                            let fontSize = DEFAULT_READING_TYPOGRAPHY.fontSize
                            setCacheItem('fontColor', DEFAULT_READING_COLORS.fontColor)
                            setCacheItem('fontLineHeight', lineHeight+"")
                            setCacheItem('bgColor', DEFAULT_READING_COLORS.bgColor)
                            setCacheItem('fontSize', fontSize+"")
                            setCacheItem('clickPage', "1")
                            setCacheItem('autoPage', "0")
                            setCacheItem('autoPageInterval', 30)
                            setCacheItem('autoScroll', '0')
                            setCacheItem('autoScrollSpeed', 20)
                            setCacheItem('showProgress', "1")
                            setCacheItem('isAlwaysTop', "1")
                            setCacheItem('transparentMode', "0")
                            setCacheItem('leaveWindowHid', "0")

                            // 默认智能断章
                            putSplitMap(currentBookName,"1");
                            const splitTypeValue = setSplitTypeValue(currentBookName,"");

                            setCacheItem('miniRows', 3)
                            setSettingState({
                                splitMap:splitTypeValue,
                                fontColor: DEFAULT_READING_COLORS.fontColor,
                                fontLineHeight: lineHeight+"",
                                bgColor: DEFAULT_READING_COLORS.bgColor,
                                fontSize: fontSize+"",
                                clickPage: "1",
                                autoPage: "0",
                                autoPageInterval: 30,
                                autoScroll: '0',
                                autoScrollSpeed: 20,
                                showProgress: "1",
                                isAlwaysTop: "1",
                                transparentMode: "0",
                                leaveWindowHid: "0",
                                miniRows: 3,
                            })
                            setDisplay(true)
                            isAlwaysTop(getCacheItem('isAlwaysTop'));
                            setTimeout(()=>{
                                props.calculateFontLines(lineHeight, fontSize)
                            },100)
                        }}>{t("恢复默认")}</Button>
                    </Form.Item>
                </Form>

            </div>
        </div>
    )

}

ContentSetting.propTypes = {

    setState: PropTypes.func,
    state: PropTypes.any,
    settingState: PropTypes.any,
    display: PropTypes.any,
    isAlwaysTop: PropTypes.func,
    miniSize: PropTypes.func,
    toggleTitle: PropTypes.func,
    calculateFontLines: PropTypes.func,
    setDisplay: PropTypes.func,
    setSettingState: PropTypes.func,
    setAutoReadingMode: PropTypes.func,
};
export default ContentSetting
