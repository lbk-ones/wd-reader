import {Button, Form, Input, InputNumber, Modal, Popover, Select, Switch, Tooltip} from "antd";
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
import {ExclamationCircleOutlined} from "@ant-design/icons";
import * as PropTypes from "prop-types";
import {useState} from "react";
import {WindowReloadApp} from "../../../wailsjs/runtime/runtime.js";

function ContentSetting(props) {

    const [line, setLine] = useState(3)
    let state = props.state;
    let display = props.display;
    let settingState = props.settingState;
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
        <div className="setting-modal">
                            <span className={'setting-modal-close'} onClick={() => {
                                setState({
                                    settingVisible: false
                                })
                                // props.calculateFontLines()
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
                        sm: 4
                    }}
                    wrapperCol={{
                        span: 15,
                        xs: 15,
                        sm: 20,
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
                        <InputNumber style={{width: 100}}
                                     defaultValue={Number(settingState.fontSize)}
                                     value={Number(settingState.fontSize)}
                                     onChange={(value) => {
                                         // setCacheItem('fontLineHeight', value)
                                         // setSettingState({
                                         //     fontSize: value
                                         // })
                                         props.calculateFontLines(null,value)
                                     }}/>
                    </Form.Item>

                    <Form.Item
                        label="行间距"
                        name="行间距"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <InputNumber style={{width: 100}}
                                     defaultValue={Number(settingState.fontLineHeight)}
                                     value={Number(settingState.fontLineHeight)}
                                     onChange={(value) => {
                                         props.calculateFontLines(value)
                                         // setCacheItem('fontLineHeight', value)
                                         // setSettingState({
                                         //     fontLineHeight: value
                                         // })
                                     }}/>
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
                        }} trigger="click" title="颜色选择">

                            <div
                                style={{
                                    width: '20px',
                                    height: '20px',
                                    // margin: '20px',
                                    backgroundColor: settingState.bgColor
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
                        }} trigger="click" title="颜色选择">

                            <div
                                style={{
                                    width: '20px',
                                    height: '20px',
                                    // margin: '20px',
                                    backgroundColor: settingState.fontColor
                                }}
                            />

                        </Popover>

                    </Form.Item>
                    <Form.Item
                        label="点击翻页"
                        name="clickPage"
                    >
                        <input style={{zIndex: 9000, width: 30, height: 30}} type={'checkbox'}
                               checked={settingState.clickPage === '1'} onChange={e => {
                            let string = e.target.checked === true ? "1" : "0";
                            setCacheItem('clickPage', string)
                            setSettingState({
                                clickPage: string
                            })
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label="显示进度"
                        name="显示进度"
                    >
                        <Switch checked={settingState.showProgress === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('showProgress', string)
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
                        <Switch checked={settingState.isAlwaysTop === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('isAlwaysTop', string)
                            setSettingState({
                                isAlwaysTop: string
                            })
                            isAlwaysTop(string)
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={<div>窗口隐藏 <Tooltip placement="left"
                                                      title={<span>鼠标移入的时候才显示,<br/>移除变透明</span>}>
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
                        <Switch checked={settingState.leaveWindowHid === '1'} onChange={(value) => {
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
                        <Switch checked={settingState.transparentMode === '1'} onChange={(value) => {
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
                        label="标题开关"
                        name="标题开关"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch checked={state.showTitle === true} onChange={(value) => {
                            toggleTitle()
                        }}/>
                    </Form.Item>

                    <Form.Item
                        label="迷你化"
                        name="迷你化"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch onChange={(value) => {
                                    setState({
                                        settingVisible: false
                                    })
                                    miniSize(line)
                                }}
                        />
                        &nbsp;&nbsp;&nbsp;
                        行数：
                        <InputNumber
                            value={line}
                            min={1}
                            max={4}
                            style={{
                                width:'70px'
                            }}
                            onChange={(value) => {
                                console.log('value-000>',value)
                                setLine(value)
                            }}
                        />


                    </Form.Item>

                    <Form.Item
                        style={{
                            marginTop:'5px'
                        }}
                        label={<div>断章方式 <Tooltip placement=" left"
                                                      title={<span>章节的提取方式<br/>默认是智能提取<br/>如果文章出现章节混乱<br/>那么可以切换章节提取方式</span>}>
                            <ExclamationCircleOutlined/>
                        </Tooltip></div>}
                        name="断章方式"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <div className={"flex gap3 flex-wrap"}>
                            <Select value={splitMaElement} onChange={(value, option)=>{
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
                                <Select.Option key={"1"} value={"1"}>智能</Select.Option>
                                <Select.Option  key={"2"} value={"2"}>按段落数分章</Select.Option>
                                <Select.Option key={"3"} value={"3"}>自定义正则分章</Select.Option>
                            </Select>
                            {
                                splitMaElement === '3' && (
                                    <Input allowClear
                                           placeHolder={"请输入正则表达式"}
                                           value={getSplitTypeValue(currentBookName)}
                                           onChange={e=>{
                                               let value = e.target.value;
                                               let splitTypeValue = setSplitTypeValue(currentBookName,value);
                                               setSettingState({
                                                   splitMap:splitTypeValue
                                               })
                                           }}
                                    />
                                )
                            }

                            {
                                splitMaElement === '2' && (
                                    <InputNumber
                                        value={Number(getSplitTypeValue(currentBookName))}
                                        min={10}
                                        max={1000}
                                        style={{
                                            width:'70px'
                                        }}
                                        onChange={(value) => {
                                            let splitTypeValue = setSplitTypeValue(currentBookName,value);
                                            setSettingState({
                                                splitMap:splitTypeValue
                                            })
                                        }}
                                    />
                                )
                            }
                            <Button onClick={()=>{
                                Modal.confirm({
                                    title: "提示！",
                                    type: 'warning',
                                    okText: "我确定",
                                    cancelText: "取消",
                                    content: "是否清空该本书的章节阅读历史",
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

                            }}>应用断章方式</Button>

                        </div>



                    </Form.Item>

                    <Form.Item
                        style={{
                            marginTop:'5px'
                        }}
                        label={"操作"}
                        name="操作"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Button type="primary" onClick={() => {
                            let lineHeight = 30
                            let fontSize = 20
                            setCacheItem('fontColor', '#000')
                            setCacheItem('fontLineHeight', lineHeight+"")
                            setCacheItem('bgColor', '#E8E3D7')
                            setCacheItem('fontSize', fontSize+"")
                            setCacheItem('clickPage', "1")
                            setCacheItem('showProgress', "1")
                            setCacheItem('isAlwaysTop', "1")
                            setCacheItem('transparentMode', "0")
                            setCacheItem('leaveWindowHid', "0")

                            // 默认智能断章
                            putSplitMap(currentBookName,"1");
                            const splitTypeValue = setSplitTypeValue(currentBookName,"");

                            setLine(3)
                            setSettingState({
                                splitMap:splitTypeValue,
                                fontColor: "#000",
                                fontLineHeight: lineHeight+"",
                                bgColor: "#E8E3D7",
                                fontSize: fontSize+"",
                                clickPage: "1",
                                showProgress: "1",
                                isAlwaysTop: "1",
                                transparentMode: "0",
                                leaveWindowHid: "0",
                            })
                            setDisplay(true)
                            isAlwaysTop(getCacheItem('isAlwaysTop'));
                            setTimeout(()=>{
                                props.calculateFontLines(lineHeight)
                            },100)
                        }}>
                            恢复默认
                        </Button>
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
};
export default ContentSetting